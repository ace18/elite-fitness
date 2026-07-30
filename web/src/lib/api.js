// api.js — real HTTP client for the Go backend (repo root, chi + pgx).
//
// In production the Go binary serves this bundle itself, so the API is same
// origin and requests are relative — no hostname is baked into the build.
// In dev, vite runs on its own port and needs the backend's absolute URL.
// VITE_API_URL overrides both; see .env.example.

const BASE = (
  import.meta.env.VITE_API_URL ?? (import.meta.env.DEV ? 'http://localhost:8080' : '')
).replace(/\/+$/, '');

// Plan generation is polled, not awaited in one request — see generatePlan().
const PLAN_POLL_INTERVAL_MS = 2000;
const PLAN_POLL_TIMEOUT_MS = 10 * 60 * 1000;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const TOKEN_KEY = 'elite.token';
const USER_KEY = 'elite.user';

export class ApiError extends Error {
  constructor(status, message, body) {
    super(message);
    this.name = 'ApiError';
    this.status = status; // 0 = network unreachable
    this.body = body;
    // Stable machine code from the backend (see handler/helpers.go). The
    // backend never localises — the client owns the copy for these.
    this.code = body?.code ?? null;
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
// in internal/service/workout.go — keep the two in sync.
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
  async sendMagicLink(email, locale) {
    // The backend has no idea what language the user reads — it needs to be
    // told, because the email goes out before any account exists.
    return request('/api/auth/magic-link', { method: 'POST', body: { email, locale }, auth: false });
  },

  // ---- auth (OAuth) ----------------------------------------------------
  // Which providers the backend actually has credentials for. Buttons are
  // rendered from this so we never show one that would just error out.
  async listProviders() {
    const res = await request('/api/auth/providers', { auth: false });
    return res?.providers ?? [];
  },

  // Not a fetch: the browser has to *navigate* here so the provider can
  // redirect back. The callback returns to /login?token=… which verifyToken
  // below then consumes, exactly like the emailed magic link.
  oauthStartURL(provider) {
    return `${BASE}/api/auth/oauth/${encodeURIComponent(provider)}/start`;
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
  // { goal, level, days, length, notes } -> generated UserProgram.
  //
  // Generating takes minutes, so the backend runs it in the background: the
  // POST answers immediately with a job id and we poll for the result. Holding
  // one long request open instead would die at the Cloudflare tunnel's ~100s
  // origin timeout, with the generation still completing server-side — the user
  // would see a failure and then find a program they never saw appear.
  //
  // Callers still just await this and get the program back.
  async generatePlan(input) {
    const { jobId } = await request('/api/plans/generate', { method: 'POST', body: input });

    const deadline = Date.now() + PLAN_POLL_TIMEOUT_MS;
    while (Date.now() < deadline) {
      await sleep(PLAN_POLL_INTERVAL_MS);
      const job = await request(`/api/plans/generate/${jobId}`);
      if (job.status === 'done') return job.program;
      if (job.status === 'failed') {
        throw new ApiError(500, job.error ?? 'plan generation failed', job);
      }
    }
    // The job may still finish server-side; we just stop waiting on it.
    throw new ApiError(504, 'plan generation timed out', { code: 'plan_generation_timeout' });
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
        // No programId: the server stamps the session with the user's active
        // program itself, so anything sent here would just be ignored.
        //
        // clientSessionId makes the save idempotent — retrying the same
        // finished workout returns the original session instead of logging a
        // second one. It's fixed when the workout ends and persisted with the
        // summary, so every retry sends the same value.
        clientSessionId: summary.clientSessionId ?? null,
        // When the workout actually finished, which can be hours before we
        // manage to send it. The server clamps it (nothing in the future) but
        // otherwise stores it as given — the program's week progression and the
        // final-week count are both derived from it.
        completedAt: summary.completedAt ?? null,
        name: summary.name,
        durationMin: summary.durationMin ?? null,
        totalVolume: summary.volume ?? null,
        totalSets: summary.sets ?? null,
        avgRpe: summary.avgRpe ?? null,
        sessionRpe: summary.sessionRpe ?? null,
        // is_pr is computed server-side
        sets: summary.setLog ?? []
      }
    });
  }
};
