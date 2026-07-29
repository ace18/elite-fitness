<script>
  // Logo — ELITE wordmark.
  let { size = 28, mono = false } = $props();
  let c = $derived(mono ? '#fff' : 'var(--brand)');
</script>

<span style="display:inline-flex; align-items:center; gap:9px;">
  <span style="width:{size}px; height:{size}px; border-radius:{size * 0.3}px; background:{c};
    display:inline-flex; align-items:center; justify-content:center;
    box-shadow:{mono ? 'none' : '0 4px 12px rgba(15,181,174,.35)'};">
    <span style="display:flex; gap:2.5px; align-items:flex-end;">
      <i style="width:3px; height:{size * 0.30}px; background:#fff; border-radius:2px; display:block;"></i>
      <i style="width:3px; height:{size * 0.46}px; background:#fff; border-radius:2px; display:block;"></i>
      <i style="width:3px; height:{size * 0.22}px; background:#fff; border-radius:2px; display:block;"></i>
    </span>
  </span>
  <span class="t-title" style="font-size:{size * 0.72}px; letter-spacing:.02em; color:{mono ? '#fff' : 'var(--ink)'};">ELITE</span>
</span>
