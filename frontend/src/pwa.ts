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

  let reloadingForUpdate = false;
  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (reloadingForUpdate) return;
    reloadingForUpdate = true;
    window.location.reload();
  });

  window.addEventListener('load', () => {
    const serviceWorkerURL = new URL('/service-worker.js', window.location.origin);
    serviceWorkerURL.searchParams.set('v', currentBuildID());
    navigator.serviceWorker.register(serviceWorkerURL, { scope: '/' })
      .then((registration) => {
        registration.addEventListener('updatefound', () => {
          const worker = registration.installing;
          if (!worker) return;
          worker.addEventListener('statechange', () => {
            if (worker.state === 'installed' && navigator.serviceWorker.controller) {
              worker.postMessage({ type: 'SKIP_WAITING' });
            }
          });
        });
        void registration.update();
      })
      .catch((error) => {
        console.warn('Likeable service worker registration failed', error);
      });
  });
}

function currentBuildID() {
  const script = document.querySelector<HTMLScriptElement>('script[type="module"][src*="/assets/"]');
  const asset = script?.src.match(/\/assets\/([^/?#]+)/)?.[1];
  return asset ?? 'app';
}
