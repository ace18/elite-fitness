<script>
  // Train — active program + next session detail. Start → /session; browse → /plan.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { program, workout, reloadAll } from '$lib/stores.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import Loading from '$lib/components/ui/Loading.svelte';
  import TabBar from '$lib/components/ui/TabBar.svelte';
  import Ring from '$lib/components/ui/Ring.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';

  onMount(() => {
    if (!API.isAuthed()) {
      goto('/onboarding', { replaceState: true });
      return;
    }
    reloadAll();
  });

  let prog = $derived($program);
  let w = $derived($workout);
  let wkPct = $derived(prog ? prog.week / prog.totalWeeks : 0);
  let totalSets = $derived(w ? w.exercises.reduce((a, e) => a + e.sets, 0) : 0);
  let bumps = $derived(w ? w.exercises.filter((e) => e.suggested > e.last).length : 0);
</script>

{#if prog === undefined || w === undefined}
  <Loading>
    {#snippet footer()}<TabBar />{/snippet}
  </Loading>
{:else if prog === null}
  <!-- authenticated but no active program yet -->
  <Screen footerFlush>
    {#snippet footer()}<TabBar />{/snippet}
    <div style="padding:60px 24px; text-align:center;">
      <div style="font-size:34px;">✨</div>
      <h1 class="t-title" style="font-size:22px; margin-top:10px;">No program yet</h1>
      <p class="t-sub" style="font-size:14px; margin-top:6px;">Pick a ready-made plan or build a custom one.</p>
      <div style="margin-top:18px;"><Btn block lg onclick={() => goto('/plan')}>Choose a plan →</Btn></div>
    </div>
  </Screen>
{:else}
  <Screen footerFlush>
    {#snippet footer()}<TabBar />{/snippet}
    <div class="stagger" style="padding:8px 20px 28px; display:flex; flex-direction:column; gap:16px;">
      <!-- header -->
      <div class="row" style="justify-content:space-between; padding-top:6px;">
        <div>
          <div class="t-sub" style="font-size:14px; font-weight:600;">Your plan</div>
          <div class="t-title" style="font-size:26px;">Training</div>
        </div>
        <button class="chip" style="padding:9px 14px;" onclick={() => goto('/plan')}>Browse plans</button>
      </div>

      <!-- active program card -->
      <div class="card" style="padding:16px;">
        <div class="row" style="justify-content:space-between; align-items:flex-start;">
          <div style="min-width:0;">
            <div class="t-label" style="color:var(--brand);">ACTIVE PROGRAM</div>
            <div class="t-title" style="font-size:20px; margin-top:4px;">{prog.name}</div>
            <div class="t-sub" style="font-size:13px; margin-top:2px;">{prog.goal} · {prog.level} · {prog.daysPerWeek}×/week</div>
          </div>
          <Ring pct={wkPct} size={58} stroke={7}>
            <span class="t-num" style="font-size:16px;">{prog.week}</span>
            <span class="t-label" style="font-size:7px;">WK</span>
          </Ring>
        </div>
        <div class="row" style="justify-content:space-between; margin-top:14px; margin-bottom:8px;">
          <span class="t-label" style="font-size:10px;">WEEK {prog.week} OF {prog.totalWeeks}</span>
          <span class="t-sub" style="font-size:11.5px; font-weight:700;">{prog.totalWeeks - prog.week} weeks left</span>
        </div>
        <div class="row" style="gap:5px;">
          {#each Array(prog.totalWeeks) as _, i}
            <span style="flex:1; height:7px; border-radius:6px;
              background:{i < prog.week ? 'var(--brand)' : 'var(--line)'};
              opacity:{i === prog.week - 1 ? 0.55 : 1};"></span>
          {/each}
        </div>
      </div>

      <!-- NEXT SESSION hero -->
      <div>
        <div class="t-label" style="margin-bottom:8px; padding-left:2px;">Next session</div>
        {#if !w}
          <div class="card" style="padding:20px 18px; text-align:center;">
            <div class="t-h2" style="font-size:16px;">Rest day</div>
            <div class="t-sub" style="font-size:13px; margin-top:4px;">Nothing scheduled today.</div>
          </div>
        {:else}
        <div class="card" style="overflow:hidden; box-shadow:var(--sh-md);">
          <div style="background:linear-gradient(135deg,var(--brand),var(--brand-press)); padding:18px 18px 20px; color:#fff; position:relative;">
            <div style="position:absolute; right:-30px; top:-30px; width:140px; height:140px; border-radius:50%; background:rgba(255,255,255,.12);"></div>
            <div style="position:relative;">
              <div style="font-size:12px; font-weight:800; letter-spacing:.08em; opacity:.85;">UP NEXT · TODAY</div>
              <div class="t-title" style="font-size:26px; margin-top:4px;">{w.name}</div>
              <div style="font-size:14px; opacity:.9; margin-top:3px;">{w.focus}</div>
              <div class="row" style="gap:14px; margin-top:14px;">
                <span style="font-size:13px; font-weight:700;">◳ {w.exercises.length} exercises</span>
                <span style="font-size:13px; font-weight:700;">≡ {totalSets} sets</span>
                <span style="font-size:13px; font-weight:700;">◷ ~{w.estMin} min</span>
              </div>
            </div>
          </div>
          <div style="padding:6px 16px 4px;">
            {#each w.exercises as ex, i}
              {@const up = ex.suggested > ex.last}
              <div class="row" style="padding:12px 0; border-bottom:{i < w.exercises.length - 1 ? '1px solid var(--line)' : 'none'};">
                <div style="width:26px; height:26px; flex:0 0 auto; border-radius:9px; background:var(--brand-tint); display:flex; align-items:center; justify-content:center;">
                  <span style="color:var(--brand-ink); font-weight:800; font-size:13px;">{i + 1}</span>
                </div>
                <div style="flex:1; min-width:0;">
                  <div class="t-h2" style="font-size:14.5px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">{ex.name}</div>
                  <div class="t-sub" style="font-size:12px;">{ex.muscle}</div>
                </div>
                <div style="text-align:right; flex:0 0 auto;">
                  <div class="t-mono" style="font-size:14px; font-weight:800; white-space:nowrap;">{ex.sets} × {ex.targetReps}</div>
                  <div class="t-mono t-sub" style="font-size:11.5px; font-weight:700; white-space:nowrap; display:flex; align-items:center; gap:4px; justify-content:flex-end;">
                    {ex.suggested} kg {#if up}<span style="color:var(--up);">↑</span>{/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>
          <div style="padding:8px 14px 16px;">
            {#if bumps > 0}
              <div class="row" style="gap:8px; padding:10px 12px; background:var(--brand-tint); border-radius:12px; margin-bottom:12px;">
                <span style="font-size:15px;">↑</span>
                <span class="t-sub" style="font-size:12.5px; font-weight:700; color:var(--brand-ink);">
                  {bumps} {bumps === 1 ? 'lift' : 'lifts'} bumped up from last week — progressive overload
                </span>
              </div>
            {/if}
            <Btn block lg onclick={() => goto('/session')}>Start session →</Btn>
          </div>
        </div>
        {/if}
      </div>

      <!-- this week schedule -->
      <div>
        <div class="t-label" style="margin-bottom:8px; padding-left:2px;">This week</div>
        <div class="card" style="padding:4px 16px;">
          {#each prog.schedule as d, i}
            {@const rest = d.status === 'rest'}
            {@const today = d.status === 'today'}
            {@const isDone = d.status === 'done'}
            <div class="row" style="padding:13px 0; border-bottom:{i < prog.schedule.length - 1 ? '1px solid var(--line)' : 'none'}; align-items:center; opacity:{rest ? 0.62 : 1};">
              <div style="width:42px; flex:0 0 auto; text-align:center;">
                <div class="t-label" style="font-size:10px; color:{today ? 'var(--brand)' : 'var(--ink3)'};">{d.day}</div>
              </div>
              <div style="width:30px; height:30px; flex:0 0 auto; border-radius:50%; display:flex; align-items:center; justify-content:center;
                background:{isDone ? 'var(--brand)' : today ? 'var(--brand-tint)' : '#EEF1F4'};
                box-shadow:{today ? 'inset 0 0 0 2px var(--brand)' : 'none'};">
                {#if isDone}<span style="color:#fff; font-weight:800; font-size:14px;">✓</span>
                {:else if today}<span style="width:9px; height:9px; border-radius:50%; background:var(--brand);"></span>
                {:else}<span style="width:11px; height:2.5px; border-radius:2px; background:var(--ink4);"></span>{/if}
              </div>
              <div style="flex:1; min-width:0;">
                <div class="t-h2" style="font-size:15px; color:{rest ? 'var(--ink2)' : 'var(--ink)'};">{d.name}</div>
                <div class="t-sub" style="font-size:12.5px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">{d.focus}</div>
              </div>
              {#if isDone}<span class="t-mono t-sub" style="font-size:12.5px; font-weight:700;">{d.volume}</span>{/if}
              {#if today}<span class="chip chip--tint" style="font-size:11.5px; padding:5px 10px;">Today</span>{/if}
            </div>
          {/each}
        </div>
      </div>
    </div>
  </Screen>
{/if}
