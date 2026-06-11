// stores.js — shared app state (Svelte stores). Mirrors the prototype's
// top-level App state: workout / program / progress, plus the in-flight
// session summary handed from the Session screen to the Receipt screen.

import { writable } from 'svelte/store';
import { API } from './api.js';

export const workout = writable(API.todaySync());
export const program = writable(API.programSync());
export const progress = writable(API.progressSync());

// summary produced by a finished session and consumed by /receipt
export const DEMO_SUMMARY = {
  name: 'Push Day A',
  durationMin: 48,
  volume: 12840,
  sets: 18,
  exercises: 5,
  topSet: '50 kg × 6',
  avgRpe: 8.1,
  prs: [{ lift: 'Bench Press', value: '50 kg × 6', fresh: true }],
  perExercise: [
    { name: 'Barbell Bench Press',  sets: 4, vol: 4320, last: 42.5, best: 50 },
    { name: 'Incline DB Press',     sets: 3, vol: 2160, last: 18,   best: 18 },
    { name: 'Seated Shoulder Press',sets: 3, vol: 2925, last: 30,   best: 32.5 },
    { name: 'Cable Fly',            sets: 3, vol: 1350, last: 12.5, best: 12.5 },
    { name: 'Triceps Pushdown',     sets: 3, vol: 2085, last: 25,   best: 27.5 }
  ]
};

export const summary = writable(DEMO_SUMMARY);

// refresh helpers (swap mock for real fetch later)
export async function reloadAll() {
  workout.set(await API.getTodayWorkout());
  program.set(await API.getProgram());
  progress.set(await API.getProgress());
}
export async function reloadProgress() {
  progress.set(await API.getProgress());
}

// apply a chosen/built plan as the active program (keeps the weekly schedule)
export function applyPlan(p) {
  program.update((prev) => ({ ...prev, ...p, schedule: prev.schedule }));
}
