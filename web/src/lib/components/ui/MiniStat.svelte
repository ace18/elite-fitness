<script>
  // MiniStat — compact metric card used on Home / You.
  import Delta from './Delta.svelte';

  let { label, value, unit, delta = undefined, invert = false } = $props();
</script>

<div class="card" style="flex:1; padding:13px 14px;">
  <div class="t-label" style="font-size:10px;">{label}</div>
  <div style="display:flex; align-items:baseline; gap:3px; margin-top:5px;">
    <span class="t-num" style="font-size:21px;">{value}</span>
    {#if unit}
      <span class="t-sub" style="font-size:12px; font-weight:700;">{unit}</span>
    {/if}
  </div>
  {#if delta !== undefined}
    <div style="margin-top:3px;"><Delta v={delta} {invert} /></div>
  {/if}
</div>
