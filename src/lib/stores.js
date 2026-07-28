// stores.js — shared app state (Svelte stores): workout / program / progress,
// plus the in-flight session summary handed from the Session screen to the
// Receipt screen.
//
// The three data stores start as `null` (nothing loaded yet) and are filled by
// reloadAll(). Home / Train / Progress already render <Loading> while they're
// null; Session redirects if there's no workout.

import { writable } from 'svelte/store';
import { API, ApiError } from './api.js';

// `undefined` = not loaded yet (show a spinner).
// `null`      = loaded, but there's nothing (rest day / no program yet).
// Screens must distinguish the two or a rest day spins forever.
export const workout = writable(undefined);
export const program = writable(undefined);
export const progress = writable(undefined);

// Last load error, so screens can tell "still loading" from "backend is down".
export const loadError = writable(null);

// summary produced by a finished session and consumed by /receipt
export const summary = writable(null);

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
