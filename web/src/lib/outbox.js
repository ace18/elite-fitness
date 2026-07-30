// outbox.js — coda delle scritture che non sono riuscite a partire.
//
// Un allenamento finito in palestra senza campo non deve restare in attesa che
// l'utente ritorni a mano sulla ricevuta: entra qui e riparte da solo quando la
// rete torna.
//
// IndexedDB e non localStorage, al contrario della bozza in session-draft.js: là
// serve una write sincrona di un oggetto piccolo, qui una coda di oggetti grandi
// (il log completo delle serie) da leggere e cancellare a transazioni. E il
// limite di ~5MB di localStorage lo si raggiunge con una manciata di
// allenamenti in coda.
//
// Ogni voce porta l'utente a cui appartiene: sullo stesso telefono possono
// alternarsi due account, e la coda di uno non deve MAI partire col token
// dell'altro. Al logout la coda non si cancella — i dati restano, semplicemente
// non vengono spediti finché quell'utente non rientra.

const DB_NAME = 'elite-outbox';
const STORE = 'requests';
const DB_VERSION = 1;

// Oltre questo una voce non vale più la pena: iOS può comunque sgomberare
// IndexedDB di una PWA inutilizzata dopo circa una settimana, quindi la coda è
// un buffer, non un archivio.
const MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000;

// Dopo troppi tentativi falliti per un motivo non di rete, insistere è inutile.
const MAX_ATTEMPTS = 20;

function openDB() {
  return new Promise((resolve, reject) => {
    if (!globalThis.indexedDB) {
      reject(new Error('IndexedDB non disponibile'));
      return;
    }
    const req = indexedDB.open(DB_NAME, DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: 'id', autoIncrement: true });
        store.createIndex('user', 'user');
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

function tx(db, mode, fn) {
  return new Promise((resolve, reject) => {
    const t = db.transaction(STORE, mode);
    const store = t.objectStore(STORE);
    let out;
    try {
      out = fn(store);
    } catch (e) {
      reject(e);
      return;
    }
    // `fn` può restituire una promise (una richiesta avvolta): si risolve
    // quella, non l'oggetto promise, così chi chiama fa una sola await.
    t.oncomplete = () => Promise.resolve(out).then(resolve, reject);
    t.onerror = () => reject(t.error);
    t.onabort = () => reject(t.error);
  });
}

const wrap = (req) =>
  new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });

// IndexedDB può essere negato (modalità privata, quota). Come per la bozza, un
// fallimento qui non deve impedire di usare l'app: si perde la coda, non la
// sessione in corso — che resta comunque nel riassunto persistito.
async function safe(fn, fallback) {
  try {
    return await fn();
  } catch {
    return fallback;
  }
}

// enqueue mette in coda una scrittura. `kind` dice a chi la spedisce come
// rimandarla (vedi i senders in api.js).
export async function enqueue(user, kind, body) {
  return safe(async () => {
    const db = await openDB();
    await tx(db, 'readwrite', (s) => s.add({ user, kind, body, createdAt: Date.now(), attempts: 0 }));
    db.close();
    return true;
  }, false);
}

// pending restituisce le voci di un utente, dalla più vecchia. Le voci scadute
// vengono buttate qui: è l'unico punto che le rilegge tutte.
export async function pending(user) {
  return safe(async () => {
    const db = await openDB();
    const items = await tx(db, 'readonly', (s) => wrap(s.index('user').getAll(user)));
    const now = Date.now();
    const fresh = [];
    const stale = [];
    for (const it of items) {
      if (now - it.createdAt > MAX_AGE_MS || it.attempts >= MAX_ATTEMPTS) stale.push(it.id);
      else fresh.push(it);
    }
    if (stale.length) await tx(db, 'readwrite', (s) => stale.forEach((id) => s.delete(id)));
    db.close();
    return fresh.sort((a, b) => a.createdAt - b.createdAt);
  }, []);
}

export async function count(user) {
  return (await pending(user)).length;
}

export async function remove(id) {
  return safe(async () => {
    const db = await openDB();
    await tx(db, 'readwrite', (s) => s.delete(id));
    db.close();
    return true;
  }, false);
}

// bumpAttempts serve a non ritentare all'infinito una voce che il server
// rifiuta per un motivo suo: dopo MAX_ATTEMPTS pending() la scarta.
export async function bumpAttempts(id, lastError) {
  return safe(async () => {
    const db = await openDB();
    await tx(db, 'readwrite', (s) => {
      const get = s.get(id);
      get.onsuccess = () => {
        const it = get.result;
        if (!it) return;
        it.attempts = (it.attempts ?? 0) + 1;
        it.lastError = lastError ?? null;
        s.put(it);
      };
    });
    db.close();
    return true;
  }, false);
}

// flush ritenta le voci dell'utente in ordine. `senders[kind]` deve fare la
// richiesta e lanciare in caso di errore.
//
// Distinguere i due tipi di errore è il punto: un errore di rete significa
// "riprova più tardi" e la voce resta; una risposta del server (4xx/5xx) è una
// risposta, quindi si conta il tentativo — e con un 4xx la voce va scartata,
// perché rimandarla identica darà sempre lo stesso esito.
export async function flush(user, senders, { isNetworkError, isPermanent }) {
  const items = await pending(user);
  let sent = 0;
  for (const item of items) {
    const send = senders[item.kind];
    if (!send) {
      // Tipo sconosciuto (coda scritta da una versione più nuova): non si può
      // spedire e non si deve trattenere per sempre.
      await remove(item.id);
      continue;
    }
    try {
      await send(item.body);
      await remove(item.id);
      sent++;
    } catch (e) {
      if (isNetworkError(e)) break; // rete giù: inutile provare le successive
      if (isPermanent(e)) {
        await remove(item.id);
        continue;
      }
      await bumpAttempts(item.id, e?.message ?? String(e));
    }
  }
  return { sent, remaining: await count(user) };
}

// clearAll — solo per i test e per un reset manuale.
export async function clearAll() {
  return safe(async () => {
    const db = await openDB();
    await tx(db, 'readwrite', (s) => s.clear());
    db.close();
    return true;
  }, false);
}
