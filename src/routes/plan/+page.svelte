<script>
  // Plan — choose a ready-made program or build a custom one.
  // Phases: browse | quiz | building | done.
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { applyPlan } from '$lib/stores.js';
  import { PLAN_QUESTIONS, BUILD_MSGS, recommendPlan, normalizePlan } from '$lib/plans.js';
  import { t } from '$lib/i18n/index.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';
  import Ring from '$lib/components/ui/Ring.svelte';

  // The catalogue lives in the DB (plan_templates), not in plans.js — a
  // hardcoded copy would silently hide any template added by a migration.
  let premade = $state(null);
  let plansError = $state('');

  onMount(async () => {
    if (!API.isAuthed()) {
      goto('/onboarding', { replaceState: true });
      return;
    }
    try {
      premade = await API.getPlans();
    } catch (e) {
      plansError = e.status === 0 ? $t('plan.backendDown') : e.message;
      premade = [];
    }
  });

  let phase = $state('browse'); // browse | quiz | building | done
  let step = $state(0);
  let ans = $state({ goal: null, level: null, days: null, length: null });
  let picked = $state(null);
  let buildMsg = $state(0);

  let q = $derived(PLAN_QUESTIONS[step]);
  let answered = $derived(q ? ans[q.key] != null : false);
  let isLast = $derived(step === PLAN_QUESTIONS.length - 1);
  let recommended = $derived(recommendPlan(ans));
  let plan = $derived(picked || recommended);
  // The recommended plan carries i18n keys; a premade one carries the DB's
  // English strings, which stay as-is until the catalogue itself is translated.
  const shown = (p, keyField, plainField) => (p[keyField] ? $t(p[keyField]) : p[plainField]);
  let pct = $derived((step + 1) / PLAN_QUESTIONS.length);

  const close = () => goto('/train');
  const pick = (key, v) => (ans = { ...ans, [key]: v });
  const choosePremade = (p) => {
    picked = normalizePlan(p);
    phase = 'done';
  };
  const startCustom = () => {
    picked = null;
    step = 0;
    phase = 'quiz';
  };
  const next = () => {
    if (!isLast) {
      step += 1;
      return;
    }
    phase = 'building';
  };
  const back = () => {
    if (step === 0) phase = 'browse';
    else step -= 1;
  };
  let saving = $state(false);
  let error = $state('');
  // true once the custom plan has actually been generated server-side, so
  // "Start plan" doesn't try to create it a second time.
  let generated = $state(false);

  const startPlan = async () => {
    saving = true;
    error = '';
    try {
      // Premade plans map 1:1 onto the seeded plan_templates rows.
      if (picked && !generated) await API.setProgram(picked.id);
      await applyPlan(); // re-read program + today's workout from the server
      close();
    } catch (e) {
      error = e.status === 0 ? $t('plan.backendDown') : $t('plan.startFailed', { msg: e.message });
      saving = false;
    }
  };

  // Building phase: run the real AI generation while the messages cycle, and
  // move to `done` when the server answers (not on a fixed timer — a real
  // generation takes far longer than the old 2.5s animation).
  let cyc;
  $effect(() => {
    if (phase !== 'building') return;
    buildMsg = 0;
    let m = 0;
    cyc = setInterval(() => {
      m = Math.min(m + 1, BUILD_MSGS.length - 1);
      buildMsg = m;
    }, 600);

    let cancelled = false;
    error = '';
    API.generatePlan({
      goal: recommended.goal,
      level: recommended.level,
      days: ans.days,
      length: ans.length
    })
      .then(() => {
        if (cancelled) return;
        generated = true;
        phase = 'done';
      })
      .catch((e) => {
        if (cancelled) return;
        error =
          e.status === 0
            ? $t('plan.backendDown')
            : $t('plan.generateFailed', { msg: e.message });
        phase = 'quiz';
      });

    return () => {
      cancelled = true;
      clearInterval(cyc);
    };
  });
  onDestroy(() => clearInterval(cyc));

  const doneRows = $derived([
    [$t('plan.goal'), shown(plan, 'goalKey', 'goal')],
    [$t('plan.experience'), shown(plan, 'levelKey', 'level')],
    [$t('plan.schedule'), $t('plan.daysPerWeek', { n: plan.daysPerWeek })],
    [$t('plan.sessionLength'), $t('plan.aboutMin', { n: plan.sessionMin })],
    [$t('plan.programLength'), $t('plan.weeks', { n: plan.totalWeeks })]
  ]);
</script>

{#if phase === 'browse'}
  <Screen>
    {#snippet footer()}
      <button class="btn btn--soft btn--block btn--lg" onclick={startCustom} style="display:flex; align-items:center; justify-content:center; gap:8px;">
        <span style="font-size:17px;">✨</span> {$t('plan.buildCustom')}
      </button>
    {/snippet}
    <div class="stagger" style="padding:4px 20px 8px; display:flex; flex-direction:column; gap:16px;">
      <div class="row" style="gap:14px; align-items:flex-start;">
        <button onclick={close} style="width:38px; height:38px; flex:0 0 auto; border-radius:12px; border:none; background:#fff; box-shadow:var(--sh-sm); cursor:pointer; font-size:18px; color:var(--ink2);">‹</button>
        <div style="flex:1;">
          <h1 class="t-title" style="font-size:26px; margin:0; line-height:1.15;">{$t('plan.choosePlan')}</h1>
          <div class="t-sub" style="font-size:14.5px; margin-top:5px;">{$t('plan.chooseSub')}</div>
        </div>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px; margin-top:2px;">
        {#if premade === null}
          <div class="t-sub" style="font-size:13.5px; padding:8px 2px;">{$t('plan.loadingPlans')}</div>
        {:else if plansError}
          <div class="t-sub" style="font-size:13.5px; color:var(--down, #d1495b); padding:8px 2px;">{plansError}</div>
        {/if}
        {#each premade ?? [] as p (p.id)}
          <button class="card premade" onclick={() => choosePremade(p)}
            style="width:100%; text-align:left; border:none; cursor:pointer; padding:16px; display:flex; flex-direction:column; gap:12px; box-shadow:var(--sh-sm); -webkit-tap-highlight-color:transparent;">
            <div class="row" style="gap:13px; align-items:flex-start;">
              <div style="width:46px; height:46px; flex:0 0 auto; border-radius:14px; background:var(--brand-tint); display:flex; align-items:center; justify-content:center; font-size:23px;">{p.glyph}</div>
              <div style="flex:1; min-width:0;">
                <div class="row" style="gap:8px; align-items:center;">
                  <div class="t-h2" style="font-size:16.5px;">{p.name}</div>
                  {#if p.tag}<span class="chip chip--tint" style="font-size:10.5px; padding:3px 8px; font-weight:800;">{p.tag}</span>{/if}
                </div>
                <div class="t-sub" style="font-size:13px; margin-top:2px;">{p.focus}</div>
              </div>
              <span style="font-size:20px; color:var(--ink4); align-self:center;">›</span>
            </div>
            <div class="row" style="gap:7px; flex-wrap:wrap;">
              {#each [p.goal, $t('train.perWeek', { n: p.daysPerWeek }), $t('plan.aboutMin', { n: p.sessionMin }), p.level] as s}
                <span style="font-size:11.5px; font-weight:700; color:var(--ink2); background:#F1F3F5; padding:5px 10px; border-radius:8px; white-space:nowrap;">{s}</span>
              {/each}
            </div>
          </button>
        {/each}
      </div>
    </div>
  </Screen>
{:else if phase === 'building'}
  <Screen>
    <div style="height:100%; min-height:680px; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:22px; padding:30px;">
      <div style="animation:ringPulse 1.4s ease-in-out infinite;">
        <Ring pct={0.72} size={120} stroke={11}>
          <span style="font-size:34px;">🧠</span>
        </Ring>
      </div>
      <div style="text-align:center;">
        <div class="t-title" style="font-size:22px;">{$t('plan.building')}</div>
        {#key buildMsg}
          <div class="t-sub float-up" style="font-size:14.5px; margin-top:6px;">{$t(BUILD_MSGS[buildMsg])}</div>
        {/key}
      </div>
    </div>
  </Screen>
{:else if phase === 'done'}
  <Screen>
    {#snippet footer()}
      <div style="display:flex; flex-direction:column; gap:10px;">
        <Btn block lg onclick={startPlan} disabled={saving}>{saving ? $t('plan.starting') : $t('plan.startPlan')}</Btn>
        <button class="btn btn--soft btn--block" onclick={close}>{$t('plan.maybeLater')}</button>
        {#if error}
          <p class="t-sub" style="font-size:12.5px; color:var(--down, #d1495b); text-align:center; margin:0;">{error}</p>
        {/if}
      </div>
    {/snippet}
    <div class="stagger" style="padding:14px 20px 8px; display:flex; flex-direction:column; gap:18px;">
      <div style="text-align:center; padding-top:8px;">
        <div class="pop-in" style="width:64px; height:64px; border-radius:50%; margin:0 auto 12px; background:var(--up-tint); display:flex; align-items:center; justify-content:center; font-size:30px;">✨</div>
        <div class="t-label" style="color:var(--brand);">{picked ? $t('plan.selectedPlan') : $t('plan.recommendedPlan')}</div>
        <div class="t-title" style="font-size:26px; margin-top:4px;">{shown(plan, 'nameKey', 'name')}</div>
        <div class="t-sub" style="font-size:14px; margin-top:3px;">{shown(plan, 'focusKey', 'focus')}</div>
      </div>
      <div class="card" style="padding:4px 18px;">
        {#each doneRows as r, i}
          <div class="row" style="justify-content:space-between; padding:14px 0; border-bottom:{i < doneRows.length - 1 ? '1px solid var(--line)' : 'none'};">
            <span class="t-sub" style="font-size:14.5px;">{r[0]}</span>
            <span class="t-h2" style="font-size:14.5px; white-space:nowrap;">{r[1]}</span>
          </div>
        {/each}
      </div>
      <div class="row" style="gap:8px; padding:12px 14px; background:var(--brand-tint); border-radius:14px;">
        <span style="font-size:16px;">↻</span>
        <span class="t-sub" style="font-size:12.5px; font-weight:700; color:var(--brand-ink);">{$t('plan.autoAdjust')}</span>
      </div>
    </div>
  </Screen>
{:else}
  <!-- quiz -->
  <Screen>
    {#snippet footer()}
      <Btn block lg onclick={next} disabled={!answered}>{isLast ? $t('plan.buildMyPlan') : $t('common.continue')}</Btn>
      {#if error}
        <p class="t-sub" style="font-size:12.5px; color:var(--down, #d1495b); text-align:center; margin:10px 0 0;">{error}</p>
      {/if}
    {/snippet}
    <div style="padding:4px 20px 0;">
      <div class="row" style="gap:14px;">
        <button onclick={back} style="width:38px; height:38px; flex:0 0 auto; border-radius:12px; border:none; background:#fff; box-shadow:var(--sh-sm); cursor:pointer; font-size:18px; color:var(--ink2);">‹</button>
        <div style="flex:1; height:6px; background:#E4E8EB; border-radius:6px; overflow:hidden;">
          <div style="height:100%; width:{pct * 100}%; background:var(--brand); border-radius:6px; transition:width .35s cubic-bezier(.3,.9,.3,1);"></div>
        </div>
        <span class="t-mono t-label" style="font-size:11px; flex:0 0 auto;">{step + 1}/{PLAN_QUESTIONS.length}</span>
      </div>

      {#key q.key}
        <div class="scr-in" style="margin-top:24px;">
          <h1 class="t-title" style="font-size:26px; margin:0; line-height:1.15;">{$t(q.title)}</h1>
          <div class="t-sub" style="font-size:14.5px; margin-top:6px;">{$t(q.sub)}</div>

          <div style="margin-top:22px;">
            {#if q.kind === 'cards'}
              <div style="display:flex; flex-direction:column; gap:11px;">
                {#each q.options as o (o.v)}
                  {@const sel = ans[q.key] === o.v}
                  <button onclick={() => pick(q.key, o.v)} class="card"
                    style="width:100%; text-align:left; border:none; cursor:pointer; padding:15px 16px; display:flex; align-items:center; gap:14px;
                      box-shadow:{sel ? 'inset 0 0 0 2px var(--brand), var(--sh-md)' : 'inset 0 0 0 1.5px var(--line)'};
                      background:{sel ? 'var(--brand-tint)' : '#fff'}; transition:box-shadow .15s, background .15s; -webkit-tap-highlight-color:transparent;">
                    <div style="width:42px; height:42px; flex:0 0 auto; border-radius:13px; display:flex; align-items:center; justify-content:center; background:{sel ? 'var(--brand)' : '#F1F3F5'}; font-size:20px; transition:background .15s;">{o.glyph}</div>
                    <div style="flex:1; min-width:0;">
                      <div class="t-h2" style="font-size:16px;">{$t(o.title)}</div>
                      <div class="t-sub" style="font-size:13px; margin-top:1px;">{$t(o.desc)}</div>
                    </div>
                    <div style="width:24px; height:24px; flex:0 0 auto; border-radius:50%; display:flex; align-items:center; justify-content:center; background:{sel ? 'var(--brand)' : 'transparent'}; box-shadow:{sel ? 'none' : 'inset 0 0 0 2px var(--line)'};">
                      {#if sel}<span style="color:#fff; font-weight:800; font-size:13px;">✓</span>{/if}
                    </div>
                  </button>
                {/each}
              </div>
            {:else}
              <div style="display:grid; grid-template-columns:repeat({q.options.length > 4 ? 3 : q.options.length}, 1fr); gap:10px;">
                {#each q.options as o (o.v)}
                  {@const on = ans[q.key] === o.v}
                  <button onclick={() => pick(q.key, o.v)}
                    style="border:none; cursor:pointer; border-radius:16px; padding:16px 8px; background:{on ? 'var(--brand)' : '#fff'}; color:{on ? '#fff' : 'var(--ink)'};
                      box-shadow:{on ? 'var(--sh-md)' : 'inset 0 0 0 1.5px var(--line)'}; transition:.15s; -webkit-tap-highlight-color:transparent;">
                    <div class="t-num" style="font-size:24px;">{o.v}</div>
                    <div style="font-size:11.5px; font-weight:700; opacity:{on ? 0.9 : 0.55}; margin-top:2px;">{$t(o.label || q.unit)}</div>
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/key}
    </div>
  </Screen>
{/if}

<style>
  .premade {
    transition: box-shadow 0.15s, transform 0.1s;
  }
  .premade:active {
    transform: scale(0.985);
  }
</style>
