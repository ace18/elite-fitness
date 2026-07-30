// session-draft.js — l'allenamento in corso, salvato in locale.
//
// Perché serve: tutto lo stato del logger vive in `$state` dentro
// routes/session/+page.svelte, e il riassunto per la ricevuta in uno store in
// memoria. Bastava un refresh — o iOS che scarica dalla memoria una PWA in
// background per qualche minuto — e l'allenamento svaniva. Succedeva anche con
// la rete perfetta: non è un problema di offline, è un problema di durabilità.
//
// localStorage e non IndexedDB: qui si scrive un oggetto piccolo, spesso, e
// serve che la scrittura sia già su disco quando l'app viene uccisa. Una write
// sincrona dà esattamente questo; IndexedDB è asincrono e l'ultima transazione
// può non concludersi. L'outbox delle sessioni da spedire è un altro problema
// (là servono code e ritentativi) e userà IndexedDB.

const DRAFT_KEY = 'elite.session.draft';
const SUMMARY_KEY = 'elite.session.summary';

// Oltre questo un "allenamento in corso" non è più credibile: è roba
// dimenticata, e riprenderla settimane dopo produrrebbe una sessione con
// durata assurda e pesi non più attuali.
const MAX_AGE_MS = 24 * 60 * 60 * 1000;

// Se la forma del draft cambia, alza la versione: i draft vecchi vengono
// scartati invece di far esplodere il ripristino con campi che non esistono più.
const SCHEMA = 1;

// localStorage lancia in modalità privata e quando la quota è piena. Un
// fallimento qui non deve mai impedire di allenarsi: si perde la durabilità,
// non la sessione in corso.
function safe(fn, fallback = null) {
  try {
    return fn();
  } catch {
    return fallback;
  }
}

function read(key) {
  return safe(() => {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    const obj = JSON.parse(raw);
    if (obj?.v !== SCHEMA) return null;
    if (!obj.savedAt || Date.now() - obj.savedAt > MAX_AGE_MS) return null;
    return obj;
  });
}

function write(key, payload) {
  return safe(() => {
    localStorage.setItem(key, JSON.stringify({ ...payload, v: SCHEMA, savedAt: Date.now() }));
    return true;
  }, false);
}

// ---- allenamento in corso -------------------------------------------------

// Il draft porta dentro anche il workout completo, non solo gli indici: dopo un
// refresh gli store sono vuoti e `reloadAll()` può non arrivare mai (offline),
// quindi senza lo snapshot non si potrebbero ricostruire nomi, target e
// suggerimenti degli esercizi.
export function saveDraft(draft) {
  return write(DRAFT_KEY, draft);
}

export function loadDraft() {
  return read(DRAFT_KEY);
}

export function clearDraft() {
  safe(() => localStorage.removeItem(DRAFT_KEY));
}

// Per la schermata Allena, che deve poter offrire "riprendi" senza caricare
// tutto il draft.
export function draftInfo() {
  const d = loadDraft();
  if (!d) return null;
  return {
    name: d.workout?.name ?? null,
    doneSets: (d.logged ?? []).reduce((a, l) => a + l.length, 0),
    savedAt: d.savedAt
  };
}

// ---- riassunto in attesa di salvataggio -----------------------------------
//
// Fra la fine dell'allenamento e il POST andato a buon fine il riassunto è
// l'unica copia dei dati: va persistito anche quello, altrimenti un salvataggio
// fallito seguito dalla chiusura dell'app perde tutto — ed è esattamente lo
// scenario di una palestra senza campo.

export function saveSummary(summary) {
  return write(SUMMARY_KEY, { summary });
}

export function loadSummary() {
  return read(SUMMARY_KEY)?.summary ?? null;
}

export function clearSummary() {
  safe(() => localStorage.removeItem(SUMMARY_KEY));
}
