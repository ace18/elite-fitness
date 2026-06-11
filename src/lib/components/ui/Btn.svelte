<script>
  // Btn — primary action button. variant: primary | dark | ghost | subtle | soft
  let {
    variant = 'primary',
    lg = false,
    block = false,
    disabled = false,
    onclick,
    class: cls = '',
    style = '',
    children
  } = $props();

  let className = $derived(
    `btn btn--${variant}${lg ? ' btn--lg' : ''}${block ? ' btn--block' : ''}${cls ? ' ' + cls : ''}`
  );
</script>

<button class={className} {style} {disabled} {onclick}>
  {@render children?.()}
</button>
