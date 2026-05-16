import { SINGLE_VIEW_QUERY } from './config';

export function singleViewScreen() {
  return typeof window !== 'undefined' && window.matchMedia(SINGLE_VIEW_QUERY).matches;
}

export function currentViewportHeight() {
  if (typeof window === 'undefined') return 0;
  return window.visualViewport?.height ?? window.innerHeight;
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
    const height = currentViewportHeight();
    const width = currentViewportWidth();
    const offsetTop = visualViewport?.offsetTop ?? 0;
    const keyboardInset = Math.max(0, window.innerHeight - height - offsetTop);
    root.style.setProperty('--app-viewport-height', `${height}px`);
    root.style.setProperty('--app-viewport-width', `${width}px`);
    root.style.setProperty('--app-viewport-offset-top', `${offsetTop}px`);
    root.style.setProperty('--app-keyboard-inset', `${keyboardInset}px`);
  };

  const scheduleUpdate = () => {
    if (frame) cancelAnimationFrame(frame);
    frame = requestAnimationFrame(update);
    timers.forEach((timer) => window.clearTimeout(timer));
    timers = [80, 240, 480].map((delay) => window.setTimeout(update, delay));
  };

  scheduleUpdate();
  window.addEventListener('resize', scheduleUpdate);
  window.addEventListener('orientationchange', scheduleUpdate);
  window.addEventListener('focusin', scheduleUpdate);
  window.addEventListener('focusout', scheduleUpdate);
  visualViewport?.addEventListener('resize', scheduleUpdate);
  visualViewport?.addEventListener('scroll', scheduleUpdate);

  return () => {
    if (frame) cancelAnimationFrame(frame);
    timers.forEach((timer) => window.clearTimeout(timer));
    window.removeEventListener('resize', scheduleUpdate);
    window.removeEventListener('orientationchange', scheduleUpdate);
    window.removeEventListener('focusin', scheduleUpdate);
    window.removeEventListener('focusout', scheduleUpdate);
    visualViewport?.removeEventListener('resize', scheduleUpdate);
    visualViewport?.removeEventListener('scroll', scheduleUpdate);
  };
}

export function defaultBasicChatHeight() {
  if (typeof window === 'undefined') return 520;
  return Math.min(640, Math.max(320, Math.round(currentViewportHeight() * 0.58)));
}

export function clampBasicChatHeight(value: number) {
  if (typeof window === 'undefined') return value;
  return Math.min(Math.max(300, currentViewportHeight() - 28), Math.max(260, value));
}
