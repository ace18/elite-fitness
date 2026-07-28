-- Un programma smette di essere attivo per due motivi molto diversi: o è
-- stato portato a termine, o è stato abbandonato scegliendone un altro.
-- Entrambi mettevano solo is_active = FALSE, quindi erano indistinguibili.
-- completed_at è valorizzato solo nel primo caso.
ALTER TABLE user_programs ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;
