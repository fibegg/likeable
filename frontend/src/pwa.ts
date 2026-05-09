const THEME_COLOR = '#061014';

function upsertMeta(name: string, content: string) {
  let meta = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`);
  if (!meta) {
    meta = document.createElement('meta');
    meta.name = name;
    document.head.appendChild(meta);
  }
  meta.content = content;
}

export function syncPwaChrome() {
  const rootStyles = getComputedStyle(document.documentElement);
  const themeColor = rootStyles.getPropertyValue('--bg').trim() || THEME_COLOR;

  upsertMeta('theme-color', themeColor);
  upsertMeta('apple-mobile-web-app-status-bar-style', 'black-translucent');
}

export function installPwa() {
  syncPwaChrome();

  if (!('serviceWorker' in navigator) || !import.meta.env.PROD) return;

  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/service-worker.js', { scope: '/' })
      .then((registration) => {
        void registration.update();
      })
      .catch((error) => {
        console.warn('Likeable service worker registration failed', error);
      });
  });
}
