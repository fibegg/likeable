import type { User } from './domain';

export function formatMessageTime(value?: string, locale?: string): string {
  const date = new Date(value ?? '');
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
}

type ResetCountdownLabels = {
  fallback: string;
  lessThanMinute: string;
  day: string;
  hour: string;
  minute: string;
};

const defaultResetCountdownLabels: ResetCountdownLabels = {
  fallback: '5h',
  lessThanMinute: 'less than 1m',
  day: 'd',
  hour: 'h',
  minute: 'm'
};

export function formatResetCountdown(value?: string, now = Date.now(), labels: ResetCountdownLabels = defaultResetCountdownLabels): string {
  const target = Date.parse(value ?? '');
  if (Number.isNaN(target)) return labels.fallback;
  const remainingMs = Math.max(0, target - now);
  if (remainingMs <= 0) return labels.lessThanMinute;
  const totalMinutes = Math.max(1, Math.ceil(remainingMs / 60000));
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;
  if (days > 0) return `${days}${labels.day} ${hours}${labels.hour}`;
  if (hours > 0) return `${hours}${labels.hour} ${minutes}${labels.minute}`;
  return `${minutes}${labels.minute}`;
}

type ElapsedDurationLabels = {
  minute: string;
  second: string;
};

const defaultElapsedDurationLabels: ElapsedDurationLabels = {
  minute: 'm',
  second: 's'
};

export function formatElapsedDuration(ms?: number, labels: ElapsedDurationLabels = defaultElapsedDurationLabels): string {
  if (typeof ms !== 'number' || !Number.isFinite(ms) || ms < 0) return '';
  const totalSeconds = Math.max(1, Math.round(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) return `${seconds}${labels.second}`;
  return `${minutes}${labels.minute}${String(seconds).padStart(2, '0')}${labels.second}`;
}

export function formatShortDate(value?: string, locale?: string): string {
  const date = new Date(value ?? '');
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString(locale, { month: 'short', day: 'numeric' });
}

export function userInitials(user?: User | null): string {
  const source = user?.name || user?.email || 'U';
  return source
    .split(/[\s@._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('') || 'U';
}
