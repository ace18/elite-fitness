-- exercises.name non ha mai avuto un vincolo UNIQUE, quindi ogni
-- "ON CONFLICT DO NOTHING" su questa tabella non è mai scattato:
--   * 002_seed.sql re-inseriva l'intera libreria a ogni boot;
--   * FindOrCreateExercise inseriva sempre, senza mai trovare nulla, quindi
--     ogni piano generato dall'AI duplicava i suoi esercizi.
-- Qui si accorpano i duplicati esistenti e si aggiunge il vincolo mancante,
-- che è ciò che fa finalmente funzionare entrambi gli ON CONFLICT.
--
-- Riga canonica = id più basso per ciascun nome. DISTINCT ON invece di
-- MIN(id): min(uuid) esiste solo da Postgres 14. Ogni statement ricalcola la
-- stessa mappa in modo deterministico, così il file resta corretto sia
-- eseguito tutto in una transazione (runMigrations) sia statement per
-- statement (psql). Niente BEGIN/COMMIT: le transazioni le gestisce il runner.

-- 1. ripuntare le FK PRIMA della DELETE: entrambe le colonne sono
--    NOT NULL REFERENCES exercises(id) senza ON DELETE, quindi cancellare
--    per prime le righe duplicate fallirebbe con foreign key violation.
UPDATE program_workout_exercises pwe
SET exercise_id = c.canonical_id
FROM exercises e
JOIN (
  SELECT DISTINCT ON (name) name, id AS canonical_id
  FROM exercises
  ORDER BY name, id
) c ON c.name = e.name
WHERE pwe.exercise_id = e.id
  AND e.id <> c.canonical_id;

UPDATE set_logs sl
SET exercise_id = c.canonical_id
FROM exercises e
JOIN (
  SELECT DISTINCT ON (name) name, id AS canonical_id
  FROM exercises
  ORDER BY name, id
) c ON c.name = e.name
WHERE sl.exercise_id = e.id
  AND e.id <> c.canonical_id;

-- 2. ora nessuno referenzia più i duplicati
DELETE FROM exercises e
USING (
  SELECT DISTINCT ON (name) name, id AS canonical_id
  FROM exercises
  ORDER BY name, id
) c
WHERE c.name = e.name
  AND e.id <> c.canonical_id;

-- 3. il vincolo mancante
CREATE UNIQUE INDEX IF NOT EXISTS exercises_name_key ON exercises (name);
