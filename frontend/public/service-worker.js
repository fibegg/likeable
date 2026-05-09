// Likeable Service Worker
// Network-first for navigation with an app-shell fallback, stale-while-revalidate
// for Vite fingerprinted assets, and cache-first for static install resources.

const VERSION = "v1";
const CACHE_PREFIX = "likeable-pwa";
const APP_CACHE = `${CACHE_PREFIX}-app-${VERSION}`;
const RUNTIME_CACHE = `${CACHE_PREFIX}-runtime-${VERSION}`;
const OFFLINE_URL = "/offline.html";
const APP_SHELL_URLS = [
  "/",
  OFFLINE_URL,
  "/manifest.webmanifest",
  "/icon.svg",
  "/icon-192.png",
  "/icon-512.png",
  "/maskable-icon-512.png",
  "/apple-touch-icon.png"
];

function sameOrigin(url) {
  return url.origin === self.location.origin;
}

function isFingerprintAsset(url) {
  return /^\/assets\/.+-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$/.test(url.pathname);
}

function isStaticResource(url) {
  return /\.(?:css|js|mjs|woff2?|ttf|otf|eot|svg|png|jpg|jpeg|gif|ico|webp|avif)(?:\?|$)/.test(url.pathname);
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(APP_CACHE).then((cache) =>
      cache.addAll(APP_SHELL_URLS.map((url) => new Request(url, { cache: "reload" })))
    )
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key.startsWith(CACHE_PREFIX) && key !== APP_CACHE && key !== RUNTIME_CACHE)
          .map((key) => caches.delete(key))
      )
    )
  );
  self.clients.claim();
});

self.addEventListener("message", (event) => {
  if (event.data && event.data.type === "SKIP_WAITING") {
    self.skipWaiting();
  }
});

self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") return;

  const url = new URL(event.request.url);
  if (!sameOrigin(url)) return;

  if (event.request.mode === "navigate") {
    event.respondWith(networkFirstNavigation(event.request));
    return;
  }

  if (isFingerprintAsset(url)) {
    event.respondWith(staleWhileRevalidate(event.request));
    return;
  }

  if (isStaticResource(url) || APP_SHELL_URLS.includes(url.pathname)) {
    event.respondWith(cacheFirst(event.request));
  }
});

async function networkFirstNavigation(request) {
  const cache = await caches.open(APP_CACHE);
  try {
    const response = await fetch(request);
    if (response.ok) {
      await cache.put(request, response.clone());
      await cache.put("/", response.clone());
    }
    return response;
  } catch (_) {
    return (
      (await cache.match(request)) ||
      (await cache.match("/")) ||
      (await cache.match(OFFLINE_URL)) ||
      new Response("Offline", { status: 503, headers: { "Content-Type": "text/plain; charset=utf-8" } })
    );
  }
}

async function staleWhileRevalidate(request) {
  const cache = await caches.open(RUNTIME_CACHE);
  const cached = await cache.match(request);
  const refreshed = fetch(request)
    .then((response) => {
      if (response.ok) {
        cache.put(request, response.clone());
      }
      return response;
    })
    .catch(() => cached);

  return cached || refreshed;
}

async function cacheFirst(request) {
  const cache = await caches.open(APP_CACHE);
  const cached = await cache.match(request);
  if (cached) return cached;

  const response = await fetch(request);
  if (response.ok) {
    await cache.put(request, response.clone());
  }
  return response;
}
