import { LIKEABLE_NOTIFICATION_END, LIKEABLE_NOTIFICATION_START } from './config';
import type { Feed, FeedRow, Message, MessageAttachment, NotificationFeedRow } from './domain';

export function feedRows(feed: Feed | null): FeedRow[] {
  if (!feed) return [];
  const rows: FeedRow[] = [];
  for (const msg of feed.localMessages ?? []) {
    if (msg.role !== 'user') continue;
    const normalized = normalizeLocalMessage(msg);
    rows.push({ kind: 'user', id: msg.id, role: 'user', body: normalized.body, attachments: normalized.attachments, time: msg.createdAt });
  }
  rows.push(...notificationFeedRows(feed));
  return rows.sort(compareFeedRows);
}

function normalizeLocalMessage(msg: Message): { body: string; attachments: MessageAttachment[] } {
  const attachments = [...(msg.attachments ?? [])];
  const marker = '\n\nAttachments:';
  const compactMarker = '\nAttachments:';
  const markerIndex = msg.body.indexOf(marker);
  const compactMarkerIndex = markerIndex === -1 ? msg.body.indexOf(compactMarker) : -1;
  const splitIndex = markerIndex >= 0 ? markerIndex : compactMarkerIndex;
  if (splitIndex === -1) return { body: msg.body, attachments };

  const legacyAttachmentText = msg.body.slice(splitIndex).replace(/^\n+Attachments:\s*/i, '');
  for (const line of legacyAttachmentText.split('\n')) {
    const filename = line.replace(/^\s*-\s*/, '').trim();
    if (filename) attachments.push({ id: `legacy-${msg.id}-${attachments.length}`, filename });
  }
  return { body: msg.body.slice(0, splitIndex).trim(), attachments };
}

function notificationFeedRows(feed: Feed): NotificationFeedRow[] {
  const userTimes = (feed.localMessages ?? [])
    .filter((msg) => msg.role === 'user')
    .map((msg) => Date.parse(msg.createdAt))
    .filter((value) => !Number.isNaN(value))
    .sort((a, b) => a - b);
  const latestTurnKey = userTimes.length > 0 ? `turn-${userTimes[userTimes.length - 1]}` : '';
  const assistantRows: NotificationFeedRow[] = [];
  const assistantCounters = new Map<string, number>();
  const assistantSources = (feed.messages ?? [])
    .map((msg, sourceIndex) => ({
      sourceIndex,
      timeValue: Date.parse(msg.created_at ?? ''),
      time: msg.created_at,
      role: msg.role,
      body: msg.body ?? msg.content ?? ''
    }))
    .filter((source) => source.role === 'assistant')
    .sort((a, b) => {
      const left = Number.isNaN(a.timeValue) ? Number.MAX_SAFE_INTEGER : a.timeValue;
      const right = Number.isNaN(b.timeValue) ? Number.MAX_SAFE_INTEGER : b.timeValue;
      if (left !== right) return left - right;
      return a.sourceIndex - b.sourceIndex;
    });
  for (const source of assistantSources) {
    const turnKey = turnKeyForTime(userTimes, Number.isNaN(source.timeValue) ? null : source.timeValue) ?? `assistant-${source.sourceIndex}`;
    for (const segment of parseLikeableNotificationSegments(source.body)) {
      const segmentIndex = assistantCounters.get(turnKey) ?? 0;
      assistantCounters.set(turnKey, segmentIndex + 1);
      assistantRows.push({
        kind: 'notification',
        id: `${turnKey}-notification-${segmentIndex}`,
        body: segment.body,
        time: source.time,
        active: segment.streaming,
        fallback: segment.fallback
      });
    }
  }

  const activityRows: NotificationFeedRow[] = [];
  for (const [index, activity] of (feed.activity ?? []).entries()) {
    const sourceID = activity.id ?? activity.occurred_at ?? `activity-${index}`;
    const body = activity.message ?? '';
    for (const [segmentIndex, segment] of parseLikeableNotificationSegments(body).entries()) {
      activityRows.push({
        kind: 'notification',
        id: `activity-${sourceID}-notification-${segmentIndex}`,
        body: segment.body,
        time: activity.occurred_at,
        active: segment.streaming,
        fallback: segment.fallback
      });
    }
  }

  const rows = assistantRows.length > 0 ? [...assistantRows] : [...activityRows];
  const latestTurnAlreadyPersisted = latestTurnKey !== '' && rows.some((row) => row.id.startsWith(`${latestTurnKey}-notification-`));
  if (feed.live?.streamText && !latestTurnAlreadyPersisted) {
    const liveTurnKey = latestTurnKey || 'live';
    for (const [segmentIndex, segment] of parseLikeableNotificationSegments(feed.live.streamText).entries()) {
      rows.push({
        kind: 'notification',
        id: `${liveTurnKey}-notification-${segmentIndex}`,
        body: segment.body,
        time: feed.live.startedAt,
        active: Boolean(feed.live.isProcessing || segment.streaming),
        fallback: segment.fallback
      });
    }
  }
  return rows;
}

function turnKeyForTime(userTimes: number[], sourceTime: number | null): string | null {
  if (userTimes.length === 0) return null;
  if (sourceTime == null) return `turn-${userTimes[userTimes.length - 1]}`;
  let selected = userTimes[0];
  for (const time of userTimes) {
    if (time <= sourceTime + 5000) selected = time;
    if (time > sourceTime + 5000) break;
  }
  return `turn-${selected}`;
}

function compareFeedRows(a: FeedRow, b: FeedRow): number {
  const left = Date.parse(a.time ?? '');
  const right = Date.parse(b.time ?? '');
  if (!Number.isNaN(left) && !Number.isNaN(right) && left !== right) return left - right;
  if (a.kind !== b.kind) return a.kind === 'user' ? -1 : 1;
  return a.id.localeCompare(b.id);
}

function parseLikeableNotificationSegments(value: string): Array<{ body: string; streaming: boolean; fallback?: boolean }> {
  const segments: Array<{ body: string; streaming: boolean; fallback?: boolean }> = [];
  let cursor = 0;
  while (cursor < value.length) {
    const start = value.indexOf(LIKEABLE_NOTIFICATION_START, cursor);
    if (start === -1) {
      if (value.indexOf('[[LIKEABLE', cursor) !== -1) {
        segments.push({ body: 'Receiving update', streaming: true, fallback: true });
      }
      break;
    }
    const contentStart = start + LIKEABLE_NOTIFICATION_START.length;
    const end = value.indexOf(LIKEABLE_NOTIFICATION_END, contentStart);
    if (end === -1) {
      const body = value.slice(contentStart).trim();
      segments.push({ body: body || 'Receiving update', streaming: true, fallback: body.length === 0 });
      break;
    }
    const body = value.slice(contentStart, end).trim();
    if (body) segments.push({ body, streaming: false });
    cursor = end + LIKEABLE_NOTIFICATION_END.length;
  }
  return segments;
}

export function feedAwaitingAgent(feed: Feed | null): boolean {
  if (!feed) return false;
  if (feed.live?.isProcessing) return true;
  const latestUser = latestTimestamp((feed.localMessages ?? []).filter((msg) => msg.role === 'user').map((msg) => msg.createdAt));
  if (latestUser == null) return false;

  const latestAgent = latestTimestamp([
    ...(feed.localMessages ?? []).filter((msg) => msg.role !== 'user').map((msg) => msg.createdAt),
    ...(feed.messages ?? []).filter((msg) => msg.role !== 'user').map((msg) => msg.created_at),
    ...(feed.activity ?? []).map((activity) => activity.occurred_at)
  ]);
  if (latestAgent != null) return latestAgent < latestUser;

  return true;
}

function latestTimestamp(values: Array<string | undefined>): number | null {
  let latest: number | null = null;
  for (const value of values) {
    const timestamp = Date.parse(value ?? '');
    if (Number.isNaN(timestamp)) continue;
    latest = latest == null ? timestamp : Math.max(latest, timestamp);
  }
  return latest;
}
