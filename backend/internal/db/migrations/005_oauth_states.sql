-- Stato temporaneo del flusso OAuth (state CSRF + PKCE code_verifier).
-- Sta a database e non in un cookie perché il callback di Apple arriva come
-- POST cross-site (response_mode=form_post): un cookie SameSite=Lax non
-- verrebbe inviato, e SameSite=None richiede Secure — quindi niente localhost.
CREATE TABLE IF NOT EXISTS oauth_states (
  state         TEXT PRIMARY KEY,
  provider      TEXT NOT NULL,
  code_verifier TEXT NOT NULL,
  expires_at    TIMESTAMPTZ NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Le righe si consumano con DELETE ... RETURNING; questo indice serve solo
-- alla potatura di quelle abbandonate (l'utente che non completa il consenso).
CREATE INDEX IF NOT EXISTS oauth_states_expires_at_idx ON oauth_states (expires_at);
