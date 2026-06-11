<script>
  // Screen — scrollable body + optional pinned footer, inside the bezel.
  let {
    footer,
    footerFlush = false,
    bg = 'var(--bg)',
    padTop = 54,
    children
  } = $props();

  let footerBg = $derived(bg === 'var(--bg)' ? 'var(--bg)' : bg);
</script>

<div class="app" style="height:100%; display:flex; flex-direction:column; background:{bg};">
  <div class="noscroll" style="flex:1; overflow-y:auto; overflow-x:hidden;">
    <div style="padding-top:{padTop}px;">
      {@render children?.()}
    </div>
  </div>

  {#if footer}
    {#if footerFlush}
      {@render footer()}
    {:else}
      <div style="padding:10px 20px calc(20px + 18px); background:linear-gradient(180deg, transparent, {footerBg} 28%);">
        {@render footer()}
      </div>
    {/if}
  {/if}
</div>
