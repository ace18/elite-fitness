<script>
  // Screen — scrollable body + optional pinned footer, inside the bezel.
  // padTop era 54px: lo spazio occupato dalla barra di stato finta, che stava
  // sopra il contenuto in posizione assoluta. Ora la barra di stato è quella
  // vera del telefono, quindi resta solo il respiro visivo — più l'incavo,
  // dove c'è.
  let {
    footer,
    footerFlush = false,
    bg = 'var(--bg)',
    padTop = 12,
    children
  } = $props();

  let footerBg = $derived(bg === 'var(--bg)' ? 'var(--bg)' : bg);
</script>

<div class="app" style="height:100%; display:flex; flex-direction:column; background:{bg};">
  <div class="noscroll" style="flex:1; overflow-y:auto; overflow-x:hidden;">
    <div style="padding-top:calc(env(safe-area-inset-top, 0px) + {padTop}px);">
      {@render children?.()}
    </div>
  </div>

  {#if footer}
    {#if footerFlush}
      {@render footer()}
    {:else}
      <div style="padding:10px 20px calc(20px + env(safe-area-inset-bottom, 0px)); background:linear-gradient(180deg, transparent, {footerBg} 28%);">
        {@render footer()}
      </div>
    {/if}
  {/if}
</div>
