<script>
  // Plan — choose a ready-made program or build a custom one.
  // Phases: browse | quiz | building | done.
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { applyPlan } from '$lib/stores.js';
  import { PREMADE_PLANS, PLAN_QUESTIONS, BUILD_MSGS, recommendPlan, normalizePlan } from '$lib/plans.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';
  import Ring from '$lib/components/ui/Ring.svelte';

  onMount(() => {
    if (!API.isAuthed()) goto('/onboarding', { replaceState: true });
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
  const startPlan = () => {
    applyPlan(plan);
    close();
  };

  // building animation → done. NOTE: read only `phase` here so the effect
  // doesn't re-run (and reset its timers) every time buildMsg ticks.
  let cyc, doneT;
  $effect(() => {
    if (phase !== 'building') return;
    buildMsg = 0;
    let m = 0;
    cyc = setInterval(() => {
      m = Math.min(m + 1, BUILD_MSGS.length - 1);
      buildMsg = m;
    }, 600);
    doneT = setTimeout(() => (phase = 'done'), 2500);
    return () => {
      clearInterval(cyc);
      clearTimeout(doneT);
    };
  });
  onDestroy(() => {
    clearInterval(cyc);
    clearTimeout(doneT);
  });

  const doneRows = $derived([
    ['Goal', plan.goal],
    ['Experience', plan.level],
    ['Schedule', `${plan.daysPerWeek} days / week`],
    ['Session length', `~${plan.sessionMin} min`],
    ['Program length', `${plan.totalWeeks} weeks`]
  ]);
</script>

{#if phase === 'browse'}
  <Screen>
    {#snippet footer()}
      <button class="btn btn--soft btn--block btn--lg" onclick={startCustom} style="display:flex; align-items:center; justify-content:center; gap:8px;">
        <span style="font-size:17px;">✨</span> Build a custom plan
      </button>
    {/snippet}
    <div class="stagger" style="padding:4px 20px 8px; display:flex; flex-direction:column; gap:16px;">
      <div class="row" style="gap:14px; align-items:flex-start;">
        <button onclick={close} style="width:38px; height:38px; flex:0 0 auto; border-radius:12px; border:none; background:#fff; box-shadow:var(--sh-sm); cursor:pointer; font-size:18px; color:var(--ink2);">‹</button>
        <div style="flex:1;">
          <h1 class="t-title" style="font-size:26px; margin:0; line-height:1.15;">Choose a plan</h1>
          <div class="t-sub" style="font-size:14.5px; margin-top:5px;">Start with a proven program, or build one tailored to you.</div>
        </div>
      </div>
      <div style="display:flex; flex-direction:column; gap:12px; margin-top:2px;">
        {#each PREMADE_PLANS as p (p.id)}
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
              {#each [p.goal, `${p.daysPerWeek}×/week`, `~${p.sessionMin} min`, p.level] as s}
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
        <div class="t-title" style="font-size:22px;">Building your plan</div>
        {#key buildMsg}
          <div class="t-sub float-up" style="font-size:14.5px; margin-top:6px;">{BUILD_MSGS[buildMsg]}</div>
        {/key}
      </div>
    </div>
  </Screen>
{:else if phase === 'done'}
  <Screen>
    {#snippet footer()}
      <div style="display:flex; flex-direction:column; gap:10px;">
        <Btn block lg onclick={startPlan}>Start this plan →</Btn>
        <button class="btn btn--soft btn--block" onclick={close}>Maybe later</button>
      </div>
    {/snippet}
    <div class="stagger" style="padding:14px 20px 8px; display:flex; flex-direction:column; gap:18px;">
      <div style="text-align:center; padding-top:8px;">
        <div class="pop-in" style="width:64px; height:64px; border-radius:50%; margin:0 auto 12px; background:var(--up-tint); display:flex; align-items:center; justify-content:center; font-size:30px;">✨</div>
        <div class="t-label" style="color:var(--brand);">{picked ? 'YOUR SELECTED PLAN' : 'YOUR RECOMMENDED PLAN'}</div>
        <div class="t-title" style="font-size:26px; margin-top:4px;">{plan.name}</div>
        <div class="t-sub" style="font-size:14px; margin-top:3px;">{plan.focus}</div>
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
        <span class="t-sub" style="font-size:12.5px; font-weight:700; color:var(--brand-ink);">Weights auto-adjust each week based on your logged effort.</span>
      </div>
    </div>
  </Screen>
{:else}
  <!-- quiz -->
  <Screen>
    {#snippet footer()}
      <Btn block lg onclick={next} disabled={!answered}>{isLast ? 'Build my plan →' : 'Continue'}</Btn>
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
          <h1 class="t-title" style="font-size:26px; margin:0; line-height:1.15;">{q.title}</h1>
          <div class="t-sub" style="font-size:14.5px; margin-top:6px;">{q.sub}</div>

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
                      <div class="t-h2" style="font-size:16px;">{o.title}</div>
                      <div class="t-sub" style="font-size:13px; margin-top:1px;">{o.desc}</div>
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
                    <div style="font-size:11.5px; font-weight:700; opacity:{on ? 0.9 : 0.55}; margin-top:2px;">{o.label || q.unit}</div>
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
