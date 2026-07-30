// stores.js — shared app state (Svelte stores): workout / program / progress,
// plus the in-flight session summary handed from the Session screen to the
// Receipt screen.
//
// The three data stores start as `null` (nothing loaded yet) and are filled by
// reloadAll(). Home / Train / Progress already render <Loading> while they're
// null; Session redirects if there's no workout.

import { writable } from 'svelte/store';
import { API, ApiError } from './api.js';
import { saveSummary, loadSummary, clearSummary } from './session-draft.js';

// `undefined` = not loaded yet (show a spinner).
// `null`      = loaded, but there's nothing (rest day / no program yet).
// Screens must distinguish the two or a rest day spins forever.
export const workout = writable(undefined);
export const program = writable(undefined);
export const progress = writable(undefined);

// Last load error, so screens can tell "still loading" from "backend is down".
export const loadError = writable(null);

// Riassunto prodotto da una sessione finita e consumato da /receipt.
//
// Rispecchiato in localStorage perché fra la fine dell'allenamento e il POST
// riuscito è l'unica copia dei dati: senza, un salvataggio fallito (palestra
// senza campo) seguito dalla chiusura dell'app perdeva l'allenamento intero.
// Si legge subito all'avvio, così anche un refresh sulla ricevuta la ritrova.
const summaryStore = writable(loadSummary());
export const summary = {
  subscribe: summaryStore.subscribe,
  set(value) {
    // null = salvato e confermato: si può buttare la copia locale.
    if (value == null) clearSummary();
    else saveSummary(value);
    summaryStore.set(value);
  }
};

export async function reloadAll() {
  try {
    const [w, p, pr] = await Promise.all([
      API.getTodayWorkout(),
      API.getProgram(),
      API.getProgress()
    ]);
    workout.set(w);
    program.set(p);
    progress.set(pr);
    loadError.set(null);
  } catch (e) {
    // 401 already cleared the session; the route guards handle the redirect.
    if (e instanceof ApiError && e.status === 401) return;
    loadError.set(e.message ?? String(e));
  }
}

// Quante scritture aspettano di partire. Guida l'indicatore su Allena: una coda
// invisibile è indistinguibile da un allenamento perso.
export const pendingSync = writable(0);

export async function refreshPendingSync() {
  pendingSync.set(await API.pendingSyncCount());
}

// Svuota la coda e aggiorna il contatore. Va chiamata all'avvio e quando torna
// la rete; è sicuro chiamarla spesso perché il server è idempotente per
// clientSessionId.
export async function syncPending() {
  const res = await API.syncPending();
  pendingSync.set(res.remaining);
  // Se qualcosa è partito, i dati derivati sul server (progressi, settimana)
  // sono cambiati: vale la pena rileggerli.
  if (res.sent > 0) await reloadProgress();
  return res;
}

export async function reloadProgress() {
  try {
    progress.set(await API.getProgress());
    loadError.set(null);
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return;
    loadError.set(e.message ?? String(e));
  }
}

// Re-read program + today's workout from the server after a plan is chosen or
// generated, so the weekly schedule reflects what was actually stored.
export async function applyPlan() {
  const [p, w] = await Promise.all([API.getProgram(), API.getTodayWorkout()]);
  program.set(p);
  workout.set(w);
}
