import type { MouseEvent } from 'react';

export function openExternalLinkFromTap(event: MouseEvent<HTMLAnchorElement>, url: string) {
  if (event.defaultPrevented || event.button !== 0 || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;

  const opened = window.open(url, '_blank', 'noopener,noreferrer');
  if (opened) {
    event.preventDefault();
  }
}
