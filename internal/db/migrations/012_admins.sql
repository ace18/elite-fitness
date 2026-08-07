-- Il pannello di gestione ha un solo cliente: chi possiede l'installazione.
-- Tutti gli utenti registrati sono i suoi atleti, quindi non serve nessuna
-- tabella di collegamento allenatore↔atleta né un concetto di tenant: venderlo
-- a un altro allenatore vuol dire una nuova installazione col suo database.
--
-- Gli amministratori stanno in una tabella loro e non in `users` con un flag.
-- Non è una questione di stile: `users` si popola da sola: FindOrCreate crea
-- l'account al primo magic link riuscito e OAuth fa lo stesso. Un permesso
-- amministrativo appeso a una riga che nasce da sola è un permesso che prima o
-- poi qualcuno si ritrova per sbaglio. Qui invece l'unico modo di esistere è
-- che qualcuno ti abbia inserito.
CREATE TABLE IF NOT EXISTS admins (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  -- Chi ha creato questo amministratore. Il primo non ce l'ha: nasce
  -- dall'avvio del server (ADMIN_BOOTSTRAP_EMAIL) quando la tabella è vuota.
  created_by UUID REFERENCES admins(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_login_at TIMESTAMPTZ,
  -- Disattivazione invece di cancellazione: la catena created_by resta
  -- leggibile, e un accesso tolto per sbaglio si ripristina senza reinserire
  -- nulla. Chi è disattivato non riceve link e non supera il controllo di
  -- sessione, che viene rifatto a ogni richiesta.
  disabled_at TIMESTAMPTZ
);

-- I token di accesso al pannello puntano a un amministratore che deve già
-- esistere (admin_id, chiave esterna), non a un indirizzo email come
-- magic_link_tokens.
--
-- È la differenza che rende impossibile la registrazione. Nel flusso degli
-- atleti il token si emette per un'email qualsiasi e VerifyToken chiama
-- FindOrCreate: digitare un indirizzo nel form È l'iscrizione. Qui non esiste
-- nessun percorso che produca una riga in `admins` partendo da un form
-- pubblico, e non perché un handler si ricordi di controllare — perché non c'è
-- niente a cui agganciare il token.
CREATE TABLE IF NOT EXISTS admin_login_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
  token TEXT UNIQUE NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- La potatura periodica cancella per expires_at (vedi service/prune.go).
CREATE INDEX IF NOT EXISTS admin_login_tokens_expires_idx
  ON admin_login_tokens (expires_at);
