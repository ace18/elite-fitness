<script>
  // IOSDevice — phone bezel with dynamic island, status bar + home indicator.
  import IOSStatusBar from './IOSStatusBar.svelte';

  let { width = 402, height = 874, dark = false, children } = $props();
</script>

<div style="width:{width}px; height:{height}px; border-radius:48px; overflow:hidden;
  position:relative; background:{dark ? '#000' : '#F2F2F7'};
  box-shadow:0 40px 80px rgba(0,0,0,0.18), 0 0 0 1px rgba(0,0,0,0.12);
  font-family:-apple-system, system-ui, sans-serif; -webkit-font-smoothing:antialiased;">
  <!-- dynamic island -->
  <div style="position:absolute; top:11px; left:50%; transform:translateX(-50%);
    width:126px; height:37px; border-radius:24px; background:#000; z-index:50;"></div>
  <!-- status bar (absolute) -->
  <div style="position:absolute; top:0; left:0; right:0; z-index:10;">
    <IOSStatusBar {dark} />
  </div>
  <!-- content -->
  <div style="height:100%; display:flex; flex-direction:column;">
    <div style="flex:1; overflow:auto;">
      {@render children?.()}
    </div>
  </div>
  <!-- home indicator -->
  <div style="position:absolute; bottom:0; left:0; right:0; z-index:60; height:34px;
    display:flex; justify-content:center; align-items:flex-end; padding-bottom:8px; pointer-events:none;">
    <div style="width:139px; height:5px; border-radius:100px; background:{dark ? 'rgba(255,255,255,0.7)' : 'rgba(0,0,0,0.25)'};"></div>
  </div>
</div>
