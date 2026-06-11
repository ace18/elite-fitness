// api.js — mock backend. Every function mimics an async API call so the real
// app can swap `mock(...)` for `fetch('/api/...')` with no UI changes.

const LS = 'elite.v1';

function loadDB() {
  try {
    return JSON.parse(localStorage.getItem(LS)) || {};
  } catch (e) {
    return {};
  }
}
function saveDB(p) {
  try {
    localStorage.setItem(LS, JSON.stringify(p));
  } catch (e) {
    /* ignore (SSR / private mode) */
  }
}

// simulate network latency
function mock(value, ms = 420) {
  return new Promise((r) => setTimeout(() => r(typeof value === 'function' ? value() : value), ms));
}

// 1RM estimate (Epley)
export function est1RM(w, reps) {
  return Math.round(w * (1 + reps / 30));
}

// progressive-overload suggestion from last set's RPE
export function suggestNext(weight, rpe, step = 2.5) {
  if (rpe == null) return weight;
  if (rpe <= 7) return +(weight + step).toFixed(1);
  if (rpe >= 9.5) return +(weight - step).toFixed(1);
  return weight;
}

// ---- today's programmed workout ----
const TODAY_WORKOUT = {
  id: 'push-a',
  name: 'Push Day A',
  focus: 'Chest · Shoulders · Triceps',
  estMin: 48,
  exercises: [
    { id: 'bench',   name: 'Barbell Bench Press',  muscle: 'Chest',       sets: 4, targetReps: 8,  last: 42.5, suggested: 45,   rest: 120, prToBeat: 48 },
    { id: 'incline', name: 'Incline DB Press',      muscle: 'Upper chest', sets: 3, targetReps: 10, last: 18,   suggested: 18,   rest: 90 },
    { id: 'ohp',     name: 'Seated Shoulder Press', muscle: 'Shoulders',   sets: 3, targetReps: 10, last: 30,   suggested: 32.5, rest: 90,  prToBeat: 34 },
    { id: 'fly',     name: 'Cable Fly',             muscle: 'Chest',       sets: 3, targetReps: 12, last: 12.5, suggested: 12.5, rest: 60 },
    { id: 'tri',     name: 'Triceps Pushdown',      muscle: 'Triceps',     sets: 3, targetReps: 12, last: 25,   suggested: 27.5, rest: 60 }
  ]
};

// ---- active training program (the plan the user is following) ----
const PROGRAM = {
  id: 'ppl-hyp',
  name: 'Push / Pull / Legs',
  goal: 'Hypertrophy',
  level: 'Intermediate',
  week: 3,
  totalWeeks: 6,
  daysPerWeek: 5,
  nextSessionId: 'push-a',
  // this week at a glance — status: done | today | upcoming | rest
  schedule: [
    { day: 'Mon', name: 'Push Day A',  focus: 'Chest · Shoulders · Tri', status: 'done',     volume: '12.8k' },
    { day: 'Tue', name: 'Pull Day B',  focus: 'Back · Biceps',           status: 'done',     volume: '14.1k' },
    { day: 'Wed', name: 'Leg Day',     focus: 'Quads · Hamstrings',      status: 'done',     volume: '19.4k' },
    { day: 'Thu', name: 'Rest',        focus: 'Active recovery',         status: 'rest' },
    { day: 'Fri', name: 'Upper Power', focus: 'Strength focus',          status: 'done',     volume: '11.2k' },
    { day: 'Sat', name: 'Push Day A',  focus: 'Chest · Shoulders · Tri', status: 'today' },
    { day: 'Sun', name: 'Rest',        focus: 'Active recovery',         status: 'rest' }
  ]
};

// ---- progress / history (for the dashboard) ----
const PROGRESS = {
  streak: 12,
  weekSessions: 4,
  weekGoal: 5,
  bodyWeight: { value: 78.4, unit: 'kg', delta: -0.6, series: [80.1, 79.8, 79.6, 79.2, 79.0, 78.7, 78.4] },
  est1RM: { lift: 'Bench', value: 62, unit: 'kg', delta: 4, series: [55, 56, 56, 58, 59, 60, 61, 62] },
  volume: { value: 48.2, unit: 'k kg', delta: +6, series: [31, 38, 34, 41, 39, 44, 42, 48] },
  prs: [
    { lift: 'Bench Press', value: '50 kg × 6',  when: 'Today',     fresh: true },
    { lift: 'Back Squat',  value: '90 kg × 5',  when: '2d ago',    fresh: true },
    { lift: 'Deadlift',    value: '120 kg × 3', when: 'Last week' }
  ]
};

const clone = (o) => JSON.parse(JSON.stringify(o));

export const API = {
  est1RM,
  suggestNext,

  // auth — passwordless
  async sendMagicLink(email) {
    return mock({ ok: true, email }, 700);
  },
  async verifyLink(email) {
    const db = loadDB();
    db.user = { email: email || 'alex@elite.fit', name: 'Alex' };
    db.authed = true;
    saveDB(db);
    return mock({ ok: true, user: db.user }, 600);
  },
  async oauth(provider) {
    const db = loadDB();
    db.user = { email: `alex@${provider}.com`, name: 'Alex' };
    db.authed = true;
    saveDB(db);
    return mock({ ok: true, user: db.user }, 800);
  },
  isAuthed() {
    return !!loadDB().authed;
  },
  user() {
    return loadDB().user || { name: 'Alex' };
  },
  logout() {
    const db = loadDB();
    db.authed = false;
    saveDB(db);
  },

  async getTodayWorkout() {
    return mock(clone(TODAY_WORKOUT));
  },
  todaySync() {
    return clone(TODAY_WORKOUT);
  },
  async getProgram() {
    return mock(clone(PROGRAM));
  },
  programSync() {
    return clone(PROGRAM);
  },
  progressSync() {
    return clone(PROGRESS);
  },
  async getProgress() {
    const db = loadDB();
    const p = clone(PROGRESS);
    if (db.lastSession) {
      p.streak = db.streakBump ? PROGRESS.streak + 1 : PROGRESS.streak;
    }
    return mock(p, 380);
  },

  // persist a finished session (so streak/home reflect it)
  async saveSession(summary) {
    const db = loadDB();
    db.lastSession = summary;
    db.streakBump = true;
    saveDB(db);
    return mock({ ok: true }, 500);
  },
  lastSession() {
    return loadDB().lastSession || null;
  }
};
