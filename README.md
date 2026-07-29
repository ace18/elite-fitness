# ELITE — Strength Training

A mobile-first strength-training PWA: onboarding, passwordless auth (magic link +
OAuth), a workout dashboard, a focus-mode session logger with auto-progression and
a rest timer, a post-workout receipt, a progress dashboard, and a plan picker
(curated programs + an AI-generated custom plan).

Two halves in one repo, shipped as **one container**: a Go API at the root and a
SvelteKit SPA in `web/`. In production the Go binary serves the built SPA itself,
so there is a single origin — no CORS, and the OAuth `redirect_uri` matches the
public domain.

## Layout

```
cmd/server/          # entrypoint
internal/
  config/            # env → Config
  db/                # pool + migrations (applied on boot)
  handler/           # HTTP handlers, SPA static serving
  middleware/        # JWT auth, Cloudflare client-IP
  model/             # domain types
  repository/        # SQL
  service/           # auth, email, oauth, workout, AI plans, pruning
web/                 # SvelteKit SPA (its own package.json)
  src/lib/api.js     # HTTP client for the API above
  src/routes/        # one route per screen
Dockerfile           # multi-stage: builds both halves into one image
```

## Run it

Two processes in development — vite serves the frontend on its own port and talks
to the API cross-origin.

```bash
# 1. Postgres (throwaway)
docker run -d --rm --name elite-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=elite \
  -p 55432:5432 postgres:16-alpine

# 2. API — copy .env.example to .env first
DATABASE_URL="postgres://postgres:test@localhost:55432/elite?sslmode=disable" \
APP_ENV=development FRONTEND_URL=http://localhost:5173 go run ./cmd/server

# 3. Frontend
cd web && pnpm install && pnpm dev      # http://localhost:5173
```

Migrations run automatically on boot. In development the magic-link endpoint
returns a `devToken` when `RESEND_API_KEY` is unset, so you can log in without
sending mail.

```bash
go test ./...            # backend
cd web && pnpm build     # SPA bundle
cd web && pnpm check:i18n
```

> The app renders inside a fixed 402×874 iPhone frame that auto-scales to your
> viewport — open it on a desktop browser and it letterboxes; resize freely.

## Production

```bash
docker build -t elite .
```

The image builds both halves and sets `STATIC_DIR=/app/static`, which is what
makes the binary serve the SPA. Required environment: `DATABASE_URL`,
`JWT_SECRET`, `RESEND_API_KEY`, `ANTHROPIC_API_KEY`, `APP_ENV=production`, and
`API_URL`/`FRONTEND_URL` both set to the public hostname. Behind Cloudflare also
set `TRUST_PROXY_HEADERS=true` — see `.env.example` for why each one matters.

Run **one replica**: migrations apply on boot and the plan-generation job store is
in-process.

## How it's built

- **Svelte 5 + SvelteKit 2** with the runes API (`$state`, `$derived`, `$props`, `$effect`).
- **SPA mode** — `web/src/routes/+layout.js` sets `ssr = false`; the build is a
  static bundle (`@sveltejs/adapter-static`, fallback to `index.html`). All data
  comes from the Go API.
- **Routing = screens.** Each screen is a route under `web/src/routes/`.
- **Shared state** lives in `web/src/lib/stores.js`.
- **Go API** — chi + pgx, JWT auth. Errors are returned as stable machine codes
  (`handler/helpers.go`); the client owns the translations, since the backend
  doesn't know the user's language.
- **AI plan generation is asynchronous.** `POST /api/plans/generate` returns
  `202 {jobId}` and the client polls, because generation outlives Cloudflare's
  ~100s origin timeout. `API.generatePlan()` hides the polling from callers.
