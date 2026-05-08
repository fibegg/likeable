import { SINGLE_VIEW_QUERY } from './config';

export function singleViewScreen() {
  return typeof window !== 'undefined' && window.matchMedia(SINGLE_VIEW_QUERY).matches;
}

export function defaultBasicChatHeight() {
  if (typeof window === 'undefined') return 520;
  return Math.min(640, Math.max(320, Math.round(window.innerHeight * 0.58)));
}

export function clampBasicChatHeight(value: number) {
  if (typeof window === 'undefined') return value;
  return Math.min(Math.max(300, window.innerHeight - 28), Math.max(260, value));
}
