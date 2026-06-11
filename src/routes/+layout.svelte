<script>
  import '../app.css';
  import Stage from '$lib/components/device/Stage.svelte';
  import IOSDevice from '$lib/components/device/IOSDevice.svelte';
  import { page } from '$app/stores';

  let { children } = $props();

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
