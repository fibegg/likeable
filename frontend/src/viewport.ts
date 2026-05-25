import { SINGLE_VIEW_QUERY } from './config';

const BASIC_CHAT_MIN_HEIGHT = 260;
const BASIC_CHAT_DEFAULT_MIN_HEIGHT = 320;
const BASIC_CHAT_DEFAULT_MAX_HEIGHT = 640;
const BASIC_CHAT_DESKTOP_EDGE_GUARD = 28;
const BASIC_CHAT_MOBILE_TOP_GUARD = 118;
const KEYBOARD_INSET_THRESHOLD = 80;
const KEYBOARD_RECOVERY_MS = 1800;

let stableViewportHeight = 0;
let preserveStableViewportUntil = 0;

export function singleViewScreen() {
  return typeof window !== 'undefined' && window.matchMedia(SINGLE_VIEW_QUERY).matches;
}

export function currentViewportHeight() {
  if (typeof window === 'undefined') return 0;
  const visualViewport = window.visualViewport;
  if (!visualViewport) return window.innerHeight;
  if (standaloneDisplayMode()) return currentLayoutViewportHeight();
  const keyboardInset = currentKeyboardInset();
  return keyboardInset > KEYBOARD_INSET_THRESHOLD ? currentLayoutViewportHeight() : visualViewport.height;
}

export function currentViewportWidth() {
  if (typeof window === 'undefined') return 0;
  return window.visualViewport?.width ?? window.innerWidth;
}

export function installViewportCssVars() {
  if (typeof window === 'undefined') return () => undefined;
  const root = document.documentElement;
  const visualViewport = window.visualViewport;
  let frame = 0;
  let timers: number[] = [];

  const update = () => {
    frame = 0;
    const visualHeight = Math.round(visualViewport?.height ?? window.innerHeight);
    const height = Math.round(currentViewportHeight());
    const width = Math.round(currentViewportWidth());
    const offsetTop = Math.round(visualViewportOffsetTop());
    const keyboardInset = Math.round(currentKeyboardInset());
    const keyboardOverlayInset = Math.round(currentKeyboardOverlayInset(keyboardInset));
    root.style.setProperty('--app-viewport-height', `${height}px`);
    root.style.setProperty('--app-visual-viewport-height', `${visualHeight}px`);
    root.style.setProperty('--app-viewport-width', `${width}px`);
    root.style.setProperty('--app-viewport-offset-top', `${offsetTop}px`);
    root.style.setProperty('--app-keyboard-inset', `${keyboardInset}px`);
    root.style.setProperty('--app-keyboard-overlay-inset', `${keyboardOverlayInset}px`);
    root.dataset.keyboardOpen = keyboardInset > KEYBOARD_INSET_THRESHOLD ? 'true' : 'false';
  };

  const scheduleUpdate = () => {
    if (frame) cancelAnimationFrame(frame);
    frame = requestAnimationFrame(update);
    timers.forEach((timer) => window.clearTimeout(timer));
    timers = [80, 240, 480, 900, 1500].map((delay) => window.setTimeout(update, delay));
  };

  scheduleUpdate();
  window.addEventListener('resize', scheduleUpdate);
  window.addEventListener('orientationchange', scheduleUpdate);
  window.addEventListener('blur', scheduleUpdate);
  window.addEventListener('focusin', scheduleUpdate);
  window.addEventListener('focusout', scheduleUpdate);
  document.addEventListener('visibilitychange', scheduleUpdate);
  visualViewport?.addEventListener('resize', scheduleUpdate);
  visualViewport?.addEventListener('scroll', scheduleUpdate);

  return () => {
    if (frame) cancelAnimationFrame(frame);
    timers.forEach((timer) => window.clearTimeout(timer));
    window.removeEventListener('resize', scheduleUpdate);
    window.removeEventListener('orientationchange', scheduleUpdate);
    window.removeEventListener('blur', scheduleUpdate);
    window.removeEventListener('focusin', scheduleUpdate);
    window.removeEventListener('focusout', scheduleUpdate);
    document.removeEventListener('visibilitychange', scheduleUpdate);
    visualViewport?.removeEventListener('resize', scheduleUpdate);
    visualViewport?.removeEventListener('scroll', scheduleUpdate);
  };
}

function visualViewportOffsetTop() {
  if (typeof window === 'undefined') return 0;
  return window.visualViewport?.offsetTop ?? 0;
}

function currentKeyboardInset() {
  if (typeof window === 'undefined') return 0;
  const visualViewport = window.visualViewport;
  if (!visualViewport) return 0;
  if (!textInputFocused()) return 0;
  return Math.max(0, currentLayoutViewportHeight() - visualViewport.height - visualViewportOffsetTop());
}

function currentKeyboardOverlayInset(keyboardInset: number) {
  if (typeof window === 'undefined') return 0;
  const visualViewport = window.visualViewport;
  if (!visualViewport || keyboardInset <= KEYBOARD_INSET_THRESHOLD) return 0;
  return Math.max(0, window.innerHeight - visualViewport.height - visualViewportOffsetTop());
}

function currentLayoutViewportHeight() {
  if (typeof window === 'undefined') return 0;
  const height = window.innerHeight;
  const visualHeight = window.visualViewport?.height ?? height;
  const currentHeight = Math.max(height, visualHeight);
  const focused = textInputFocused();
  if (!stableViewportHeight) {
    stableViewportHeight = currentHeight;
  } else if (currentHeight > stableViewportHeight) {
    stableViewportHeight = currentHeight;
  } else {
    const now = Date.now();
    const viewportShrunkFromStable = stableViewportHeight - currentHeight > KEYBOARD_INSET_THRESHOLD;
    if (focused && viewportShrunkFromStable) {
      preserveStableViewportUntil = now + KEYBOARD_RECOVERY_MS;
    }
    if (!viewportShrunkFromStable || now >= preserveStableViewportUntil) {
      stableViewportHeight = currentHeight;
    }
  }
  return stableViewportHeight;
}

function textInputFocused() {
  if (typeof document === 'undefined') return false;
  const active = document.activeElement;
  if (!(active instanceof HTMLElement)) return false;
  return Boolean(active.closest('textarea, input, select, [contenteditable="true"]'));
}

function standaloneDisplayMode() {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') return false;
  const navigatorWithStandalone = navigator as Navigator & { standalone?: boolean };
  return window.matchMedia?.('(display-mode: standalone)').matches || navigatorWithStandalone.standalone === true;
}

export function defaultBasicChatHeight() {
  if (typeof window === 'undefined') return 520;
  const preferred = Math.min(BASIC_CHAT_DEFAULT_MAX_HEIGHT, Math.max(BASIC_CHAT_DEFAULT_MIN_HEIGHT, Math.round(currentViewportHeight() * 0.58)));
  return clampBasicChatHeight(preferred);
}

export function clampBasicChatHeight(value: number) {
  if (typeof window === 'undefined') return value;
  const topGuard = singleViewScreen() ? BASIC_CHAT_MOBILE_TOP_GUARD : BASIC_CHAT_DESKTOP_EDGE_GUARD;
  const maxHeight = Math.max(BASIC_CHAT_MIN_HEIGHT, currentViewportHeight() - topGuard);
  return Math.min(maxHeight, Math.max(BASIC_CHAT_MIN_HEIGHT, value));
}
