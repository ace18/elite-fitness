# ELITE — Strength Training (SvelteKit)

A SvelteKit port of the ELITE fitness prototype: a mobile-first strength-training
PWA with onboarding, passwordless auth, a workout dashboard, a focus-mode session
logger with auto-progression and a rest timer, a post-workout receipt, a progress
dashboard, and a plan picker (curated programs + a custom-plan questionnaire).

## Run it

```bash
cd sveltekit-elite
npm install
npm run dev      # http://localhost:5173
```

Build a static SPA bundle:

```bash
npm run build
npm run preview
```

> The app renders inside a fixed 402×874 iPhone frame that auto-scales to your
> viewport — open it on a desktop browser and it letterboxes; resize freely.

## How it's built

- **Svelte 5 + SvelteKit 2** with the runes API (`$state`, `$derived`, `$props`, `$effect`).
- **SPA mode** — `src/routes/+layout.js` sets `ssr = false`. The app uses
  `localStorage` as a mock backend, so everything runs client-side. Build output
  is a static bundle (`@sveltejs/adapter-static`, SPA fallback to `index.html`).
- **Routing = screens.** Each app screen is a route under `src/routes/`. Navigation
  uses `goto()`; the bottom tab bar highlights the active route from `$page`.
- **Shared state** lives in `src/lib/stores.js` (Svelte stores for the workout,
  program, progress, and the in-flight session summary handed to `/receipt`).
- **Mock backend** is `src/lib/api.js`. Every call is `async` and simulates latency,
  so swapping `mock(...)` for `fetch('/api/...')` later requires no UI changes.

## Project structure

```
src/
  app.html              # document shell + Manrope font
  app.css               # design system (tokens, type, buttons, cards, animations)
  lib/
    api.js              # mock backend (localStorage-backed)
    stores.js           # workout / program / progress / summary stores
    plans.js            # curated plans, questionnaire, recommendation engine
    components/
      device/           # Stage (auto-scale), IOSDevice bezel, IOSStatusBar
      ui/               # Btn, Screen, Ring, Sparkline, Stepper, RPEScale,
                        # Delta, Toast, MiniStat, Avatar, Logo, Loading, TabBar
  routes/
    +layout.svelte      # wraps every route in the device frame
    +layout.js          # ssr = false (SPA)
    +page.svelte        # entry → /home or /onboarding
    onboarding/         # value carousel
    login/              # passwordless auth (OAuth + magic link)
    home/               # dashboard
    train/              # active program + next session
    plan/               # choose a plan (browse curated → or build custom)
    session/            # focus-mode set logger + rest timer
    receipt/            # post-workout summary
    progress/           # metrics dashboard
    you/                # profile + settings
```

## Connecting a real backend

Replace the bodies of the methods in `src/lib/api.js` with real `fetch` calls.
The screens consume the stores in `src/lib/stores.js`, so as long as the shapes
stay the same, no screen code needs to change. Drop `ssr = false` from
`+layout.js` (and switch to `@sveltejs/adapter-auto` or a server adapter) once
data no longer depends on `localStorage`.
