<script>
  // Progress — metrics dashboard (card grid).
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { progress, reloadProgress } from '$lib/stores.js';
  import { t } from '$lib/i18n/index.js';
  import { relativeDay, weightReps, num } from '$lib/format.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import Loading from '$lib/components/ui/Loading.svelte';
  import TabBar from '$lib/components/ui/TabBar.svelte';
  import Ring from '$lib/components/ui/Ring.svelte';
  import Sparkline from '$lib/components/ui/Sparkline.svelte';
  import Delta from '$lib/components/ui/Delta.svelte';

  onMount(() => {
    if (!API.isAuthed()) {
      goto('/onboarding', { replaceState: true });
      return;
    }
    reloadProgress();
  });

  let p = $derived($progress);
  // Sempre un array: il contratto lo garantisce lato server, ma il service
  // worker può restituire una risposta messa in cache da una build vecchia,
  // dove `prs` era null e questa pagina andava in TypeError.
  let prs = $derived(p?.prs ?? []);
  let wkPct = $derived(p ? p.weekSessions / p.weekGoal : 0);
  const heatmap = [0, 1, 3, 5, 6, 8, 9, 10, 12, 13];
</script>

{#if p === undefined}
  <Loading>
    {#snippet footer()}<TabBar />{/snippet}
  </Loading>
{:else}
  <Screen footerFlush>
    {#snippet footer()}<TabBar />{/snippet}
    <div class="stagger" style="padding:10px 20px 28px; display:flex; flex-direction:column; gap:14px;">
      <div class="row" style="justify-content:space-between; padding-top:6px;">
        <h1 class="t-title" style="font-size:28px;">{$t('progress.title')}</h1>
        <div class="seg" style="width:168px;">
          {#each ['week', 'month', 'year'] as x, i}
            <button class="seg__btn{i === 1 ? ' seg__btn--on' : ''}">{$t(`progress.${x}`)}</button>
          {/each}
        </div>
      </div>

      <!-- streak / consistency -->
      <div class="card" style="padding:16px; display:flex; align-items:center; gap:16px;">
        <Ring pct={wkPct} size={72} stroke={8}>
          <span class="t-num" style="font-size:21px;">{p.streak}</span><span class="t-label" style="font-size:8px;">{$t('progress.daysLabel')}</span>
        </Ring>
        <div style="flex:1;">
          <div class="t-h2" style="font-size:16px;">{$t('progress.consistency')}</div>
          <div class="t-sub" style="font-size:13px; margin-top:2px;">{$t('progress.thisWeekStreak', { done: p.weekSessions, goal: p.weekGoal, streak: p.streak })}</div>
          <div class="row" style="gap:5px; margin-top:10px;">
            {#each Array(14) as _, i}
              {@const on = heatmap.includes(i)}
              <span style="flex:1; height:22px; border-radius:5px; background:{on ? 'var(--brand)' : 'var(--line2)'}; opacity:{on ? 0.55 + i / 30 : 1};"></span>
            {/each}
          </div>
        </div>
      </div>

      <!-- metric grid -->
      <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
        {#each [{ label: $t('progress.est1rm', { lift: p.est1RM.lift.toUpperCase() }), value: p.est1RM.value, unit: 'kg', delta: p.est1RM.delta, series: p.est1RM.series, color: 'var(--brand)', invert: false }, { label: $t('progress.volume7d'), value: p.volume.value, unit: 'k kg', delta: p.volume.delta, series: p.volume.series, color: '#6E8BFF', invert: false }, { label: $t('progress.bodyWeight'), value: p.bodyWeight.value, unit: 'kg', delta: p.bodyWeight.delta, series: p.bodyWeight.series, color: '#9B7BFF', invert: true }] as g}
          <div class="card" style="padding:15px;">
            <div class="t-label" style="font-size:10px;">{g.label}</div>
            <div style="display:flex; align-items:baseline; gap:4px; margin-top:6px;">
              <span class="t-num" style="font-size:24px;">{$num(g.value)}</span>
              <span class="t-sub" style="font-size:12.5px; font-weight:700;">{g.unit}</span>
            </div>
            <div style="margin-top:3px;"><Delta v={g.delta} invert={g.invert} /></div>
            <div style="margin-top:10px;"><Sparkline data={g.series} w={130} h={38} color={g.color} /></div>
          </div>
        {/each}
        <div class="card" style="padding:15px; background:var(--brand-tint);">
          <div class="t-label" style="font-size:10px; color:var(--brand-ink);">{$t('progress.recentPrs')}</div>
          <div class="t-num" style="font-size:24px; margin-top:6px; color:var(--brand-ink);">{prs.length} <span style="font-size:15px;">★</span></div>
          <div style="margin-top:8px; display:flex; flex-direction:column; gap:3px;">
            {#each prs.slice(0, 3) as r}
              <span style="font-size:11.5px; font-weight:700; color:var(--brand-ink); white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">{r.lift}</span>
            {/each}
          </div>
        </div>
      </div>

      <!-- PR list -->
      <div>
        <div class="t-label" style="margin:4px 2px 8px;">{$t('progress.personalRecords')}</div>
        <div class="card" style="padding:4px 16px;">
          {#each prs as r, i}
            <div class="row" style="padding:13px 0; border-bottom:{i < prs.length - 1 ? '1px solid var(--line)' : 'none'};">
              <span style="font-size:18px; color:{r.fresh ? 'var(--amber)' : 'var(--ink4)'};">★</span>
              <div style="flex:1;"><div class="t-h2" style="font-size:15px;">{r.lift}</div><div class="t-sub" style="font-size:12.5px;">{$relativeDay(r.achievedAt)}</div></div>
              <span class="t-mono" style="font-size:15px; font-weight:800;">{$weightReps(r.weight, r.reps)}</span>
            </div>
          {/each}
        </div>
      </div>
    </div>
  </Screen>
{/if}
