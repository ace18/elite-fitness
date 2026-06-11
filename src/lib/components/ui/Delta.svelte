<script>
  // Delta — colored ▲/▼ change indicator. invert=true means "down is good".
  let { v, unit = '', invert = false } = $props();

  let pos = $derived(v > 0);
  let good = $derived(invert ? !pos : pos);
  let col = $derived(v === 0 ? 'var(--ink3)' : good ? 'var(--up)' : 'var(--down)');
  let arrow = $derived(v === 0 ? '' : pos ? '▲' : '▼');
</script>

<span style="color:{col}; font-weight:800; font-size:12.5px;">{arrow} {v > 0 ? '+' : ''}{v}{unit}</span>
