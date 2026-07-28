// api.js — real HTTP client for the Go backend (backend/, chi + pgx).
// The base URL comes from VITE_API_URL; see .env.example.

const BASE = (import.meta.env.VITE_API_URL ?? 'http://localhost:8080').replace(/\/+$/, '');

const TOKEN_KEY = 'elite.token';
const USER_KEY = 'elite.user';

export class ApiError extends Error {
  constructor(status, message, body) {
    super(message);
    this.name = 'ApiError';
    this.status = status; // 0 = network unreachable
    this.body = body;
  }
}

// localStorage can throw in private mode; the app runs SPA-only (ssr = false)
// but stay defensive anyway.
function ls(fn, fallback = null) {
  try {
    return fn();
  } catch {
    return fallback;
  }
}

const getToken = () => ls(() => localStorage.getItem(TOKEN_KEY));
const setToken = (t) => ls(() => localStorage.setItem(TOKEN_KEY, t));

function setUser(u) {
  ls(() => localStorage.setItem(USER_KEY, JSON.stringify(u)));
}
function clearSession() {
  ls(() => {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  });
}

function parse(text) {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

async function request(path, { method = 'GET', body, auth = true } = {}) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  const token = getToken();
  if (auth && token) headers.Authorization = `Bearer ${token}`;

  let res;
  try {
    res = await fetch(`${BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body)
    });
  } catch (e) {
    throw new ApiError(0, `cannot reach the API at ${BASE}`, null);
  }

  // The JWT is gone or expired — drop it so the guards send the user to login.
  if (res.status === 401 && auth) {
    clearSession();
    throw new ApiError(401, 'session expired', null);
  }

  const text = await res.text();
  const data = text ? parse(text) : null;
  if (!res.ok) {
    throw new ApiError(res.status, data?.error ?? res.statusText, data);
  }
  return data;
}

// GET that treats 404 as "nothing yet" rather than an error — the backend
// returns 404 for a user with no active program, which is a normal state.
async function getOrNull(path) {
  try {
    return await request(path);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

// 1RM estimate (Epley) — pure, stays client-side
export function est1RM(w, reps) {
  return Math.round(w * (1 + reps / 30));
}

// progressive-overload suggestion from last set's RPE. Mirrors suggestNext()
// in backend/internal/service/workout.go — keep the two in sync.
export function suggestNext(weight, rpe, step = 2.5) {
  if (rpe == null) return weight;
  if (rpe <= 7) return +(weight + step).toFixed(1);
  if (rpe >= 9.5) return +(weight - step).toFixed(1);
  return weight;
}

export const API = {
  est1RM,
  suggestNext,

  // ---- auth (passwordless magic link) ----------------------------------
  // Returns { ok, email, devToken? }. devToken is only present when the
  // backend runs with APP_ENV=development.
  async sendMagicLink(email) {
    return request('/api/auth/magic-link', { method: 'POST', body: { email }, auth: false });
  },

  // Exchanges the emailed token for a JWT and caches the session.
  async verifyToken(token) {
    const res = await request(`/api/auth/verify?token=${encodeURIComponent(token)}`, { auth: false });
    setToken(res.token);
    setUser(res.user);
    return res.user;
  },

  isAuthed() {
    return !!getToken();
  },

  // Synchronous cached read, so route guards stay sync. refreshUser() keeps
  // it current.
  user() {
    const raw = ls(() => localStorage.getItem(USER_KEY));
    return (raw && parse(raw)) || { name: '', email: '' };
  },

  async refreshUser() {
    const u = await request('/api/auth/me');
    setUser(u);
    return u;
  },

  logout() {
    clearSession();
  },

  // ---- training --------------------------------------------------------
  getTodayWorkout() {
    return getOrNull('/api/workout/today');
  },
  getProgram() {
    return getOrNull('/api/program');
  },
  getProgress() {
    return request('/api/progress');
  },
  getLastSession() {
    return getOrNull('/api/sessions/last');
  },

  // ---- plans -----------------------------------------------------------
  getPlans() {
    return request('/api/plans');
  },
  setProgram(templateId) {
    return request('/api/program', { method: 'POST', body: { templateId } });
  },
  // { goal, level, days, length, notes } -> generated UserProgram (slow: AI call)
  generatePlan(input) {
    return request('/api/plans/generate', { method: 'POST', body: input });
  },

  logWeight(weight, unit = 'kg') {
    return request('/api/progress/weight', { method: 'POST', body: { weight, unit } });
  },

  // ---- sessions --------------------------------------------------------
  // Maps the UI summary onto the backend's SessionLog. `setLog` carries the
  // per-set rows: without them set_logs stays empty and nothing downstream
  // works (no PRs, no est-1RM, no "last time" suggestions).
  async saveSession(summary) {
    return request('/api/sessions', {
      method: 'POST',
      body: {
        workoutId: summary.workoutId ?? null,
        programId: summary.programId ?? null,
        name: summary.name,
        durationMin: summary.durationMin ?? null,
        totalVolume: summary.volume ?? null,
        totalSets: summary.sets ?? null,
        avgRpe: summary.avgRpe ?? null,
        sessionRpe: summary.sessionRpe ?? null,
        // completed_at defaults to NOW() server-side; is_pr is computed there too
        sets: summary.setLog ?? []
      }
    });
  }
};
