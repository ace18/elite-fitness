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
docker-compose.yml   # local stack: app + persistent Postgres
```

## Run it

Fastest path — the whole stack in Docker, exactly as it ships:

```bash
docker compose up -d --build      # → http://localhost:8080
docker compose logs -f app
docker compose down               # stops; data stays in the pgdata volume
```

Postgres data persists in a named volume; `docker compose down -v` wipes it.
`RESEND_API_KEY` is deliberately not passed, so the magic-link endpoint returns
a `devToken` and you can log in without sending mail — see the comments in
`docker-compose.yml`.

### Without Docker

For frontend work you want vite's hot reload, which means two processes and a
cross-origin API. Any local Postgres will do — point `DATABASE_URL` at it and
create an empty database; migrations build the schema on first boot.

```bash
# once: create an empty database (adjust host/port/user for your Postgres)
createdb -h 127.0.0.1 -p 5433 -U postgres elite

# terminal 1 — API
RESEND_API_KEY= \
DATABASE_URL="postgres://postgres@127.0.0.1:5433/elite?sslmode=disable" \
APP_ENV=development PORT=8080 FRONTEND_URL=http://localhost:5173 \
go run ./cmd/server

# terminal 2 — frontend
cd web && pnpm install && pnpm dev      # → http://localhost:5173
```

`createdb` needs the Postgres client tools on `PATH`, which GUI managers like
DBngin and Postgres.app don't add — their binaries live inside the app's own
directory (DBngin: `/Users/Shared/DBngin/postgresql/<version>/bin/`). Use the
full path, or just create the database from the manager's UI.

**`RESEND_API_KEY=` — with an empty value — is load-bearing, and `env -u` is
not a substitute.** `godotenv` loads the repo-root `.env` from the working
directory, so a real Resend key gets picked up; the magic-link endpoint then
tries to send mail for real, the provider rejects `@example.com` addresses, and
the request fails with 500 and no `devToken` — leaving no way to log in.
Setting the variable to empty leaves the key *present* in the environment, which
is what makes `godotenv` skip it: `Load` only skips keys already set, so
*unsetting* the variable is precisely what invites the value in from `.env`.

### Single origin (PWA, service worker, offline)

The two-process setup can't exercise the service worker: it only caches
same-origin requests, and there the API is on another port. To test installing
the app, offline mode, or the sync queue, serve the built SPA from the API — the
production topology, minus hot reload.

```bash
cd web && pnpm build && cd ..
RESEND_API_KEY= STATIC_DIR=web/build \
DATABASE_URL="postgres://postgres@127.0.0.1:5433/elite?sslmode=disable" \
APP_ENV=development PORT=8080 \
FRONTEND_URL=http://localhost:8080 API_URL=http://localhost:8080 \
go run ./cmd/server                     # → http://localhost:8080
```

> `go run` leaves the compiled binary running after you kill the parent, and it
> keeps holding `:8080` — which makes the *next* start look like it worked when
> it never bound. If behaviour looks stale: `kill -9 $(lsof -ti :8080)`.

### Tests

```bash
go test ./...            # backend; repository tests skip without a database
cd web && pnpm test      # session-draft persistence + i18n parity
cd web && pnpm build     # SPA bundle

# repository integration tests, against any scratch database
TEST_DATABASE_URL="postgres://postgres@127.0.0.1:5433/elite?sslmode=disable" \
  go test ./internal/repository/
```

> Mobile-first: the app fills the viewport on a phone and sits in a centred
> 520px column on wider screens. Safe-area insets are respected, so an installed
> PWA clears the notch and the home indicator.

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
