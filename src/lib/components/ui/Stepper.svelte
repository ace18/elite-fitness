<script>
  // Stepper — big +/- numeric control.
  let { value, onChange, step = 2.5, min = 0, unit, big = false, decimals = 1 } = $props();

  let sz = $derived(big ? 56 : 46);

  function fmt(v) {
    return Number.isInteger(v) ? `${v}` : v.toFixed(decimals).replace(/\.0$/, '');
  }
  function bump(delta) {
    onChange(Math.max(min, +(value + delta).toFixed(2)));
  }
</script>

<div style="display:flex; align-items:center; justify-content:space-between; gap:14px;">
  <button
    class="step-btn"
    style="width:{sz}px; height:{sz}px; font-size:{big ? 30 : 24}px;"
    onclick={() => bump(-step)}>−</button>

  <div style="text-align:center; flex:1; line-height:1;">
    <span class="t-num" style="font-size:{big ? 52 : 38}px;">{fmt(value)}</span>
    {#if unit}
      <span class="t-sub" style="font-size:{big ? 18 : 15}px; font-weight:700; margin-left:5px;">{unit}</span>
    {/if}
  </div>

  <button
    class="step-btn"
    style="width:{sz}px; height:{sz}px; font-size:{big ? 30 : 24}px;"
    onclick={() => bump(step)}>+</button>
</div>

<style>
  .step-btn {
    border-radius: 18px;
    border: none;
    cursor: pointer;
    background: #f1f3f5;
    color: var(--ink);
    font-weight: 700;
    line-height: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: transform 0.1s, background 0.15s;
    -webkit-tap-highlight-color: transparent;
  }
  .step-btn:active {
    transform: scale(0.9);
  }
</style>
