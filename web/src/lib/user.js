// displayName — come chiamare l'utente quando non ha un nome.
//
// Il nome è opzionale: il magic link non lo chiede mai, e i provider OAuth non
// lo restituiscono in modo affidabile. L'email invece è l'identità di accesso,
// quindi c'è sempre — ma mostrarla intera in un saluto ("Buongiorno,
// mario.rossi@gmail.com 👋") legge male e va a capo su un telefono. La parte
// prima della @ è quasi sempre il nome, o qualcosa di abbastanza vicino.
//
// Unica fonte per il fallback: prima era ripetuto come 'Alex' in ogni schermata
// — il nome del mockup — e ogni copia poteva divergere dalle altre.
export function displayName(user) {
  const name = (user?.name ?? '').trim();
  if (name) return name;
  return (user?.email ?? '').trim().split('@')[0];
}
