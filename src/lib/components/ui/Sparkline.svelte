<script>
  // Sparkline — line + soft area fill.
  let {
    data,
    w = 120,
    h = 40,
    color = 'var(--brand)',
    fill = true,
    dot = true,
    sw = 2.4
  } = $props();

  let min = $derived(Math.min(...data));
  let max = $derived(Math.max(...data));
  let rng = $derived(max - min || 1);
  let pts = $derived(
    data.map((v, i) => [(i / (data.length - 1)) * w, h - 4 - ((v - min) / rng) * (h - 8)])
  );
  let line = $derived(pts.map((p) => `${p[0].toFixed(1)},${p[1].toFixed(1)}`).join(' '));
  let area = $derived(`0,${h} ` + line + ` ${w},${h}`);
  let last = $derived(pts[pts.length - 1]);
  // unique gradient id per instance
  let gid = 'g' + Math.random().toString(36).slice(2, 7);
</script>

<svg width={w} height={h} viewBox="0 0 {w} {h}" style="display:block; overflow:visible;">
  <defs>
    <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color={color} stop-opacity="0.22" />
      <stop offset="100%" stop-color={color} stop-opacity="0" />
    </linearGradient>
  </defs>
  {#if fill}
    <polygon points={area} fill="url(#{gid})" />
  {/if}
  <polyline points={line} fill="none" stroke={color} stroke-width={sw} stroke-linecap="round" stroke-linejoin="round" />
  {#if dot}
    <circle cx={last[0]} cy={last[1]} r="3.4" fill="#fff" stroke={color} stroke-width="2.4" />
  {/if}
</svg>
