<script>
  // Loading — skeleton shimmer placeholder screen. Optional `footer` snippet.
  import Screen from './Screen.svelte';
  let { footer } = $props();
  const heights = [80, 150, 220, 120];
</script>

<Screen {footer} footerFlush>
  <div style="padding:20px; display:flex; flex-direction:column; gap:14px;">
    {#each heights as h}
      <div style="height:{h}px; border-radius:20px;
        background:linear-gradient(100deg,#eef1f3 30%,#f6f8f9 50%,#eef1f3 70%);
        background-size:200% 100%; animation:shimmer 1.2s infinite linear;"></div>
    {/each}
  </div>
</Screen>
