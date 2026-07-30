/// <reference lib="webworker" />
// service-worker.js — fa aprire l'app senza rete.
//
// Senza di questo un'app installata offline non parte affatto: non è solo una
// questione di dati. Durante la verifica dello stage A, con il backend giù, il
// router client non riusciva a scaricare il chunk JS della rotta /receipt e
// SvelteKit mostrava la sua pagina 500 — l'allenamento era salvo in
// localStorage ma non c'era modo di raggiungerlo. Precaricare la shell chiude
// quel buco.
//
// SvelteKit registra questo file da sé e fornisce `build` (i chunk con l'hash
// nel nome), `files` (tutto static/: icone, manifest, font) e `version`.
import { build, files, version } from '$service-worker';

// Due cache separate, di proposito:
//  - gli asset sono versionati e si buttano interi a ogni deploy;
//  - le risposte API no, perché devono poter essere cancellate al logout senza
//    sapere con che versione erano state scritte.
const ASSET_CACHE = `elite-assets-${version}`;
const API_CACHE = 'elite-api';

// La shell. `/` va aggiunto a mano: con adapter-static in modalità SPA
// l'index.html è il fallback e non compare né in `build` né in `files`, ma è
// esattamente il documento che serve per aprire l'app offline.
const SHELL = ['/', ...build, ...files];

self.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(ASSET_CACHE);
      // Uno per uno invece di addAll: addAll fallisce tutto se una sola
      // richiesta non va, e un asset mancante non deve impedire
      // l'installazione del service worker.
      await Promise.all(
        SHELL.map(async (path) => {
          try {
            const res = await fetch(path, { cache: 'no-cache' });
            if (res.ok) await cache.put(path, res);
          } catch {
            /* si riproverà a runtime */
          }
        })
      );
      await self.skipWaiting();
    })()
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      for (const key of await caches.keys()) {
        if (key.startsWith('elite-assets-') && key !== ASSET_CACHE) await caches.delete(key);
      }
      await self.clients.claim();
    })()
  );
});

// Il client chiede di svuotare la cache API al logout: le risposte sono legate
// a un utente ma la cache è dell'origine, quindi lasciarle lì significherebbe
// servire i dati di chi c'era prima al prossimo che entra sullo stesso telefono.
self.addEventListener('message', (event) => {
  if (event.data?.type === 'purge-api-cache') {
    event.waitUntil(caches.delete(API_CACHE));
  }
});

// Le rotte di autenticazione non si cachano mai: contengono token e stato di
// sessione, e una risposta vecchia servita a freddo è peggio di un errore.
const isAuthPath = (pathname) => pathname.startsWith('/api/auth/');

async function networkFirstAPI(request) {
  const cache = await caches.open(API_CACHE);
  try {
    const res = await fetch(request);
    // Solo le 200: cachare un 401 o un 500 vorrebbe dire riproporlo offline
    // come se fosse la verità.
    if (res.ok) cache.put(request, res.clone());
    return res;
  } catch (err) {
    const hit = await cache.match(request);
    if (hit) return hit;
    throw err;
  }
}

async function cacheFirst(request) {
  const cache = await caches.open(ASSET_CACHE);
  const hit = await cache.match(request);
  if (hit) return hit;
  const res = await fetch(request);
  if (res.ok) cache.put(request, res.clone());
  return res;
}

// Navigazione: rete se c'è, altrimenti la shell dalla cache. Il router client
// risolve poi la rotta vera, quindi /train offline apre comunque.
async function navigation(request) {
  try {
    return await fetch(request);
  } catch {
    const cache = await caches.open(ASSET_CACHE);
    return (await cache.match('/')) ?? Response.error();
  }
}

self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Le scritture non passano da qui: se ne occupa la coda in lib/outbox.js, che
  // sa cosa vuol dire rimandare una POST. Un service worker che le ritenta da
  // solo rischierebbe di spedirle due volte.
  if (request.method !== 'GET') return;
  if (url.origin !== self.location.origin) return;

  if (request.mode === 'navigate') {
    event.respondWith(navigation(request));
    return;
  }

  if (url.pathname.startsWith('/api/')) {
    if (isAuthPath(url.pathname)) return; // sempre in rete
    event.respondWith(networkFirstAPI(request));
    return;
  }

  event.respondWith(cacheFirst(request));
});
