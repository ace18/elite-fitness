<script>
  // Receipt — post-workout summary. Save & finish → /home.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { summary, reloadProgress, refreshPendingSync } from '$lib/stores.js';
  import { t, intlLocale } from '$lib/i18n/index.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';
  import RPEScale from '$lib/components/ui/RPEScale.svelte';

  onMount(() => {
    if (!API.isAuthed()) {
      goto('/onboarding', { replaceState: true });
      return;
    }
    // No in-flight session (direct navigation or refresh) — nothing to show.
    if (!$summary) goto('/home', { replaceState: true });
  });

  let s = $derived($summary);
  let sRpe = $state(null);
  let saving = $state(false);
  let error = $state('');

  async function save() {
    saving = true;
    error = '';
    try {
      const res = await API.saveSession({ ...s, sessionRpe: sRpe });
      // Messo in coda invece di spedito: per l'utente è finita comunque, quindi
      // si va avanti. Il riassunto si può togliere perché la coda è ora la copia
      // durevole, e Allena mostra quante sincronizzazioni sono in sospeso.
      if (res?.queued) {
        summary.set(null);
        await refreshPendingSync();
        goto('/home');
        return;
      }
      await reloadProgress();
      summary.set(null);
      goto('/home');
    } catch (e) {
      error =
        e.status === 0
          ? $t('receipt.backendDown')
          : $t('receipt.saveFailed', { msg: e.message });
      saving = false;
    }
  }
</script>

{#if s}
<Screen bg="#F4F6F8">
  {#snippet footer()}
    <Btn block lg onclick={save} disabled={saving}>{saving ? $t('receipt.saving') : $t('receipt.save')}</Btn>
    {#if error}
      <p class="t-sub" style="font-size:12.5px; color:var(--down, #d1495b); text-align:center; margin:10px 0 0;">{error}</p>
      <!-- Il riassunto è persistito in locale: senza dirlo, un salvataggio
           fallito sembra un allenamento perso e l'utente riprova a vuoto o
           chiude l'app rassegnato. -->
      <p class="t-sub" style="font-size:12px; text-align:center; margin:6px 0 0;">{$t('receipt.keptLocally')}</p>
    {/if}
  {/snippet}
  <div style="padding:30px 20px 10px;">
    <!-- hero -->
    <div style="display:flex; flex-direction:column; align-items:center; text-align:center;">
      <div class="pop-in" style="width:72px; height:72px; border-radius:50%; background:var(--brand); display:flex; align-items:center; justify-content:center; box-shadow:0 10px 24px rgba(15,181,174,.4);">
        <svg width="36" height="36" viewBox="0 0 24 24" fill="none"><path d="M5 12.5l4.5 4.5L19 7.5" stroke="#fff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </div>
      <h1 class="t-title scr-in" style="font-size:24px; line-height:1.12; margin:18px 0 7px;">{$t('receipt.done', { name: s.name })}</h1>
      <div class="t-sub scr-in" style="font-size:14.5px;">{$t('receipt.summary', { min: s.durationMin, exercises: s.exercises, sets: s.sets })}</div>
    </div>

    <!-- stat grid -->
    <div class="stagger" style="display:grid; grid-template-columns:1fr 1fr; gap:11px; margin-top:24px;">
      {#each [[$t('receipt.totalVolume'), s.volume.toLocaleString($intlLocale), 'kg', true, false], [$t('receipt.topSet'), s.topSet, '', false, false], [$t('receipt.avgRpe'), s.avgRpe ?? '—', '', false, false], [$t('receipt.newPrs'), s.prs.length, '★', false, true]] as [label, value, unit, big, accent]}
        <div class="card" style="padding:15px 16px; background:{accent ? 'var(--brand-tint)' : '#fff'};">
          <div class="t-label" style="font-size:10px;">{label}</div>
          <div style="display:flex; align-items:baseline; gap:4px; margin-top:6px;">
            <span class="t-num" style="font-size:{big ? 26 : 23}px; color:{accent ? 'var(--brand-ink)' : 'var(--ink)'};">{value}</span>
            {#if unit}<span class="t-sub" style="font-size:13px; font-weight:700; color:{accent ? 'var(--brand-ink)' : 'var(--ink2)'};">{unit}</span>{/if}
          </div>
        </div>
      {/each}
    </div>

    <!-- PR callout -->
    {#if s.prs.length > 0}
      <div class="float-up card" style="margin-top:12px; padding:14px 16px; background:linear-gradient(120deg,#fff,#fff7e8); box-shadow:var(--sh-sm);">
        <div class="row" style="gap:10px;">
          <span style="font-size:22px;">🏆</span>
          <div style="flex:1;">
            <div class="t-h2" style="font-size:14px;">{s.prs.length > 1 ? $t('receipt.newPrPlural') : $t('receipt.newPr')}</div>
            {#each s.prs as p}
              <div class="t-sub" style="font-size:13px;">{p.lift} · <b style="color:var(--ink);">{p.value}</b></div>
            {/each}
          </div>
        </div>
      </div>
    {/if}

    <!-- per-exercise breakdown -->
    <div class="t-label" style="margin:22px 2px 8px;">{$t('receipt.sessionDetail')}</div>
    <div class="card" style="padding:4px 16px;">
      {#each s.perExercise as e, i}
        {@const up = e.best > e.last}
        {@const same = e.best === e.last}
        <div class="row" style="padding:13px 0; border-bottom:{i < s.perExercise.length - 1 ? '1px solid var(--line)' : 'none'};">
          <div style="flex:1;">
            <div class="t-h2" style="font-size:15px;">{e.name}</div>
            <div class="t-sub" style="font-size:12.5px;">{$t('receipt.setsAndVolume', { sets: e.sets, vol: e.vol.toLocaleString($intlLocale) })}</div>
          </div>
          <span style="font-size:12.5px; font-weight:800; color:{same ? 'var(--ink3)' : up ? 'var(--up)' : 'var(--down)'};">
            {same ? $t('receipt.same') : `${up ? '▲' : '▼'} ${e.best} kg`}
          </span>
        </div>
      {/each}
    </div>

    <!-- session RPE -->
    <div class="card" style="margin-top:14px; padding:16px 16px 18px;">
      <div class="t-h2" style="font-size:15.5px;">{$t('receipt.howHard')}</div>
      <div class="t-sub" style="font-size:13px; margin:3px 0 14px;">{$t('receipt.tuneLoads')}</div>
      <RPEScale value={sRpe} onChange={(v) => (sRpe = v)} from={4} to={10} />
    </div>
  </div>
</Screen>
{/if}
