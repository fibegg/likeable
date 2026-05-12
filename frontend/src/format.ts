import type { User } from './domain';

export function formatMessageTime(value?: string): string {
  const date = new Date(value ?? '');
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function formatResetCountdown(value?: string, now = Date.now()): string {
  const target = Date.parse(value ?? '');
  if (Number.isNaN(target)) return '5h';
  const remainingMs = Math.max(0, target - now);
  if (remainingMs <= 0) return 'less than 1m';
  const totalMinutes = Math.max(1, Math.ceil(remainingMs / 60000));
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

export function formatElapsedDuration(ms?: number): string {
  if (typeof ms !== 'number' || !Number.isFinite(ms) || ms < 0) return '';
  const totalSeconds = Math.max(1, Math.round(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) return `${seconds}s`;
  return `${minutes}m${String(seconds).padStart(2, '0')}s`;
}

export function projectLaunchErrorMessage(_value?: string): string {
  return 'The canvas could not start. Check workspace settings in Admin, then create a new project.';
}

export function formatShortDate(value?: string): string {
  const date = new Date(value ?? '');
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
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
