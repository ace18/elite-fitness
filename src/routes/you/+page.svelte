<script>
  // You — profile + settings. Log out → /onboarding.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import TabBar from '$lib/components/ui/TabBar.svelte';
  import Avatar from '$lib/components/ui/Avatar.svelte';
  import MiniStat from '$lib/components/ui/MiniStat.svelte';

  let user = $state({ name: 'Alex' });

  onMount(() => {
    if (!API.isAuthed()) {
      goto('/onboarding', { replaceState: true });
      return;
    }
    user = API.user();
  });

  function logout() {
    API.logout();
    goto('/onboarding');
  }

  const rows = [
    ['Units', 'kg'],
    ['Rest timer', 'Auto · 90s'],
    ['Apple Health', 'Connected'],
    ['Notifications', 'On'],
    ['Help & support', '']
  ];
</script>

<Screen footerFlush>
  {#snippet footer()}<TabBar />{/snippet}
  <div class="scr-in" style="padding:12px 20px 28px; display:flex; flex-direction:column; gap:18px;">
    <div style="display:flex; flex-direction:column; align-items:center; gap:10px; padding-top:14px;">
      <Avatar name={user.name} size={76} />
      <div style="text-align:center;">
        <div class="t-title" style="font-size:22px;">{user.name || 'Alex'}</div>
        <div class="t-sub" style="font-size:14px;">{user.email || 'alex@elite.fit'}</div>
      </div>
    </div>
    <div class="row" style="gap:11px;">
      <MiniStat label="WORKOUTS" value="86" />
      <MiniStat label="STREAK" value="12" unit="d" />
      <MiniStat label="PRS" value="23" />
    </div>
    <div class="card" style="padding:2px 16px;">
      {#each rows as r, i}
        <div class="row" style="padding:15px 0; border-bottom:{i < rows.length - 1 ? '1px solid var(--line)' : 'none'};">
          <span style="flex:1; font-size:16px; font-weight:600;">{r[0]}</span>
          <span class="t-sub" style="font-size:15px;">{r[1]}</span>
          <span style="color:var(--ink4); margin-left:8px;">›</span>
        </div>
      {/each}
    </div>
    <button class="btn btn--soft btn--block" style="color:var(--down);" onclick={logout}>Log out</button>
  </div>
</Screen>
