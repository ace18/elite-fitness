<script>
  // You — profile + settings. Log out → /onboarding.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { t, locale, setLocale, LOCALES } from '$lib/i18n/index.js';
  import Screen from '$lib/components/ui/Screen.svelte';
  import TabBar from '$lib/components/ui/TabBar.svelte';
  import Avatar from '$lib/components/ui/Avatar.svelte';
  import MiniStat from '$lib/components/ui/MiniStat.svelte';
  import { displayName } from '$lib/user.js';

  let user = $state({ name: '', email: '' });

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

  let rows = $derived([
    [$t('you.units'), 'kg'],
    [$t('you.restTimer'), $t('you.restTimerValue')],
    [$t('you.appleHealth'), $t('you.connected')],
    [$t('you.notifications'), $t('you.on')],
    [$t('you.help'), '']
  ]);

  const LOCALE_NAMES = { it: 'Italiano', en: 'English' };
</script>

<Screen footerFlush>
  {#snippet footer()}<TabBar />{/snippet}
  <div class="scr-in" style="padding:12px 20px 28px; display:flex; flex-direction:column; gap:18px;">
    <div style="display:flex; flex-direction:column; align-items:center; gap:10px; padding-top:14px;">
      <Avatar name={displayName(user)} size={76} />
      <div style="text-align:center;">
        <div class="t-title" style="font-size:22px;">{displayName(user)}</div>
        <div class="t-sub" style="font-size:14px;">{user.email}</div>
      </div>
    </div>
    <div class="row" style="gap:11px;">
      <MiniStat label={$t('you.workouts')} value="86" />
      <MiniStat label={$t('you.streak')} value="12" unit="d" />
      <MiniStat label={$t('you.prs')} value="23" />
    </div>
    <!-- language switcher — the one setting on this screen that actually works -->
    <div class="card" style="padding:14px 16px;">
      <div class="row" style="justify-content:space-between; align-items:center; gap:12px;">
        <span style="flex:1; font-size:16px; font-weight:600;">{$t('you.language')}</span>
        <div class="seg" style="width:150px; flex:0 0 auto;">
          {#each LOCALES as l}
            <button class="seg__btn{$locale === l ? ' seg__btn--on' : ''}" onclick={() => setLocale(l)}>
              {LOCALE_NAMES[l]}
            </button>
          {/each}
        </div>
      </div>
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
    <button class="btn btn--soft btn--block" style="color:var(--down);" onclick={logout}>{$t('you.logOut')}</button>
  </div>
</Screen>
