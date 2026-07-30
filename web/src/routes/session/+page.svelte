<script>
  // Session — active workout, FOCUS MODE. One set at a time · steppers ·
  // auto-suggested next weight · per-set RPE · rest timer. Finish → /receipt.
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { workout, program, summary } from '$lib/stores.js';
  import { saveDraft, loadDraft, clearDraft, newClientSessionId } from '$lib/session-draft.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';
  import Ring from '$lib/components/ui/Ring.svelte';
  import Stepper from '$lib/components/ui/Stepper.svelte';
  import { t } from '$lib/i18n/index.js';
  import RPEScale, { rpeColor } from '$lib/components/ui/RPEScale.svelte';
  import Toast from '$lib/components/ui/Toast.svelte';

  // Un allenamento sospeso (refresh, app chiusa, iOS che scarica la PWA dalla
  // memoria) viene ripreso da qui. Il draft porta il suo snapshot del workout,
  // così il ripristino funziona anche quando gli store sono vuoti e la rete non
  // c'è — che è il caso in cui serve davvero.
  const draft = loadDraft();

  // Snapshot at mount. Null when the store hasn't loaded (or the user has no
  // program) — onMount redirects and the template stays unrendered.
  const w = draft?.workout ?? $workout;
  const prog = $program;
  const exs = w?.exercises ?? [];
  const totalSets = exs.reduce((a, e) => a + e.sets, 0);

  // Il draft vale solo per lo stesso allenamento: se il programma è cambiato o
  // è un altro giorno, riprendere i set dentro una scheda diversa produrrebbe
  // dati attribuiti all'esercizio sbagliato.
  const resuming = !!draft && draft.workout?.id === w?.id && exs.length > 0;

  let exIndex = $state(resuming ? (draft.exIndex ?? 0) : 0);
  let setIndex = $state(resuming ? (draft.setIndex ?? 0) : 0);
  let ex = $derived(exs[exIndex]);

  let weight = $state(resuming ? draft.weight : (exs[0]?.suggested ?? 0));
  let reps = $state(resuming ? draft.reps : (exs[0]?.targetReps ?? 8));
  let rpe = $state(resuming ? (draft.rpe ?? null) : null);
  let logged = $state(resuming ? draft.logged : exs.map(() => []));
  let prs = $state(resuming ? (draft.prs ?? []) : []);

  let resting = $state(false);
  let rest = $state(0);
  let restTotal = $state(0);
  let toast = $state(null);
  let toastT;

  // Ripreso dal draft, così la durata dell'allenamento resta quella vera anche
  // se l'app è stata chiusa in mezzo.
  let startTime = resuming ? (draft.startTime ?? Date.now()) : Date.now();
  let elapsed = $state(0);
  let elapsedT, restT;

  // Lo stato completo del logger, nella forma in cui viene persistito.
  function snapshot() {
    return { workout: w, programId: prog?.id ?? null, exIndex, setIndex, weight, reps, rpe, logged, prs, startTime };
  }

  // Si scrive quando un set è registrato — il dato che costa fatica — e quando
  // la pagina passa in background, che è il momento prima che iOS la scarichi:
  // così anche peso e ripetizioni digitati e non ancora registrati sopravvivono.
  // Non a ogni tocco dello stepper: sono write sincrone, e ribattere un valore
  // costa un tap.
  function persist() {
    if (exs.length) saveDraft(snapshot());
  }
  function onHide() {
    if (document.visibilityState === 'hidden') persist();
  }

  onMount(() => {
    if (!API.isAuthed()) {
      goto('/onboarding', { replaceState: true });
      return;
    }
    // Deep link / refresh straight onto /session: nothing is loaded yet.
    if (!w || exs.length === 0) {
      goto('/home', { replaceState: true });
      return;
    }
    // Un draft che non combacia col workout di oggi non è recuperabile: meglio
    // toglierlo di mezzo che lasciare "riprendi" a puntare nel vuoto.
    if (draft && !resuming) clearDraft();
    if (resuming) {
      persist(); // rinfresca savedAt: la sessione è di nuovo attiva
      flashToast($t('session.resumed'));
    }
    elapsedT = setInterval(() => (elapsed = Math.floor((Date.now() - startTime) / 1000)), 1000);
    addEventListener('visibilitychange', onHide);
    addEventListener('pagehide', persist);
  });
  onDestroy(() => {
    clearInterval(elapsedT);
    clearTimeout(restT);
    clearTimeout(toastT);
    removeEventListener('visibilitychange', onHide);
    removeEventListener('pagehide', persist);
  });

  // rest countdown
  $effect(() => {
    if (!resting) return;
    if (rest <= 0) {
      resting = false;
      return;
    }
    restT = setTimeout(() => (rest -= 1), 1000);
    return () => clearTimeout(restT);
  });

  let doneSets = $derived(logged.reduce((a, l) => a + l.length, 0));
  let showSuggested = $derived(weight !== ex?.suggested);
  let isLastSet = $derived(setIndex + 1 >= (ex?.sets ?? 0));
  let isLastEx = $derived(exIndex + 1 >= exs.length);
  let finishing = $derived(isLastSet && isLastEx);

  function fmtClock(s) {
    const m = Math.floor(s / 60), ss = s % 60;
    return `${m}:${ss < 10 ? '0' : ''}${ss}`;
  }
  function flashToast(msg) {
    toast = msg;
    clearTimeout(toastT);
    toastT = setTimeout(() => (toast = null), 2200);
  }
  function startRest(secs) {
    restTotal = secs;
    rest = secs;
    resting = true;
  }
  function advanceTo(nextEx, nextSet) {
    const e = exs[nextEx];
    const sameEx = nextEx === exIndex;
    exIndex = nextEx;
    setIndex = nextSet;
    weight = sameEx ? API.suggestNext(weight, rpe) : e.suggested;
    reps = e.targetReps;
    rpe = null;
  }

  function logSet() {
    const rec = { w: weight, r: reps, rpe };
    const nl = logged.map((a) => a.slice());
    nl[exIndex] = [...nl[exIndex], rec];
    logged = nl;

    // PR check
    if (ex.prToBeat && weight >= ex.prToBeat) {
      prs = [...prs, { lift: ex.name, value: `${weight} kg × ${reps}`, fresh: true }];
      flashToast(`🏆 New PR — ${ex.name}!`);
    }

    if (finishing) {
      const flat = nl.flat();
      const volume = flat.reduce((a, s) => a + s.w * s.r, 0);
      const top = flat.reduce((a, s) => (s.w > a.w ? s : a), flat[0]);
      const rpes = flat.map((s) => s.rpe).filter((x) => x != null);
      // Per-set rows for the backend (set_logs). exerciseId comes from the
      // workout the API returned; is_pr is recomputed server-side.
      const setLog = [];
      exs.forEach((e, i) => {
        nl[i].forEach((rec, j) => {
          setLog.push({
            exerciseId: e.exerciseId,
            exerciseName: e.name,
            setNumber: j + 1,
            weight: rec.w,
            reps: rec.r,
            rpe: rec.rpe
          });
        });
      });

      const s = {
        workoutId: w.id,
        programId: prog?.id ?? null,
        // Identità e istante di fine si fissano QUI, non al momento dell'invio:
        // vengono persistiti col riassunto, così un ritentativo mezz'ora dopo
        // manda la stessa identità (niente doppioni) e la stessa ora (la
        // sessione resta datata a quando è stata fatta).
        clientSessionId: newClientSessionId(),
        completedAt: new Date().toISOString(),
        setLog,
        name: w.name,
        durationMin: Math.max(1, Math.round((Date.now() - startTime) / 60000)),
        volume: Math.round(volume),
        sets: flat.length,
        exercises: exs.length,
        topSet: `${top.w} kg × ${top.r}`,
        avgRpe: rpes.length ? +(rpes.reduce((a, b) => a + b, 0) / rpes.length).toFixed(1) : null,
        prs: [...prs, ...(ex.prToBeat && weight >= ex.prToBeat ? [{ lift: ex.name, value: `${weight} kg × ${reps}`, fresh: true }] : [])],
        perExercise: exs.map((e, i) => ({
          name: e.name,
          sets: nl[i].length,
          vol: nl[i].reduce((a, s) => a + s.w * s.r, 0),
          last: e.last,
          best: Math.max(...nl[i].map((s) => s.w), 0)
        }))
      };
      // Il riassunto prende il posto del draft: da qui in poi è lui l'unica
      // copia dei dati, e viene persistito da summary.set() (vedi stores.js)
      // finché il server non conferma il salvataggio.
      summary.set(s);
      clearDraft();
      goto('/receipt');
      return;
    }

    // next set or next exercise + rest
    if (!isLastSet) {
      startRest(ex.rest);
      advanceTo(exIndex, setIndex + 1);
    } else {
      startRest(ex.rest);
      advanceTo(exIndex + 1, 0);
    }
    // Dopo advanceTo, così indici e peso suggerito salvati sono già quelli del
    // set successivo: riprendendo si riparte da dove si era arrivati.
    persist();
  }

  let restPct = $derived(restTotal ? rest / restTotal : 0);
</script>

{#if w && exs.length}
<div style="position:relative; height:100%;">
  <Screen bg="#F4F6F8">
    {#snippet footer()}
      <Btn block lg variant={finishing ? 'dark' : 'primary'} onclick={logSet} disabled={rpe == null}>
        {rpe == null ? $t('session.rateToLog') : finishing ? $t('session.finishWorkout') : $t('session.logSet')}
      </Btn>
    {/snippet}

    <div style="padding-bottom:8px;">
      <!-- header -->
      <div style="padding:4px 18px 0;">
        <div class="row" style="justify-content:space-between;">
          <button onclick={() => goto('/home')} style="width:38px; height:38px; border-radius:12px; border:none; background:#fff; box-shadow:var(--sh-sm); cursor:pointer; font-size:18px; color:var(--ink2); flex:0 0 auto;">✕</button>
          <div style="flex:1; min-width:0; text-align:center;">
            <div class="t-h2" style="font-size:15px; white-space:nowrap;">{w.name}</div>
            <div class="t-mono t-sub" style="font-size:12.5px; font-weight:700;">⏱ {fmtClock(elapsed)}</div>
          </div>
          <div style="width:38px; height:38px; border-radius:12px; background:#fff; box-shadow:var(--sh-sm); display:flex; align-items:center; justify-content:center; flex:0 0 auto;">
            <span class="t-mono" style="font-size:12.5px; font-weight:800; color:var(--brand);">{doneSets}/{totalSets}</span>
          </div>
        </div>
        <div style="height:6px; background:#E4E8EB; border-radius:6px; margin-top:14px; overflow:hidden;">
          <div style="height:100%; width:{(doneSets / totalSets) * 100}%; background:var(--brand); border-radius:6px; transition:width .4s;"></div>
        </div>
      </div>

      <!-- exercise heading -->
      {#key ex.id + setIndex}
        <div class="scr-in" style="padding:18px 20px 0;">
          <div class="row" style="justify-content:space-between; align-items:flex-start;">
            <div>
              <div class="t-label" style="color:var(--brand);">{$t('session.exerciseOf', { i: exIndex + 1, n: exs.length, muscle: ex.muscle })}</div>
              <h1 class="t-title" style="font-size:25px; margin:4px 0 0;">{ex.name}</h1>
            </div>
            <div style="display:flex; gap:5px; margin-top:6px;">
              {#each Array(ex.sets) as _, i}
                <span style="width:9px; height:9px; border-radius:9px;
                  background:{i <= setIndex ? 'var(--brand)' : 'var(--line)'};
                  box-shadow:{i === setIndex ? '0 0 0 3px var(--brand-tint)' : 'none'};"></span>
              {/each}
            </div>
          </div>
          <div class="t-sub" style="font-size:13.5px; margin-top:8px;">{$t('session.setOf', { i: setIndex + 1, n: ex.sets })} <b style="color:var(--ink2);">{ex.last} kg × {ex.targetReps}</b></div>

          {#if logged[exIndex].length > 0}
            <div class="row" style="gap:7px; margin-top:12px; flex-wrap:wrap;">
              {#each logged[exIndex] as s}
                <span class="chip" style="background:var(--up-tint); color:#0a7a44;">✓ {s.w}×{s.r} · {s.rpe}</span>
              {/each}
            </div>
          {/if}

          <!-- input card -->
          <div class="card" style="padding:18px; margin-top:16px; box-shadow:var(--sh-md);">
            <div class="t-label" style="margin-bottom:6px;">{$t('session.weight')}</div>
            <Stepper value={weight} onChange={(v) => (weight = v)} step={2.5} unit="kg" big />
            <div style="text-align:center; margin-top:8px;">
              {#if showSuggested}
                <button class="chip chip--tint" onclick={() => (weight = ex.suggested)}>{$t('session.backToSuggested', { n: ex.suggested })}</button>
              {:else}
                <span class="chip" style="background:transparent; color:var(--ink3); cursor:default;">{$t('session.onPlan', { n: ex.suggested })}</span>
              {/if}
            </div>
            <div class="divider" style="margin:16px 0;"></div>
            <div class="t-label" style="margin-bottom:6px;">{$t('session.reps')}</div>
            <Stepper value={reps} onChange={(v) => (reps = v)} step={1} min={1} unit="reps" decimals={0} />
          </div>

          <!-- RPE -->
          <div style="margin-top:18px;">
            <div class="row" style="justify-content:space-between; margin-bottom:9px;">
              <span class="t-label" style="white-space:nowrap;">{$t('session.effortRpe')}</span>
              <span class="t-sub" style="font-size:12.5px; font-weight:700; color:{rpe != null ? rpeColor(rpe) : 'var(--ink3)'};">
                {rpe == null ? 'tap how hard it felt' : rpe >= 9.5 ? 'maximal' : rpe >= 8 ? 'hard — 1-2 left' : rpe >= 7 ? 'challenging' : 'easy'}
              </span>
            </div>
            <RPEScale value={rpe} onChange={(v) => (rpe = v)} from={6} to={10} />
          </div>
        </div>
      {/key}
    </div>
  </Screen>

  <Toast show={!!toast}>{toast}</Toast>

  <!-- rest sheet -->
  {#if resting}
    <div style="position:absolute; inset:0; z-index:90; display:flex; flex-direction:column; justify-content:flex-end;">
      <div onclick={() => (resting = false)} role="button" tabindex="0" style="position:absolute; inset:0; background:rgba(16,20,24,.42); backdrop-filter:blur(2px); animation:fadeIn .3s;"></div>
      <div style="position:relative; background:#fff; border-radius:30px 30px 0 0; padding:18px 24px calc(28px + 18px); animation:sheetUp .42s cubic-bezier(.2,.9,.3,1);">
        <div style="width:40px; height:5px; border-radius:5px; background:var(--line); margin:0 auto 18px;"></div>
        <div style="display:flex; flex-direction:column; align-items:center; gap:6px;">
          <div class="t-label">{$t('session.rest')}</div>
          <Ring pct={restPct} size={150} stroke={12} color="var(--brand)">
            <span class="t-num" style="font-size:42px;">{fmtClock(rest)}</span>
          </Ring>
          <div class="t-sub" style="font-size:14px; margin-top:8px; text-align:center;">
            {$t('session.upNext')}<b style="color:var(--ink);">{ex.name}</b><br />{$t('session.upNextSet', { i: setIndex + 1, w: weight, r: reps })}
          </div>
        </div>
        <div class="row" style="gap:10px; margin-top:20px;">
          <button class="btn btn--soft" style="flex:1;" onclick={() => { restTotal += 15; rest += 15; }}>+15s</button>
          <button class="btn btn--primary" style="flex:1.6;" onclick={() => (resting = false)}>{$t('session.skipRest')}</button>
        </div>
      </div>
    </div>
  {/if}
</div>
{/if}
