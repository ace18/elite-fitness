<script>
  // Home — dashboard. Start workout → /session; avatar → /you.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { workout, progress, reloadAll } from '$lib/stores.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import Loading from '$lib/components/ui/Loading.svelte';
  import TabBar from '$lib/components/ui/TabBar.svelte';
  import Ring from '$lib/components/ui/Ring.svelte';
  import Avatar from '$lib/components/ui/Avatar.svelte';
  import MiniStat from '$lib/components/ui/MiniStat.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';

  let user = $state({ name: 'Alex' });

  onMount(() => {
    if (!API.isAuthed()) {
      goto('/onboarding', { replaceState: true });
      return;
    }
    user = API.user(); // cached, instant
    API.refreshUser()
      .then((u) => (user = u))
      .catch(() => {}); // guards handle 401; stale cached name is fine otherwise
    reloadAll();
  });

  function greeting() {
    const h = new Date().getHours();
    return h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening';
  }

  let w = $derived($workout);
  let p = $derived($progress);
  let wkPct = $derived(p ? p.weekSessions / p.weekGoal : 0);

  const recent = [
    ['Pull Day B', '2 days ago', '14,120 kg'],
    ['Leg Day', '4 days ago', '19,450 kg']
  ];
</script>

{#if w === undefined || p === undefined}
  <Loading>
    {#snippet footer()}<TabBar />{/snippet}
  </Loading>
{:else}
  <Screen footerFlush>
    {#snippet footer()}<TabBar />{/snippet}
    <div class="stagger" style="padding:8px 20px 28px; display:flex; flex-direction:column; gap:16px;">
      <!-- header -->
      <div class="row" style="justify-content:space-between; padding-top:6px;">
        <div>
          <div class="t-sub" style="font-size:14px; font-weight:600;">{greeting()},</div>
          <div class="t-title" style="font-size:26px;">{user.name || 'Alex'} 👋</div>
        </div>
        <div onclick={() => goto('/you')} role="button" tabindex="0" style="cursor:pointer;"><Avatar name={user.name} /></div>
      </div>

      <!-- streak + week ring -->
      <div class="card" style="padding:16px; display:flex; align-items:center; gap:16px;">
        <Ring pct={wkPct} size={74} stroke={8}>
          <span class="t-num" style="font-size:22px;">{p.streak}</span>
          <span class="t-label" style="font-size:8px;">DAYS</span>
        </Ring>
        <div style="flex:1;">
          <div class="t-h2" style="font-size:16px;">{p.streak}-day streak 🔥</div>
          <div class="t-sub" style="font-size:13px; margin-top:2px;">{p.weekSessions} of {p.weekGoal} workouts this week</div>
          <div class="row" style="gap:6px; margin-top:10px;">
            {#each Array(p.weekGoal) as _, i}
              <span style="flex:1; height:7px; border-radius:6px; background:{i < p.weekSessions ? 'var(--brand)' : 'var(--line)'};"></span>
            {/each}
          </div>
        </div>
      </div>

      <!-- today's workout hero -->
      <div>
        <div class="t-label" style="margin-bottom:8px; padding-left:2px;">Today's plan</div>
        {#if w}
          <div class="card" style="overflow:hidden; box-shadow:var(--sh-md);">
            <div style="background:linear-gradient(135deg,var(--brand),var(--brand-press)); padding:18px 18px 20px; color:#fff; position:relative;">
              <div style="position:absolute; right:-30px; top:-30px; width:140px; height:140px; border-radius:50%; background:rgba(255,255,255,.12);"></div>
              <div style="position:relative;">
                <div style="font-size:12px; font-weight:800; letter-spacing:.08em; opacity:.85;">READY WHEN YOU ARE</div>
                <div class="t-title" style="font-size:27px; margin-top:4px;">{w.name}</div>
                <div style="font-size:14px; opacity:.9; margin-top:3px;">{w.focus}</div>
                <div class="row" style="gap:14px; margin-top:14px;">
                  <span style="font-size:13px; font-weight:700;">◳ {w.exercises.length} exercises</span>
                  <span style="font-size:13px; font-weight:700;">◷ ~{w.estMin} min</span>
                </div>
              </div>
            </div>
            <div style="padding:14px;">
              <Btn block lg onclick={() => goto('/session')}>Start workout →</Btn>
            </div>
          </div>
        {:else}
          <!-- loaded, but nothing scheduled: rest day, or no program yet -->
          <div class="card" style="padding:22px 18px; text-align:center;">
            <div style="font-size:30px;">{p ? '😌' : '✨'}</div>
            <div class="t-h2" style="font-size:17px; margin-top:8px;">
              {p ? 'Rest day' : 'No program yet'}
            </div>
            <div class="t-sub" style="font-size:13.5px; margin-top:4px;">
              {p ? 'Nothing scheduled today — recover well.' : 'Pick a plan to get your first workout.'}
            </div>
            <div style="margin-top:14px;">
              <Btn block onclick={() => goto('/plan')}>{p ? 'Browse plans' : 'Choose a plan →'}</Btn>
            </div>
          </div>
        {/if}
      </div>

      <!-- this week mini stats -->
      <div>
        <div class="t-label" style="margin-bottom:8px; padding-left:2px;">This week</div>
        <div class="row" style="gap:11px;">
          <MiniStat label="VOLUME" value={p.volume.value} unit="k kg" delta={p.volume.delta} />
          <MiniStat label="EST. 1RM" value={p.est1RM.value} unit="kg" delta={p.est1RM.delta} />
          <MiniStat label="NEW PRS" value={p.prs.filter((x) => x.fresh).length} unit="★" />
        </div>
      </div>

      <!-- recent -->
      <div>
        <div class="t-label" style="margin-bottom:8px; padding-left:2px;">Recent</div>
        <div class="card" style="padding:4px 16px;">
          {#each recent as r, i}
            <div class="row" style="padding:13px 0; border-bottom:{i < recent.length - 1 ? '1px solid var(--line)' : 'none'};">
              <div style="width:36px; height:36px; border-radius:11px; background:var(--brand-tint); display:flex; align-items:center; justify-content:center;">
                <span style="color:var(--brand); font-weight:800; font-size:15px;">✓</span>
              </div>
              <div style="flex:1;"><div class="t-h2" style="font-size:15px;">{r[0]}</div><div class="t-sub" style="font-size:12.5px;">{r[1]}</div></div>
              <span class="t-mono t-sub" style="font-size:13px; font-weight:700;">{r[2]}</span>
            </div>
          {/each}
        </div>
      </div>
    </div>
  </Screen>
{/if}
