package likeable

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	fibegateway "github.com/fibegg/likeable/internal/fibe"
)

const notificationTurnTimeSlop = 5 * time.Second

type projectNotificationRow struct {
	ID     string
	Body   string
	Time   string
	Active bool
}

type likeableNotificationSegment struct {
	Body      string
	Streaming bool
	Fallback  bool
}

func (s *Server) syncProjectNotificationTimings(ctx context.Context, project *Project, local []Message, messages []any, activity []any, live *fibegateway.ConversationLiveState) (map[string]ProjectNotificationTiming, bool, error) {
	return s.syncProjectNotificationTimingsAt(ctx, project, local, messages, activity, live, time.Now().UTC())
}

func (s *Server) syncProjectNotificationTimingsAt(ctx context.Context, project *Project, local []Message, messages []any, activity []any, live *fibegateway.ConversationLiveState, observedAt time.Time) (map[string]ProjectNotificationTiming, bool, error) {
	shouldContinue := projectNotificationMonitorShouldContinue(live)
	if project == nil {
		return map[string]ProjectNotificationTiming{}, shouldContinue, nil
	}
	rows := projectNotificationRows(local, messages, activity, live)
	timings, err := s.store.ProjectNotificationTimingMap(ctx, project.ID)
	if err != nil {
		return nil, shouldContinue, err
	}
	if len(rows) == 0 {
		return timings, shouldContinue, nil
	}
	observedAt = observedAt.UTC()
	for _, row := range rows {
		if _, ok := timings[row.ID]; ok {
			continue
		}
		if err := s.store.UpsertProjectNotificationStarted(ctx, project.ID, row.ID, row.Body, observedAt); err != nil {
			return nil, shouldContinue, err
		}
	}
	timings, err = s.store.ProjectNotificationTimingMap(ctx, project.ID)
	if err != nil {
		return nil, shouldContinue, err
	}
	for i, row := range rows {
		timing, ok := timings[row.ID]
		if !ok || strings.TrimSpace(timing.CompletedAt) != "" {
			continue
		}
		if row.Active {
			continue
		}
		completedAt := observedAt
		nextInSequence := i+1 < len(rows) && sameNotificationSequence(row.ID, rows[i+1].ID)
		if nextInSequence {
			if nextTiming, ok := timings[rows[i+1].ID]; ok {
				if parsed, ok := parseProjectNotificationTime(nextTiming.StartedAt); ok {
					completedAt = parsed
				}
			}
		} else if i+1 >= len(rows) && shouldContinue {
			continue
		} else if parsed, ok := parseProjectNotificationTime(row.Time); ok && parsed.After(completedAt) {
			completedAt = parsed
		}
		if !nextInSequence {
			elapsedMs := projectNotificationSummaryElapsedMs(rows, timings, row.ID, completedAt)
			if err := s.store.CompleteProjectNotificationTimingWithElapsed(ctx, project.ID, row.ID, completedAt, elapsedMs); err != nil {
				return nil, shouldContinue, err
			}
			continue
		}
		if err := s.store.CompleteProjectNotificationTiming(ctx, project.ID, row.ID, completedAt); err != nil {
			return nil, shouldContinue, err
		}
	}
	timings, err = s.store.ProjectNotificationTimingMap(ctx, project.ID)
	return timings, shouldContinue, err
}

func projectNotificationRows(local []Message, messages []any, activity []any, live *fibegateway.ConversationLiveState) []projectNotificationRow {
	userTimes := notificationUserTimes(local)
	latestTurnKey := ""
	var latestUserTime time.Time
	hasLatestUserTime := false
	if len(userTimes) > 0 {
		latestUserTime = userTimes[len(userTimes)-1]
		hasLatestUserTime = true
		latestTurnKey = notificationTurnKey(latestUserTime)
	}

	assistantRows := assistantNotificationRows(userTimes, messages)
	activityRows := activityNotificationRows(activity)
	rows := dedupeProjectNotifications(append(assistantRows, activityRows...))
	if live == nil || strings.TrimSpace(live.StreamText) == "" {
		return rows
	}
	liveIdle := !live.IsProcessing && live.QueuedTurns <= 0
	liveTurnKey := latestTurnKey
	if liveTurnKey == "" {
		liveTurnKey = "live"
	}
	liveTime := liveNotificationRowTime(live.StartedAt, latestUserTime, hasLatestUserTime)
	segments := parseLikeableNotificationSegments(live.StreamText)
	lastLiveIndex := len(segments) - 1
	for segmentIndex, segment := range segments {
		if !segment.Streaming && durableNotificationCovers(rows, segment.Body, liveTime) {
			continue
		}
		isLast := segmentIndex == lastLiveIndex
		rows = append(rows, projectNotificationRow{
			ID:     fmt.Sprintf("%s-notification-%d", liveTurnKey, segmentIndex),
			Body:   segment.Body,
			Time:   liveTime,
			Active: isLast && !liveIdle && (live.IsProcessing || segment.Streaming),
		})
	}
	return rows
}

func projectNotificationSummaryElapsedMs(rows []projectNotificationRow, timings map[string]ProjectNotificationTiming, notificationID string, completedAt time.Time) int64 {
	startedAt, ok := projectNotificationSequenceStartedAt(rows, timings, notificationID)
	if !ok {
		timing, ok := timings[notificationID]
		if !ok {
			return 0
		}
		startedAt, ok = parseProjectNotificationTime(timing.StartedAt)
		if !ok {
			return 0
		}
	}
	if completedAt.Before(startedAt) {
		return 0
	}
	return completedAt.Sub(startedAt).Milliseconds()
}

func projectNotificationSequenceStartedAt(rows []projectNotificationRow, timings map[string]ProjectNotificationTiming, notificationID string) (time.Time, bool) {
	sequenceKey, ok := notificationSequenceKey(notificationID)
	if !ok {
		return time.Time{}, false
	}
	var startedAt time.Time
	found := false
	for _, row := range rows {
		rowSequenceKey, ok := notificationSequenceKey(row.ID)
		if !ok || rowSequenceKey != sequenceKey {
			continue
		}
		timing, ok := timings[row.ID]
		if !ok {
			continue
		}
		parsed, ok := parseProjectNotificationTime(timing.StartedAt)
		if !ok {
			continue
		}
		if !found || parsed.Before(startedAt) {
			startedAt = parsed
			found = true
		}
	}
	return startedAt, found
}

func notificationSequenceKey(notificationID string) (string, bool) {
	prefix, _, ok := strings.Cut(notificationID, "-notification-")
	return prefix, ok
}

func sameNotificationSequence(leftID, rightID string) bool {
	leftKey, leftOK := notificationSequenceKey(leftID)
	rightKey, rightOK := notificationSequenceKey(rightID)
	return leftOK && rightOK && leftKey == rightKey
}

type assistantNotificationSource struct {
	index   int
	timeRaw string
	time    time.Time
	hasTime bool
	role    string
	body    string
}

func assistantNotificationRows(userTimes []time.Time, messages []any) []projectNotificationRow {
	sources := make([]assistantNotificationSource, 0, len(messages))
	for index, item := range messages {
		message := mapFromAny(item)
		role := anyString(message["role"])
		body := anyString(message["body"])
		if body == "" {
			body = anyString(message["content"])
		}
		timeRaw := anyString(message["created_at"])
		parsed, hasTime := parseProjectNotificationTime(timeRaw)
		if role != "assistant" {
			continue
		}
		sources = append(sources, assistantNotificationSource{index: index, timeRaw: timeRaw, time: parsed, hasTime: hasTime, role: role, body: body})
	}
	sort.SliceStable(sources, func(i, j int) bool {
		left, right := sources[i], sources[j]
		if left.hasTime && right.hasTime && !left.time.Equal(right.time) {
			return left.time.Before(right.time)
		}
		if left.hasTime != right.hasTime {
			return left.hasTime
		}
		return left.index < right.index
	})
	rows := []projectNotificationRow{}
	counters := map[string]int{}
	for _, source := range sources {
		turnKey, ok := notificationTurnKeyForTime(userTimes, source.time, source.hasTime)
		if !ok {
			turnKey = fmt.Sprintf("assistant-%d", source.index)
		}
		for _, segment := range parseLikeableNotificationSegments(source.body) {
			segmentIndex := counters[turnKey]
			counters[turnKey] = segmentIndex + 1
			rows = append(rows, projectNotificationRow{
				ID:   fmt.Sprintf("%s-notification-%d", turnKey, segmentIndex),
				Body: segment.Body,
				Time: source.timeRaw,
			})
		}
	}
	return rows
}

func activityNotificationRows(activity []any) []projectNotificationRow {
	rows := []projectNotificationRow{}
	for index, item := range activity {
		entry := mapFromAny(item)
		sourceID := firstNonEmpty(anyString(entry["id"]), anyString(entry["occurred_at"]), fmt.Sprintf("activity-%d", index))
		body := anyString(entry["message"])
		for segmentIndex, segment := range parseLikeableNotificationSegments(body) {
			rows = append(rows, projectNotificationRow{
				ID:   fmt.Sprintf("activity-%s-notification-%d", sourceID, segmentIndex),
				Body: segment.Body,
				Time: anyString(entry["occurred_at"]),
			})
		}
	}
	return rows
}

func notificationUserTimes(local []Message) []time.Time {
	times := []time.Time{}
	for _, msg := range local {
		if msg.Role != "user" {
			continue
		}
		if parsed, ok := parseProjectNotificationTime(msg.CreatedAt); ok {
			times = append(times, parsed)
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	return times
}

func notificationTurnKeyForTime(userTimes []time.Time, sourceTime time.Time, hasSourceTime bool) (string, bool) {
	if len(userTimes) == 0 {
		return "", false
	}
	if !hasSourceTime {
		return notificationTurnKey(userTimes[len(userTimes)-1]), true
	}
	selected := userTimes[0]
	limit := sourceTime.Add(notificationTurnTimeSlop)
	for _, candidate := range userTimes {
		if !candidate.After(limit) {
			selected = candidate
			continue
		}
		break
	}
	return notificationTurnKey(selected), true
}

func notificationTurnKey(value time.Time) string {
	return fmt.Sprintf("turn-%d", value.UTC().UnixMilli())
}

func liveNotificationRowTime(startedAt string, latestUserTime time.Time, hasLatestUserTime bool) string {
	if !hasLatestUserTime {
		return startedAt
	}
	startedTime, ok := parseProjectNotificationTime(startedAt)
	if !ok || !startedTime.After(latestUserTime) {
		return latestUserTime.Add(time.Millisecond).UTC().Format(time.RFC3339Nano)
	}
	return startedAt
}

func dedupeProjectNotifications(rows []projectNotificationRow) []projectNotificationRow {
	seen := map[string]bool{}
	out := []projectNotificationRow{}
	for _, row := range rows {
		body := normalizeNotificationBody(row.Body)
		if body == "" {
			continue
		}
		parsed, ok := parseProjectNotificationTime(row.Time)
		timeBucket := ""
		if ok {
			timeBucket = fmt.Sprintf("%d", parsed.UnixMilli()/2000)
		}
		key := body + ":" + timeBucket
		if seen[key] {
			continue
		}
		seen[key] = true
		row.Body = body
		out = append(out, row)
	}
	return out
}

func durableNotificationCovers(rows []projectNotificationRow, body string, timeRaw string) bool {
	normalized := normalizeNotificationBody(body)
	if normalized == "" {
		return false
	}
	targetTime, hasTargetTime := parseProjectNotificationTime(timeRaw)
	for _, row := range rows {
		if row.Active {
			continue
		}
		if hasTargetTime {
			if rowTime, ok := parseProjectNotificationTime(row.Time); ok {
				delta := rowTime.Sub(targetTime)
				if delta < 0 {
					delta = -delta
				}
				if delta > time.Minute {
					continue
				}
			}
		}
		candidate := normalizeNotificationBody(row.Body)
		if candidate == normalized || strings.HasPrefix(candidate, normalized) || strings.HasPrefix(normalized, candidate) {
			return true
		}
	}
	return false
}

func parseLikeableNotificationSegments(value string) []likeableNotificationSegment {
	segments := []likeableNotificationSegment{}
	cursor := 0
	for cursor < len(value) {
		start := strings.Index(value[cursor:], likeableNotificationStart)
		if start == -1 {
			if strings.Contains(value[cursor:], "[[LIKEABLE") {
				segments = append(segments, likeableNotificationSegment{Body: "Receiving update", Streaming: true, Fallback: true})
			}
			break
		}
		start += cursor
		contentStart := start + len(likeableNotificationStart)
		end := strings.Index(value[contentStart:], likeableNotificationEnd)
		if end == -1 {
			body := strings.TrimSpace(value[contentStart:])
			fallback := false
			if body == "" {
				body = "Receiving update"
				fallback = true
			}
			segments = append(segments, likeableNotificationSegment{Body: body, Streaming: true, Fallback: fallback})
			break
		}
		end += contentStart
		body := strings.TrimSpace(value[contentStart:end])
		if body != "" {
			segments = append(segments, likeableNotificationSegment{Body: body})
		}
		cursor = end + len(likeableNotificationEnd)
	}
	return segments
}

func projectNotificationMonitorShouldContinue(live *fibegateway.ConversationLiveState) bool {
	return live != nil && (live.IsProcessing || live.QueuedTurns > 0)
}

func parseProjectNotificationTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, raw)
	}
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func normalizeNotificationBody(body string) string {
	return strings.Join(strings.Fields(body), " ")
}

func mapFromAny(value any) map[string]any {
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}
