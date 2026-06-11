<script>
  // Ring — circular progress indicator with centered slot content.
  let {
    pct = 0.7,
    size = 72,
    stroke = 8,
    color = 'var(--brand)',
    track = '#EAEDF0',
    children
  } = $props();

  let r = $derived((size - stroke) / 2);
  let c = $derived(2 * Math.PI * r);
</script>

<div style="position:relative; width:{size}px; height:{size}px;">
  <svg width={size} height={size} style="transform:rotate(-90deg);">
    <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={track} stroke-width={stroke} />
    <circle
      cx={size / 2}
      cy={size / 2}
      r={r}
      fill="none"
      stroke={color}
      stroke-width={stroke}
      stroke-linecap="round"
      stroke-dasharray={c}
      stroke-dashoffset={c * (1 - pct)}
      style="transition:stroke-dashoffset .8s cubic-bezier(.3,.9,.3,1);"
    />
  </svg>
  <div style="position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center;">
    {@render children?.()}
  </div>
</div>
