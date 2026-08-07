-- Periodizzazione nelle template: la 004 definiva solo la settimana 1 e la
-- copiava con `week_number = 1` fisso, quindi un programma di 12 settimane era
-- la stessa settimana dodici volte. Un 8/12 settimane vero cambia schema di
-- serie e ripetizioni fra un blocco di volume e uno di intensità.
--
-- Non serve definire tutte le settimane: GetWorkoutsForWeek risolve con
-- MAX(week_number) <= richiesta, quindi una settimana definita vale finché non
-- ne arriva un'altra. Definendo la 1, la 5 e la 9 si coprono dodici settimane
-- con tre blocchi.
--
-- Il default 1 tiene valide le template esistenti: restano a blocco unico e si
-- comportano esattamente come prima.
ALTER TABLE template_workouts
  ADD COLUMN IF NOT EXISTS week_number INT NOT NULL DEFAULT 1;

-- Il vincolo della 004 era UNIQUE (template_id, day_of_week): impedisce allo
-- stesso giorno di comparire in due blocchi, che è esattamente ciò che serve
-- adesso. Si sostituisce includendo la settimana.
--
-- Il nome è quello generato da Postgres per lo UNIQUE inline della 004; IF
-- EXISTS copre il caso in cui la tabella sia stata creata altrove con un nome
-- diverso.
ALTER TABLE template_workouts
  DROP CONSTRAINT IF EXISTS template_workouts_template_id_day_of_week_key;

ALTER TABLE template_workouts
  ADD CONSTRAINT template_workouts_template_id_week_day_key
  UNIQUE (template_id, week_number, day_of_week);

-- order_in_week è ordinale dentro la settimana, non dentro la template: due
-- blocchi ripartono entrambi da 0. Nessun dato da correggere, le template
-- esistenti stanno tutte nel blocco 1.
