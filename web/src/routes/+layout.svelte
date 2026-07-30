<script>
  import '../app.css';
  import { onMount } from 'svelte';
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

<!--
  Niente più cornice di iPhone finta: l'app È l'iPhone. La scocca con isola
  dinamica, barra di stato alle 9:41 e indicatore home serviva a mostrare il
  prototipo dentro un browser desktop; su un telefono vero raddoppiava la barra
  di stato e rimpiccioliva tutto dentro una lettera boxata.

  Su schermo largo resta una colonna stretta invece di allargarsi a tutta
  pagina: le schermate sono disegnate per una larghezza da telefono.
-->
<div class="app-backdrop">
  <div class="app-shell">
    {#key routeKey}
      <div style="height:100%; animation:fadeIn .3s ease;">
        {@render children?.()}
      </div>
    {/key}
  </div>
</div>
