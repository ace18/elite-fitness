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

Postgres is pinned to **18**, the same major as the deployed database. If you
have a volume from an older major the container refuses to start (`database
files are incompatible with server`) — PGDATA can't be read across majors.
`docker compose down -v` and let the migrations rebuild; there's nothing in a
local volume that isn't in `internal/db/migrations/`.

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

### Deploying on Coolify

**The database.** Add a PostgreSQL resource and pin the image to
`postgres:18-alpine` rather than leaving the tag floating — a managed database
that jumps a major on redeploy comes back as a crash loop. Copy its *internal*
connection URL; the public one is for `psql` from your laptop, and that port is
better left closed. Enable scheduled backups before there are real users in
there: Coolify does not take any on its own.

**The application.** Deploy the **published image**, not the repository:
`.github/workflows/docker-publish.yml` already builds it on GitHub and pushes to
`ghcr.io/ace18/elite-fitness`. Pointing Coolify at the Dockerfile instead would
move a `pnpm install` + `vite build` + Go compile onto the machine that is also
serving the app, at every deploy, and give up the `sha-<commit>` tags that make
rollback a matter of changing a string.

So: resource type **Docker Image**, `ghcr.io/ace18/elite-fitness:master`, port
**8080**, health check path `/healthz`. Keep it in the same project and
environment as the database so the internal hostname resolves — if the boot log
shows `no such host`, turn on *Advanced → Connect To Predefined Network*.

Rolling back is repointing the tag: `sha-<commit>` for any build on `master`,
or one of the `v*.*.*` tags the workflow publishes from version tags.

**The environment.** Everything from the list above, plus `sslmode=disable` on
`DATABASE_URL` — the Postgres container serves no TLS, and the traffic never
leaves the Docker network. `API_URL` and `FRONTEND_URL` are both the public
hostname. A first deploy against an empty database logs
`migrations: applicata 001_init.sql` through the last file in
`internal/db/migrations/`; `nessuna nuova migration` instead means you're
pointed at a database that already has the schema.

#### What will bite you

- **Deploying the image means nothing watches for pushes.** The build pack's
  git trigger is what you gave up; CI has to say when a new image is ready, so
  add Coolify's deploy webhook as a final step in `docker-publish.yml`.
- **`master` is a mutable tag.** Without *force pull* Coolify re-runs the
  layers it already has and the deploy is a silent no-op — same code, green
  checkmark.
- **A private repo means a private GHCR package.** The server needs a PAT with
  `read:packages`, registered in Coolify's registry settings or applied once on
  the host with `docker login ghcr.io`.
- **The published image is `linux/amd64` only.** On an ARM server it won't run;
  that needs `platforms: linux/arm64` plus QEMU in the workflow, and a much
  slower build.
- **One replica, still.** The advisory lock in `internal/db/db.go` makes
  concurrent migrations safe, but the plan-generation job store doesn't leave
  the process: with two instances a client gets `202 {jobId}` from one and
  polls the other, which has never heard of that job.
- **`APP_ENV=production` makes `JWT_SECRET` and `RESEND_API_KEY` fatal if
  missing.** The process exits during boot (`cmd/server/main.go`), which from
  the outside is indistinguishable from a container that starts and dies.
- **Don't set `PORT`.** The image fixes it at 8080 and that's what the proxy
  targets; overriding it produces a healthy container the proxy can't reach.
- **`TRUST_PROXY_HEADERS` only pays off behind Cloudflare.** Traefik terminates
  every connection, so the magic-link rate limiter sees one client IP and users
  lock each other out. The flag doesn't fix that by itself: the middleware
  trusts `CF-Connecting-IP` and nothing else, because it's the only forwarding
  header a client can't forge (`internal/middleware/realip.go`). Either put
  Cloudflare in front as a proxied record and set it to `true`, or leave it
  `false` and accept a shared bucket.
- **The OAuth `redirect_uri` must match `API_URL` character for character** —
  `https://<host>/api/auth/oauth/google/callback`, registered once the domain
  is live. Apple additionally needs all four `APPLE_*` variables, and refuses
  `http://` and `localhost`, so this is the first place it can be tested.
- **The image is fixed to `TZ=Europe/Rome`.** "Which workout is today" comes
  from `time.Now()`, so users in other timezones see the wrong day's session
  between midnight and the offset. Fine while the audience is Italian; a
  per-user timezone is the real fix.
- **Postgres majors don't upgrade in place.** Moving off 18 later means a
  `pg_dump` and restore, not an image tag change.

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
