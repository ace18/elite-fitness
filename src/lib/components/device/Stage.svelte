<script>
  // Stage — scales the fixed 402×874 device to fit any viewport.
  import { onMount } from 'svelte';

  let { w = 402, h = 874, children } = $props();
  let s = $state(1);

  onMount(() => {
    const fit = () => {
      const pad = 24;
      s = Math.min((window.innerWidth - pad) / w, (window.innerHeight - pad) / h, 1);
    };
    fit();
    window.addEventListener('resize', fit);
    return () => window.removeEventListener('resize', fit);
  });
</script>

<div style="position:fixed; inset:0; display:flex; align-items:center; justify-content:center;
  background:radial-gradient(120% 120% at 50% 0%, #f7f9fb 0%, #e9edf1 60%, #e2e7ec 100%);">
  <div style="width:{w}px; height:{h}px; transform:scale({s}); transform-origin:center;">
    {@render children?.()}
  </div>
</div>
