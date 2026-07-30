<script>
  // TabBar — bottom navigation. Active tab derived from the current route.
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { t } from '$lib/i18n/index.js';

  const items = [
    ['home', '/home'],
    ['train', '/train'],
    ['progress', '/progress'],
    ['you', '/you']
  ];

  let active = $derived($page.url.pathname.split('/')[1] || 'home');
</script>

<div style="display:flex; padding:10px 8px calc(8px + env(safe-area-inset-bottom, 0px)); background:rgba(255,255,255,0.86);
  backdrop-filter:blur(16px); -webkit-backdrop-filter:blur(16px); border-top:1px solid var(--line);">
  {#each items as [k, href]}
    {@const on = active === k}
    {@const c = on ? 'var(--brand)' : '#A6ACB3'}
    <button
      onclick={() => goto(href)}
      style="flex:1; border:none; background:transparent; cursor:pointer; display:flex;
        flex-direction:column; align-items:center; gap:4px; -webkit-tap-highlight-color:transparent;">
      {#if k === 'home'}
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><path d="M4 10.5 12 4l8 6.5V19a1 1 0 0 1-1 1h-4v-5h-6v5H5a1 1 0 0 1-1-1z" stroke={c} stroke-width="2.2" stroke-linejoin="round" /></svg>
      {:else if k === 'train'}
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><circle cx="6" cy="12" r="2.4" stroke={c} stroke-width="2.2" /><circle cx="18" cy="12" r="2.4" stroke={c} stroke-width="2.2" /><path d="M8.4 12h7.2" stroke={c} stroke-width="2.2" stroke-linecap="round" /><path d="M3 12h0.6M20.4 12H21" stroke={c} stroke-width="2.2" stroke-linecap="round" /></svg>
      {:else if k === 'progress'}
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><path d="M4 15l4-4 3 3 5-6" stroke={c} stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" /><path d="M4 20h16" stroke={c} stroke-width="2.2" stroke-linecap="round" /></svg>
      {:else}
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="8.5" r="3.3" stroke={c} stroke-width="2.2" /><path d="M5.5 19.5c.6-3.2 3.2-5 6.5-5s5.9 1.8 6.5 5" stroke={c} stroke-width="2.2" stroke-linecap="round" /></svg>
      {/if}
      <span style="font-size:10.5px; font-weight:700; color:{on ? 'var(--brand)' : '#A6ACB3'};">{$t(`nav.${k}`)}</span>
    </button>
  {/each}
</div>
