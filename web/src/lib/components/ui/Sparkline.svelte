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

  // The API returns null (nothing logged yet) or a 1-point series for a brand
  // new user; both used to crash here. Render nothing until there are 2 points
  // to draw a line between.
  let series = $derived(Array.isArray(data) ? data.filter((v) => typeof v === 'number') : []);
  let enough = $derived(series.length >= 2);

  let min = $derived(enough ? Math.min(...series) : 0);
  let max = $derived(enough ? Math.max(...series) : 0);
  let rng = $derived(max - min || 1);
  let pts = $derived(
    enough
      ? series.map((v, i) => [(i / (series.length - 1)) * w, h - 4 - ((v - min) / rng) * (h - 8)])
      : []
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
  {#if enough}
    {#if fill}
      <polygon points={area} fill="url(#{gid})" />
    {/if}
    <polyline points={line} fill="none" stroke={color} stroke-width={sw} stroke-linecap="round" stroke-linejoin="round" />
    {#if dot}
      <circle cx={last[0]} cy={last[1]} r="3.4" fill="#fff" stroke={color} stroke-width="2.4" />
    {/if}
  {/if}
</svg>
