// test-session-draft.js — verifica la persistenza dell'allenamento in corso.
//
// Node semplice, senza framework: è lo stesso approccio di check-i18n.js, e il
// frontend non ha dipendenze runtime da mantenere tali.
//
//   node scripts/test-session-draft.js
//
// Serve perché questo modulo è l'unica cosa che sta fra un allenamento e la sua
// perdita: se scarta un draft valido, l'utente perde il lavoro in silenzio.

let store = {};
let throwOnAccess = false;

globalThis.localStorage = {
  getItem(k) {
    if (throwOnAccess) throw new DOMException('denied');
    return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null;
  },
  setItem(k, v) {
    if (throwOnAccess) throw new DOMException('quota');
    store[k] = String(v);
  },
  removeItem(k) {
    if (throwOnAccess) throw new DOMException('denied');
    delete store[k];
  }
};

const M = await import('../src/lib/session-draft.js');

let failed = 0;
function check(name, cond, detail = '') {
  if (cond) return;
  failed++;
  console.error(`✗ ${name}${detail ? ' — ' + detail : ''}`);
}
function reset() {
  store = {};
  throwOnAccess = false;
}

const draft = () => ({
  workout: { id: 'w1', name: 'Full Body A', exercises: [{ exerciseId: 'e1', name: 'Squat' }] },
  programId: 'p1',
  exIndex: 1,
  setIndex: 2,
  weight: 82.5,
  reps: 5,
  rpe: 8,
  // Volutamente asimmetrico: 2 esercizi, 3 set. Con un set per esercizio
  // "conta gli esercizi" e "conta i set" darebbero lo stesso numero e un
  // errore di conteggio passerebbe inosservato.
  logged: [
    [
      { w: 80, r: 5, rpe: 7 },
      { w: 80, r: 5, rpe: 8 }
    ],
    [{ w: 82.5, r: 5, rpe: 8 }]
  ],
  prs: [],
  startTime: Date.now() - 600000
});

// --- round trip -------------------------------------------------------------
reset();
M.saveDraft(draft());
const back = M.loadDraft();
check('round trip returns a draft', back !== null);
check('indici preservati', back?.exIndex === 1 && back?.setIndex === 2);
check('peso preservato (float)', back?.weight === 82.5);
check('set registrati preservati', back?.logged?.[0]?.length === 2 && back?.logged?.[1]?.[0]?.rpe === 8);
check('snapshot del workout preservato', back?.workout?.id === 'w1', 'senza questo il ripristino offline è impossibile');
check('startTime preservato', typeof back?.startTime === 'number');

// --- clear ------------------------------------------------------------------
M.clearDraft();
check('clearDraft rimuove il draft', M.loadDraft() === null);

// --- draftInfo --------------------------------------------------------------
reset();
M.saveDraft(draft());
const info = M.draftInfo();
check('draftInfo conta i set totali, non gli esercizi', info?.doneSets === 3, `atteso 3, letto ${info?.doneSets}`);
check('draftInfo riporta il nome', info?.name === 'Full Body A');
reset();
check('draftInfo è null senza draft', M.draftInfo() === null);

// --- scadenza ---------------------------------------------------------------
// Un draft vecchio non va ripreso: produrrebbe una sessione con durata assurda.
reset();
M.saveDraft(draft());
const stale = JSON.parse(store['elite.session.draft']);
stale.savedAt = Date.now() - 25 * 60 * 60 * 1000;
store['elite.session.draft'] = JSON.stringify(stale);
check('draft più vecchio di 24h viene scartato', M.loadDraft() === null);

reset();
M.saveDraft(draft());
const fresh = JSON.parse(store['elite.session.draft']);
fresh.savedAt = Date.now() - 23 * 60 * 60 * 1000;
store['elite.session.draft'] = JSON.stringify(fresh);
check('draft di 23h è ancora valido', M.loadDraft() !== null);

// --- versione schema --------------------------------------------------------
reset();
M.saveDraft(draft());
const old = JSON.parse(store['elite.session.draft']);
old.v = 0;
store['elite.session.draft'] = JSON.stringify(old);
check('draft di uno schema precedente viene scartato', M.loadDraft() === null);

// --- dati corrotti ----------------------------------------------------------
reset();
store['elite.session.draft'] = '{non JSON';
check('JSON illeggibile non lancia', M.loadDraft() === null);

// --- localStorage non disponibile ------------------------------------------
// Modalità privata, quota piena: si perde la durabilità, non si rompe l'app.
reset();
throwOnAccess = true;
check('saveDraft non lancia', M.saveDraft(draft()) === false);
check('loadDraft non lancia', M.loadDraft() === null);
check('draftInfo non lancia', M.draftInfo() === null);
let threw = false;
try {
  M.clearDraft();
} catch {
  threw = true;
}
check('clearDraft non lancia', !threw);

// --- riassunto in attesa ---------------------------------------------------
reset();
const summary = { name: 'Full Body A', volume: 4200, sets: 12, setLog: [{ exerciseId: 'e1', weight: 80 }] };
M.saveSummary(summary);
check('riassunto persistito', M.loadSummary()?.volume === 4200);
check('setLog del riassunto preservato', M.loadSummary()?.setLog?.length === 1, 'senza i set il backend non ricostruisce niente');
M.clearSummary();
check('clearSummary rimuove il riassunto', M.loadSummary() === null);

// Draft e riassunto sono chiavi separate: nel passaggio fra le due schermate
// esistono entrambi per un istante, e cancellare l'uno non deve toccare l'altro.
reset();
M.saveDraft(draft());
M.saveSummary(summary);
M.clearDraft();
check('clearDraft non tocca il riassunto', M.loadSummary() !== null);
M.saveDraft(draft());
M.clearSummary();
check('clearSummary non tocca il draft', M.loadDraft() !== null);

if (failed) {
  console.error(`\nsession-draft: ${failed} controlli falliti`);
  process.exit(1);
}
console.log('session-draft OK — persistenza, scadenza, schema e fallback verificati');
