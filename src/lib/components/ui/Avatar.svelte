<script>
  // Avatar — initials circle.
  let { name = 'Alex', size = 40 } = $props();
  let init = $derived(
    name.split(' ').map((s) => s[0]).slice(0, 2).join('')
  );
</script>

<div style="width:{size}px; height:{size}px; border-radius:50%;
  background:linear-gradient(135deg,var(--brand),var(--brand-press)); color:#fff;
  display:flex; align-items:center; justify-content:center; font-weight:800; font-size:{size * 0.38}px;">
  {init}
</div>
