-- Idempotenza del salvataggio sessione.
--
-- POST /api/sessions faceva sempre INSERT: un ritentativo — risposta ambigua,
-- app chiusa a metà richiesta, coda offline rispedita — registrava
-- l'allenamento due volte. E una sessione doppia non è solo una riga in più:
-- conta anche verso il completamento dell'ultima settimana, quindi può chiudere
-- un programma prima del dovuto.
--
-- Il client genera un UUID quando l'allenamento finisce e lo riusa a ogni
-- tentativo, così il server riconosce il duplicato.
ALTER TABLE session_logs ADD COLUMN IF NOT EXISTS client_session_id TEXT;

-- Indice non parziale di proposito: in Postgres due NULL non sono uguali, così
-- le righe già esistenti e i client che non mandano l'id restano tutte valide e
-- ON CONFLICT non ha bisogno di ripetere un predicato.
CREATE UNIQUE INDEX IF NOT EXISTS session_logs_user_client_session_id_key
  ON session_logs (user_id, client_session_id);
