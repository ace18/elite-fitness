<script>
  // Toast — floating pill near the top of the device.
  let { show = false, children } = $props();
</script>

{#if show}
  <div style="position:absolute; left:0; right:0; top:64px; display:flex; justify-content:center; z-index:80; pointer-events:none;">
    <div class="float-up" style="background:var(--ink); color:#fff; padding:10px 16px; border-radius:999px;
      font-size:13.5px; font-weight:700; box-shadow:var(--sh-lg); display:flex; align-items:center; gap:8px;">
      {@render children?.()}
    </div>
  </div>
{/if}
