<script>
  // Avatar — initials circle.
  //
  // Il fallback per chi non ha un nome sta in displayName() ($lib/user.js), non
  // qui: le iniziali devono essere quelle del nome mostrato accanto all'avatar,
  // e con due implementazioni separate prima o poi non lo sono più.
  //
  // Nessun valore di default per `name`: il default di un prop scatta solo su
  // undefined, mai su stringa vuota, e da API.user() arriva proprio '' finché
  // il profilo non è caricato — quindi non servirebbe comunque a niente.
  let { name = '', size = 40 } = $props();

  let init = $derived(
    (name ?? '')
      .split(/\s+/)
      .filter(Boolean)
      .map((w) => w[0])
      .slice(0, 2)
      .join('')
      .toUpperCase()
  );
</script>

<div style="width:{size}px; height:{size}px; border-radius:50%;
  background:linear-gradient(135deg,var(--brand),var(--brand-press)); color:#fff;
  display:flex; align-items:center; justify-content:center; font-weight:800; font-size:{size * 0.38}px;">
  {init}
</div>
