<script>
  // Train — active program + next session detail. Start → /session; browse → /plan.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { program, workout, reloadAll, summary, pendingSync, syncPending } from '$lib/stores.js';
  import { draftInfo, clearDraft } from '$lib/session-draft.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import { t } from '$lib/i18n/index.js';
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

  // Due stati recuperabili da rendere visibili qui, altrimenti restano
  // invisibili e l'utente crede di aver perso il lavoro:
  //  - un allenamento sospeso a metà (chiuso con ✕, refresh, app scaricata)
  //  - un allenamento finito il cui salvataggio è fallito. La ricevuta non è
  //    raggiungibile dal tab bar, quindi senza questo non ci sarebbe modo di
  //    riprovare.
  let draft = $state(draftInfo());
  let pendingSave = $derived($summary != null);

  function discardDraft() {
    clearDraft();
    draft = null;
  }

  let syncing = $state(false);
  async function retrySync() {
    syncing = true;
    try {
      await syncPending();
    } finally {
      syncing = false;
    }
  }

  let prog = $derived($program);
  let w = $derived($workout);
  // Vedi home/progress: `exercises` non ha omitempty, quindi una slice vuota
  // lato server arrivava come null e .reduce() qui sotto esplodeva.
  let exs = $derived(w?.exercises ?? []);
  let wkPct = $derived(prog ? prog.week / prog.totalWeeks : 0);
  let totalSets = $derived(exs.reduce((a, e) => a + e.sets, 0));
  let bumps = $derived(exs.filter((e) => e.suggested > e.last).length);
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
      <h1 class="t-title" style="font-size:22px; margin-top:10px;">{$t('common.noProgram')}</h1>
      <p class="t-sub" style="font-size:14px; margin-top:6px;">{$t('train.noProgramSub')}</p>
      <div style="margin-top:18px;"><Btn block lg onclick={() => goto('/plan')}>{$t('common.choosePlan')}</Btn></div>
    </div>
  </Screen>
{:else}
  <Screen footerFlush>
    {#snippet footer()}<TabBar />{/snippet}
    <div class="stagger" style="padding:8px 20px 28px; display:flex; flex-direction:column; gap:16px;">
      <!-- header -->
      <div class="row" style="justify-content:space-between; padding-top:6px;">
        <div>
          <div class="t-sub" style="font-size:14px; font-weight:600;">{$t('train.yourPlan')}</div>
          <div class="t-title" style="font-size:26px;">{$t('train.heading')}</div>
        </div>
        <button class="chip" style="padding:9px 14px;" onclick={() => goto('/plan')}>{$t('train.browsePlans')}</button>
      </div>

      <!-- active program card -->
      <div class="card" style="padding:16px;">
        <div class="row" style="justify-content:space-between; align-items:flex-start;">
          <div style="min-width:0;">
            <div class="t-label" style="color:var(--brand);">{$t('train.activeProgram')}</div>
            <div class="t-title" style="font-size:20px; margin-top:4px;">{prog.name}</div>
            <div class="t-sub" style="font-size:13px; margin-top:2px;">{prog.goal} · {prog.level} · {$t('train.perWeek', { n: prog.daysPerWeek })}</div>
          </div>
          <Ring pct={wkPct} size={58} stroke={7}>
            <span class="t-num" style="font-size:16px;">{prog.week}</span>
            <span class="t-label" style="font-size:7px;">{$t('train.wk')}</span>
          </Ring>
        </div>
        <div class="row" style="justify-content:space-between; margin-top:14px; margin-bottom:8px;">
          <span class="t-label" style="font-size:10px;">{$t('train.weekOf', { week: prog.week, total: prog.totalWeeks })}</span>
          <span class="t-sub" style="font-size:11.5px; font-weight:700;">{$t('train.weeksLeft', { n: prog.totalWeeks - prog.week })}</span>
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
        <div class="t-label" style="margin-bottom:8px; padding-left:2px;">{$t('train.nextSession')}</div>
        {#if !w}
          <div class="card" style="padding:20px 18px; text-align:center;">
            <div class="t-h2" style="font-size:16px;">{$t('common.restDay')}</div>
            <div class="t-sub" style="font-size:13px; margin-top:4px;">{$t('train.nothingToday')}</div>
          </div>
        {:else}
        <div class="card" style="overflow:hidden; box-shadow:var(--sh-md);">
          <div style="background:linear-gradient(135deg,var(--brand),var(--brand-press)); padding:18px 18px 20px; color:#fff; position:relative;">
            <div style="position:absolute; right:-30px; top:-30px; width:140px; height:140px; border-radius:50%; background:rgba(255,255,255,.12);"></div>
            <div style="position:relative;">
              <div style="font-size:12px; font-weight:800; letter-spacing:.08em; opacity:.85;">{$t('train.upNext')}</div>
              <div class="t-title" style="font-size:26px; margin-top:4px;">{w.name}</div>
              <div style="font-size:14px; opacity:.9; margin-top:3px;">{w.focus}</div>
              <div class="row" style="gap:14px; margin-top:14px;">
                <span style="font-size:13px; font-weight:700;">◳ {exs.length} {$t('common.exercises')}</span>
                <span style="font-size:13px; font-weight:700;">≡ {totalSets} {$t('common.sets')}</span>
                <span style="font-size:13px; font-weight:700;">◷ ~{w.estMin} {$t('common.min')}</span>
              </div>
            </div>
          </div>
          <div style="padding:6px 16px 4px;">
            {#each exs as ex, i}
              {@const up = ex.suggested > ex.last}
              <div class="row" style="padding:12px 0; border-bottom:{i < exs.length - 1 ? '1px solid var(--line)' : 'none'};">
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
                    <!-- Nessun peso da proporre: esercizio mai fatto e nessun
                         carico prescritto. Un "0 kg" si legge come "carica
                         zero", che non è quello che si intende. -->
                    {#if ex.suggested > 0}
                      {ex.suggested} kg {#if up}<span style="color:var(--up);">↑</span>{/if}
                    {:else}
                      <span style="color:var(--ink4);">—</span>
                    {/if}
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
                  {bumps === 1 ? $t('train.bumpedOne', { n: bumps }) : $t('train.bumpedMany', { n: bumps })}
                </span>
              </div>
            {/if}
            {#if $pendingSync > 0}
              <!-- In coda e fuori dalle mani dell'utente: non è un errore, ma
                   dev'essere visibile, altrimenti "in coda" e "perso" si
                   assomigliano troppo. -->
              <div class="row" style="gap:8px; padding:10px 12px; background:var(--brand-tint); border-radius:12px; margin-bottom:12px;">
                <span style="font-size:15px;">☁</span>
                <span class="t-sub" style="font-size:12.5px; font-weight:700; color:var(--brand-ink); flex:1;">
                  {$pendingSync === 1 ? $t('train.syncPendingOne') : $t('train.syncPendingMany', { n: $pendingSync })}
                </span>
              </div>
              <button class="chip chip--tint" style="margin:0 auto 12px; display:block;" onclick={retrySync} disabled={syncing}>
                {syncing ? $t('train.syncing') : $t('train.syncNow')}
              </button>
              <Btn block lg onclick={() => goto('/session')}>{$t('train.startSession')}</Btn>
            {:else if pendingSave}
              <div class="row" style="gap:8px; padding:10px 12px; background:var(--up-tint); border-radius:12px; margin-bottom:12px;">
                <span style="font-size:15px;">↻</span>
                <span class="t-sub" style="font-size:12.5px; font-weight:700; color:#0a7a44; flex:1;">{$t('train.pendingSave')}</span>
              </div>
              <Btn block lg onclick={() => goto('/receipt')}>{$t('train.retrySave')}</Btn>
            {:else if draft}
              <div class="row" style="gap:8px; padding:10px 12px; background:var(--brand-tint); border-radius:12px; margin-bottom:12px;">
                <span style="font-size:15px;">⏸</span>
                <span class="t-sub" style="font-size:12.5px; font-weight:700; color:var(--brand-ink); flex:1;">{$t('train.draftPending', { n: draft.doneSets })}</span>
              </div>
              <Btn block lg onclick={() => goto('/session')}>{$t('train.resumeSession')}</Btn>
              <button
                class="chip"
                style="margin:10px auto 0; display:block; color:var(--ink3);"
                onclick={discardDraft}>{$t('train.discardDraft')}</button>
            {:else}
              <Btn block lg onclick={() => goto('/session')}>{$t('train.startSession')}</Btn>
            {/if}
          </div>
        </div>
        {/if}
      </div>

      <!-- this week schedule -->
      <div>
        <div class="t-label" style="margin-bottom:8px; padding-left:2px;">{$t('train.thisWeek')}</div>
        <div class="card" style="padding:4px 16px;">
          {#each prog.schedule as d, i}
            {@const rest = d.status === 'rest'}
            {@const today = d.status === 'today'}
            {@const isDone = d.status === 'done'}
            <div class="row" style="padding:13px 0; border-bottom:{i < prog.schedule.length - 1 ? '1px solid var(--line)' : 'none'}; align-items:center; opacity:{rest ? 0.62 : 1};">
              <div style="width:42px; flex:0 0 auto; text-align:center;">
                <div class="t-label" style="font-size:10px; color:{today ? 'var(--brand)' : 'var(--ink3)'};">{$t(`weekday.${d.dayIndex}`)}</div>
              </div>
              <div style="width:30px; height:30px; flex:0 0 auto; border-radius:50%; display:flex; align-items:center; justify-content:center;
                background:{isDone ? 'var(--brand)' : today ? 'var(--brand-tint)' : '#EEF1F4'};
                box-shadow:{today ? 'inset 0 0 0 2px var(--brand)' : 'none'};">
                {#if isDone}<span style="color:#fff; font-weight:800; font-size:14px;">✓</span>
                {:else if today}<span style="width:9px; height:9px; border-radius:50%; background:var(--brand);"></span>
                {:else}<span style="width:11px; height:2.5px; border-radius:2px; background:var(--ink4);"></span>{/if}
              </div>
              <div style="flex:1; min-width:0;">
                <div class="t-h2" style="font-size:15px; color:{rest ? 'var(--ink2)' : 'var(--ink)'};">{d.name ?? $t('common.rest')}</div>
                <div class="t-sub" style="font-size:12.5px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">{d.focus ?? $t('common.activeRecovery')}</div>
              </div>
              {#if isDone}<span class="t-mono t-sub" style="font-size:12.5px; font-weight:700;">{d.volume}</span>{/if}
              {#if today}<span class="chip chip--tint" style="font-size:11.5px; padding:5px 10px;">{$t('common.today')}</span>{/if}
            </div>
          {/each}
        </div>
      </div>
    </div>
  </Screen>
{/if}
