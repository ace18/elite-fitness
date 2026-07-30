<script>
  import '../app.css';
  import { onMount } from 'svelte';
  import Stage from '$lib/components/device/Stage.svelte';
  import IOSDevice from '$lib/components/device/IOSDevice.svelte';
  import { page } from '$app/stores';
  import { API } from '$lib/api.js';
  import { syncPending, refreshPendingSync } from '$lib/stores.js';

  let { children } = $props();

  // La coda va svuotata da qualche parte che vive per tutta l'app, non da una
  // singola schermata: un allenamento messo in coda ieri deve ripartire alla
  // prima apertura, non solo se si passa da Allena.
  onMount(() => {
    if (!API.isAuthed()) return;
    refreshPendingSync();
    syncPending().catch(() => {}); // offline: riproverà all'evento online
    const onOnline = () => syncPending().catch(() => {});
    addEventListener('online', onOnline);
    return () => removeEventListener('online', onOnline);
  });

  // re-trigger the fade when the route changes (mirrors the prototype's keyed fade)
  let routeKey = $derived($page.url.pathname);
</script>

<Stage>
  <IOSDevice>
    {#key routeKey}
      <div style="height:100%; animation:fadeIn .3s ease;">
        {@render children?.()}
      </div>
    {/key}
  </IOSDevice>
</Stage>
