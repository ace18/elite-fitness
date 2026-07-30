<script>
  // MiniStat — compact metric card used on Home / You.
  import Delta from './Delta.svelte';
  // I numeri arrivano grezzi dall'API: la virgola decimale e il separatore
  // delle migliaia dipendono dalla lingua. Formattare qui li sistema in ogni
  // schermata che usa questa card.
  import { num } from '$lib/format.js';

  let { label, value, unit, delta = undefined, invert = false } = $props();
</script>

<div class="card" style="flex:1; padding:13px 14px;">
  <div class="t-label" style="font-size:10px;">{label}</div>
  <div style="display:flex; align-items:baseline; gap:3px; margin-top:5px;">
    <span class="t-num" style="font-size:21px;">{$num(value)}</span>
    {#if unit}
      <span class="t-sub" style="font-size:12px; font-weight:700;">{unit}</span>
    {/if}
  </div>
  {#if delta !== undefined}
    <div style="margin-top:3px;"><Delta v={delta} {invert} /></div>
  {/if}
</div>
