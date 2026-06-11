<script module>
  // shared helper — RPE → semantic color
  export function rpeColor(n) {
    return n <= 6 ? 'var(--up)' : n <= 8 ? 'var(--brand)' : n <= 9 ? 'var(--amber)' : 'var(--down)';
  }
</script>

<script>
  // RPEScale — tap a number from `from`..`to`. Active pill grows + colorizes.
  let { value, onChange, from = 6, to = 10 } = $props();

  let nums = $derived(Array.from({ length: to - from + 1 }, (_, i) => from + i));
</script>

<div style="display:flex; gap:7px;">
  {#each nums as n (n)}
    {@const on = n === value}
    {@const col = rpeColor(n)}
    <button
      onclick={() => onChange(n)}
      style="flex:1; height:{on ? 56 : 48}px; border-radius:14px; cursor:pointer; border:none;
        background:{on ? col : '#fff'}; color:{on ? '#fff' : 'var(--ink2)'};
        box-shadow:{on ? '0 6px 16px rgba(0,0,0,.12)' : 'inset 0 0 0 1.5px var(--line)'};
        font-weight:800; font-size:{on ? 19 : 17}px; transition:.18s cubic-bezier(.2,.9,.3,1);
        align-self:center; font-variant-numeric:tabular-nums; -webkit-tap-highlight-color:transparent;">{n}</button>
  {/each}
</div>
