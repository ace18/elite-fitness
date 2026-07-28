-- I premade plan (plan_templates) avevano solo metadati: scegliendone uno si
-- creava un user_programs vuoto, senza workout, quindi /api/workout/today
-- rispondeva 404 e lo schedule era tutto "Rest". Qui si aggiunge il contenuto
-- vero e proprio dei 5 programmi.
--
-- Struttura: template_workouts (i giorni) + template_workout_exercises (gli
-- esercizi di ogni giorno). SetProgram copia questi nella tabella del
-- programma dell'utente. day_of_week: 0=lunedì ... 6=domenica.
--
-- Come per i piani generati dall'AI, si definisce solo la settimana 1: niente
-- periodizzazione per ora.

CREATE TABLE IF NOT EXISTS template_workouts (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id   TEXT NOT NULL REFERENCES plan_templates(id) ON DELETE CASCADE,
  name          TEXT NOT NULL,
  focus         TEXT NOT NULL,
  day_of_week   INT  NOT NULL,
  order_in_week INT  NOT NULL,
  UNIQUE (template_id, day_of_week)
);

CREATE TABLE IF NOT EXISTS template_workout_exercises (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_workout_id UUID NOT NULL REFERENCES template_workouts(id) ON DELETE CASCADE,
  exercise_id         UUID NOT NULL REFERENCES exercises(id),
  sets                INT  NOT NULL,
  target_reps         INT  NOT NULL,
  rest_seconds        INT  NOT NULL,
  order_index         INT  NOT NULL,
  UNIQUE (template_workout_id, order_index)
);

-- ---------------------------------------------------------------- i giorni
INSERT INTO template_workouts (template_id, name, focus, day_of_week, order_in_week) VALUES
  -- Push / Pull / Legs — 6 giorni, ipertrofia
  ('ppl-hyp',     'Push Day A',      'Chest · Shoulders · Triceps',  0, 0),
  ('ppl-hyp',     'Pull Day A',      'Back · Biceps',                1, 1),
  ('ppl-hyp',     'Leg Day A',       'Quads · Hamstrings',           2, 2),
  ('ppl-hyp',     'Push Day B',      'Shoulders · Chest · Triceps',  3, 3),
  ('ppl-hyp',     'Pull Day B',      'Back · Biceps',                4, 4),
  ('ppl-hyp',     'Leg Day B',       'Glutes · Hamstrings',          5, 5),

  -- 5×5 Strength — 3 giorni, A/B alternati
  ('str-5x5',     'Strength A',      'Squat · Bench · Row',          0, 0),
  ('str-5x5',     'Strength B',      'Squat · Press · Deadlift',     2, 1),
  ('str-5x5',     'Strength A',      'Squat · Bench · Row',          4, 2),

  -- Upper / Lower — 4 giorni
  ('upper-lower', 'Upper Power',     'Strength focus',               0, 0),
  ('upper-lower', 'Lower Power',     'Quads · Posterior chain',      1, 1),
  ('upper-lower', 'Upper Hypertrophy', 'Volume · Pump',              3, 2),
  ('upper-lower', 'Lower Hypertrophy', 'Glutes · Hamstrings',        4, 3),

  -- Lean & Conditioned — 4 giorni, rest brevi
  ('lean-cond',   'Full Body Circuit A', 'Compounds · short rest',   0, 0),
  ('lean-cond',   'Upper Conditioning',  'Upper body · high reps',   1, 1),
  ('lean-cond',   'Lower Conditioning',  'Legs · high reps',         3, 2),
  ('lean-cond',   'Full Body Circuit B', 'Compounds · short rest',   4, 3),

  -- Full-Body Foundations — 3 giorni, principianti
  ('full-body',   'Full Body A',     'Squat · Bench · Row',          0, 0),
  ('full-body',   'Full Body B',     'Hinge · Pull · Press',         2, 1),
  ('full-body',   'Full Body C',     'Deadlift · Press · Row',       4, 2),

  -- Classic Bro Split — 5 giorni, un gruppo muscolare al giorno
  ('bro-split',   'Chest Day',       'Chest · Pressing volume',      0, 0),
  ('bro-split',   'Back Day',        'Back · Lats · Traps',          1, 1),
  ('bro-split',   'Shoulder Day',    'Delts · Rear delts',           2, 2),
  ('bro-split',   'Arm Day',         'Biceps · Triceps',             3, 3),
  ('bro-split',   'Leg Day',         'Quads · Hamstrings · Calves',  4, 4),

  -- Powerbuilding — 4 giorni, un big lift pesante + accessori
  ('power-build', 'Squat Focus',     'Heavy squat · Quads',          0, 0),
  ('power-build', 'Bench Focus',     'Heavy bench · Chest · Triceps',1, 1),
  ('power-build', 'Deadlift Focus',  'Heavy pull · Back',            3, 2),
  ('power-build', 'Overhead Focus',  'Heavy press · Delts',          4, 3),

  -- Glutes & Lower Body — 3 giorni, tutto lower
  ('glute-lower', 'Glute Focus',     'Hips · Hamstrings',            0, 0),
  ('glute-lower', 'Quad Focus',      'Quads · Volume',               2, 1),
  ('glute-lower', 'Posterior Chain', 'Deadlift · Glutes',            4, 2),

  -- Minimalist 2-Day — 2 giorni full body
  ('min-2day',    'Full Body A',     'Squat · Bench · Row',          0, 0),
  ('min-2day',    'Full Body B',     'Deadlift · Pull-Up · Press',   3, 1),

  -- Athletic Performance — 4 giorni, potenza e condizionamento
  ('ath-perform', 'Lower Power',     'Jumps · Squat · Hinge',        0, 0),
  ('ath-perform', 'Upper Power',     'Explosive pressing · Pulling', 1, 1),
  ('ath-perform', 'Conditioning',    'Circuits · High reps',         3, 2),
  ('ath-perform', 'Full Body Strength','Deadlift · Compounds',       4, 3)
ON CONFLICT (template_id, day_of_week) DO NOTHING;

-- ------------------------------------------------------------ gli esercizi
-- Join per nome: exercises.name è UNIQUE dalla 003, quindi è 1:1.
INSERT INTO template_workout_exercises
  (template_workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
SELECT tw.id, e.id, v.sets, v.reps, v.rest, v.ord
FROM (VALUES
  -- ============================== ppl-hyp ==============================
  ('ppl-hyp', 0, 'Barbell Bench Press',     4,  8, 120, 0),
  ('ppl-hyp', 0, 'Incline Dumbbell Press',  3, 10,  90, 1),
  ('ppl-hyp', 0, 'Seated Shoulder Press',   3, 10,  90, 2),
  ('ppl-hyp', 0, 'Cable Fly',               3, 12,  60, 3),
  ('ppl-hyp', 0, 'Triceps Pushdown',        3, 12,  60, 4),
  ('ppl-hyp', 0, 'Lateral Raise',           3, 15,  45, 5),

  ('ppl-hyp', 1, 'Barbell Row',             4,  8, 120, 0),
  ('ppl-hyp', 1, 'Pull-Up',                 3,  8,  90, 1),
  ('ppl-hyp', 1, 'Lat Pulldown',            3, 10,  90, 2),
  ('ppl-hyp', 1, 'Cable Row',               3, 12,  60, 3),
  ('ppl-hyp', 1, 'Barbell Curl',            3, 12,  60, 4),
  ('ppl-hyp', 1, 'Face Pull',               3, 15,  45, 5),

  ('ppl-hyp', 2, 'Back Squat',              4,  8, 150, 0),
  ('ppl-hyp', 2, 'Romanian Deadlift',       3, 10, 120, 1),
  ('ppl-hyp', 2, 'Leg Press',               3, 12,  90, 2),
  ('ppl-hyp', 2, 'Leg Curl',                3, 12,  60, 3),
  ('ppl-hyp', 2, 'Calf Raise',              4, 15,  45, 4),

  ('ppl-hyp', 3, 'Overhead Press',          4,  8, 120, 0),
  ('ppl-hyp', 3, 'Dip',                     3, 10,  90, 1),
  ('ppl-hyp', 3, 'Incline Dumbbell Press',  3, 10,  90, 2),
  ('ppl-hyp', 3, 'Dumbbell Shoulder Press', 3, 12,  60, 3),
  ('ppl-hyp', 3, 'Skull Crushers',          3, 12,  60, 4),
  ('ppl-hyp', 3, 'Lateral Raise',           3, 15,  45, 5),

  ('ppl-hyp', 4, 'Deadlift',                3,  5, 180, 0),
  ('ppl-hyp', 4, 'Chin-Up',                 3,  8,  90, 1),
  ('ppl-hyp', 4, 'Cable Row',               3, 10,  90, 2),
  ('ppl-hyp', 4, 'Lat Pulldown',            3, 12,  60, 3),
  ('ppl-hyp', 4, 'Hammer Curl',             3, 12,  60, 4),
  ('ppl-hyp', 4, 'Face Pull',               3, 15,  45, 5),

  ('ppl-hyp', 5, 'Hip Thrust',              4, 10, 120, 0),
  ('ppl-hyp', 5, 'Back Squat',              3, 10, 120, 1),
  ('ppl-hyp', 5, 'Leg Curl',                3, 12,  60, 2),
  ('ppl-hyp', 5, 'Leg Press',               3, 15,  60, 3),
  ('ppl-hyp', 5, 'Calf Raise',              4, 15,  45, 4),

  -- ============================== str-5x5 ==============================
  ('str-5x5', 0, 'Back Squat',              5,  5, 180, 0),
  ('str-5x5', 0, 'Barbell Bench Press',     5,  5, 180, 1),
  ('str-5x5', 0, 'Barbell Row',             5,  5, 180, 2),

  ('str-5x5', 2, 'Back Squat',              5,  5, 180, 0),
  ('str-5x5', 2, 'Overhead Press',          5,  5, 180, 1),
  ('str-5x5', 2, 'Deadlift',                1,  5, 180, 2),

  ('str-5x5', 4, 'Back Squat',              5,  5, 180, 0),
  ('str-5x5', 4, 'Barbell Bench Press',     5,  5, 180, 1),
  ('str-5x5', 4, 'Barbell Row',             5,  5, 180, 2),

  -- ============================ upper-lower ============================
  ('upper-lower', 0, 'Barbell Bench Press',     4,  6, 150, 0),
  ('upper-lower', 0, 'Barbell Row',             4,  6, 150, 1),
  ('upper-lower', 0, 'Overhead Press',          3,  8, 120, 2),
  ('upper-lower', 0, 'Lat Pulldown',            3, 10,  90, 3),
  ('upper-lower', 0, 'Barbell Curl',            3, 10,  60, 4),
  ('upper-lower', 0, 'Triceps Pushdown',        3, 10,  60, 5),

  ('upper-lower', 1, 'Back Squat',              4,  6, 180, 0),
  ('upper-lower', 1, 'Romanian Deadlift',       3,  8, 120, 1),
  ('upper-lower', 1, 'Leg Press',               3, 10,  90, 2),
  ('upper-lower', 1, 'Leg Curl',                3, 12,  60, 3),
  ('upper-lower', 1, 'Calf Raise',              4, 15,  45, 4),

  ('upper-lower', 3, 'Incline Dumbbell Press',  4, 10,  90, 0),
  ('upper-lower', 3, 'Cable Row',               4, 10,  90, 1),
  ('upper-lower', 3, 'Dumbbell Shoulder Press', 3, 12,  60, 2),
  ('upper-lower', 3, 'Pull-Up',                 3,  8,  90, 3),
  ('upper-lower', 3, 'Hammer Curl',             3, 12,  60, 4),
  ('upper-lower', 3, 'Skull Crushers',          3, 12,  60, 5),

  ('upper-lower', 4, 'Deadlift',                4,  5, 180, 0),
  ('upper-lower', 4, 'Hip Thrust',              3, 10,  90, 1),
  ('upper-lower', 4, 'Leg Press',               3, 12,  90, 2),
  ('upper-lower', 4, 'Leg Curl',                3, 15,  60, 3),
  ('upper-lower', 4, 'Calf Raise',              4, 20,  45, 4),

  -- ============================= lean-cond =============================
  ('lean-cond', 0, 'Back Squat',              3, 12,  45, 0),
  ('lean-cond', 0, 'Barbell Bench Press',     3, 12,  45, 1),
  ('lean-cond', 0, 'Cable Row',               3, 12,  45, 2),
  ('lean-cond', 0, 'Overhead Press',          3, 12,  45, 3),
  ('lean-cond', 0, 'Calf Raise',              3, 20,  30, 4),

  ('lean-cond', 1, 'Incline Dumbbell Press',  3, 15,  40, 0),
  ('lean-cond', 1, 'Lat Pulldown',            3, 15,  40, 1),
  ('lean-cond', 1, 'Dumbbell Shoulder Press', 3, 15,  40, 2),
  ('lean-cond', 1, 'Triceps Pushdown',        3, 15,  30, 3),
  ('lean-cond', 1, 'Barbell Curl',            3, 15,  30, 4),

  ('lean-cond', 3, 'Leg Press',               3, 15,  45, 0),
  ('lean-cond', 3, 'Romanian Deadlift',       3, 12,  45, 1),
  ('lean-cond', 3, 'Hip Thrust',              3, 15,  40, 2),
  ('lean-cond', 3, 'Leg Curl',                3, 15,  30, 3),
  ('lean-cond', 3, 'Calf Raise',              3, 20,  30, 4),

  ('lean-cond', 4, 'Deadlift',                3, 10,  60, 0),
  ('lean-cond', 4, 'Dip',                     3, 12,  45, 1),
  ('lean-cond', 4, 'Pull-Up',                 3, 10,  45, 2),
  ('lean-cond', 4, 'Lateral Raise',           3, 15,  30, 3),
  ('lean-cond', 4, 'Face Pull',               3, 15,  30, 4),

  -- ============================= full-body =============================
  ('full-body', 0, 'Back Squat',              3,  8, 120, 0),
  ('full-body', 0, 'Barbell Bench Press',     3,  8, 120, 1),
  ('full-body', 0, 'Barbell Row',             3,  8,  90, 2),
  ('full-body', 0, 'Overhead Press',          2, 10,  90, 3),
  ('full-body', 0, 'Calf Raise',              2, 15,  45, 4),

  ('full-body', 2, 'Romanian Deadlift',       3,  8, 120, 0),
  ('full-body', 2, 'Lat Pulldown',            3, 10,  90, 1),
  ('full-body', 2, 'Incline Dumbbell Press',  3, 10,  90, 2),
  ('full-body', 2, 'Leg Press',               3, 12,  90, 3),
  ('full-body', 2, 'Barbell Curl',            2, 12,  60, 4),

  ('full-body', 4, 'Deadlift',                3,  5, 150, 0),
  ('full-body', 4, 'Dumbbell Shoulder Press', 3, 10,  90, 1),
  ('full-body', 4, 'Cable Row',               3, 10,  90, 2),
  ('full-body', 4, 'Leg Curl',                3, 12,  60, 3),
  ('full-body', 4, 'Triceps Pushdown',        2, 12,  60, 4),

  -- ============================= bro-split =============================
  ('bro-split', 0, 'Barbell Bench Press',     4,  8, 120, 0),
  ('bro-split', 0, 'Incline Dumbbell Press',  4, 10,  90, 1),
  ('bro-split', 0, 'Dip',                     3, 10,  90, 2),
  ('bro-split', 0, 'Cable Fly',               3, 12,  60, 3),
  ('bro-split', 0, 'Push-Up',                 2, 15,  45, 4),

  ('bro-split', 1, 'Deadlift',                4,  6, 180, 0),
  ('bro-split', 1, 'Barbell Row',             4,  8, 120, 1),
  ('bro-split', 1, 'Lat Pulldown',            3, 10,  90, 2),
  ('bro-split', 1, 'T-Bar Row',               3, 10,  90, 3),
  ('bro-split', 1, 'Cable Row',               3, 12,  60, 4),
  ('bro-split', 1, 'Face Pull',               3, 15,  45, 5),

  ('bro-split', 2, 'Overhead Press',          4,  8, 120, 0),
  ('bro-split', 2, 'Seated Shoulder Press',   3, 10,  90, 1),
  ('bro-split', 2, 'Dumbbell Shoulder Press', 3, 12,  60, 2),
  ('bro-split', 2, 'Lateral Raise',           4, 15,  45, 3),
  ('bro-split', 2, 'Face Pull',               3, 15,  45, 4),

  ('bro-split', 3, 'Close-Grip Bench Press',  4,  8, 120, 0),
  ('bro-split', 3, 'Barbell Curl',            4, 10,  90, 1),
  ('bro-split', 3, 'Skull Crushers',          3, 12,  60, 2),
  ('bro-split', 3, 'Preacher Curl',           3, 12,  60, 3),
  ('bro-split', 3, 'Triceps Pushdown',        3, 15,  45, 4),
  ('bro-split', 3, 'Hammer Curl',             3, 15,  45, 5),

  ('bro-split', 4, 'Back Squat',              4,  8, 150, 0),
  ('bro-split', 4, 'Romanian Deadlift',       4, 10, 120, 1),
  ('bro-split', 4, 'Leg Press',               3, 12,  90, 2),
  ('bro-split', 4, 'Walking Lunge',           3, 12,  60, 3),
  ('bro-split', 4, 'Leg Curl',                3, 12,  60, 4),
  ('bro-split', 4, 'Calf Raise',              4, 20,  45, 5),

  -- ============================ power-build ============================
  ('power-build', 0, 'Back Squat',              5,  3, 240, 0),
  ('power-build', 0, 'Front Squat',             3,  6, 180, 1),
  ('power-build', 0, 'Bulgarian Split Squat',   3, 10,  90, 2),
  ('power-build', 0, 'Leg Curl',                3, 12,  60, 3),
  ('power-build', 0, 'Hanging Leg Raise',       3, 12,  60, 4),

  ('power-build', 1, 'Barbell Bench Press',     5,  3, 240, 0),
  ('power-build', 1, 'Close-Grip Bench Press',  3,  6, 150, 1),
  ('power-build', 1, 'Incline Dumbbell Press',  3, 10,  90, 2),
  ('power-build', 1, 'Cable Fly',               3, 12,  60, 3),
  ('power-build', 1, 'Triceps Pushdown',        3, 12,  60, 4),

  ('power-build', 3, 'Deadlift',                5,  3, 240, 0),
  ('power-build', 3, 'Romanian Deadlift',       3,  8, 150, 1),
  ('power-build', 3, 'Barbell Row',             4,  8, 120, 2),
  ('power-build', 3, 'Lat Pulldown',            3, 10,  90, 3),
  ('power-build', 3, 'Cable Crunch',            3, 15,  60, 4),

  ('power-build', 4, 'Overhead Press',          5,  3, 240, 0),
  ('power-build', 4, 'Dumbbell Shoulder Press', 3,  8, 120, 1),
  ('power-build', 4, 'Pull-Up',                 4,  8, 120, 2),
  ('power-build', 4, 'Lateral Raise',           3, 15,  45, 3),
  ('power-build', 4, 'Barbell Curl',            3, 12,  60, 4),

  -- ============================ glute-lower ============================
  ('glute-lower', 0, 'Hip Thrust',              4, 12,  90, 0),
  ('glute-lower', 0, 'Romanian Deadlift',       4, 10, 120, 1),
  ('glute-lower', 0, 'Bulgarian Split Squat',   3, 12,  90, 2),
  ('glute-lower', 0, 'Leg Curl',                3, 15,  60, 3),
  ('glute-lower', 0, 'Calf Raise',              3, 20,  45, 4),

  ('glute-lower', 2, 'Back Squat',              4, 10, 120, 0),
  ('glute-lower', 2, 'Front Squat',             3,  8, 120, 1),
  ('glute-lower', 2, 'Leg Press',               4, 12,  90, 2),
  ('glute-lower', 2, 'Walking Lunge',           3, 12,  60, 3),
  ('glute-lower', 2, 'Calf Raise',              4, 20,  45, 4),

  ('glute-lower', 4, 'Deadlift',                4,  6, 180, 0),
  ('glute-lower', 4, 'Hip Thrust',              4, 12,  90, 1),
  ('glute-lower', 4, 'Leg Curl',                4, 12,  60, 2),
  ('glute-lower', 4, 'Kettlebell Swing',        3, 15,  60, 3),
  ('glute-lower', 4, 'Hanging Leg Raise',       3, 12,  60, 4),

  -- ============================= min-2day ==============================
  ('min-2day', 0, 'Back Squat',              4,  8, 120, 0),
  ('min-2day', 0, 'Barbell Bench Press',     4,  8, 120, 1),
  ('min-2day', 0, 'Barbell Row',             4, 10,  90, 2),
  ('min-2day', 0, 'Overhead Press',          3, 10,  90, 3),
  ('min-2day', 0, 'Hanging Leg Raise',       3, 12,  60, 4),

  ('min-2day', 3, 'Deadlift',                4,  6, 150, 0),
  ('min-2day', 3, 'Pull-Up',                 4,  8, 120, 1),
  ('min-2day', 3, 'Incline Dumbbell Press',  4, 10,  90, 2),
  ('min-2day', 3, 'Bulgarian Split Squat',   3, 10,  90, 3),
  ('min-2day', 3, 'Cable Crunch',            3, 15,  60, 4),

  -- ============================ ath-perform ============================
  ('ath-perform', 0, 'Box Jump',                5,  5, 120, 0),
  ('ath-perform', 0, 'Back Squat',              4,  5, 180, 1),
  ('ath-perform', 0, 'Romanian Deadlift',       3,  8, 120, 2),
  ('ath-perform', 0, 'Walking Lunge',           3, 12,  60, 3),
  ('ath-perform', 0, 'Hanging Leg Raise',       3, 15,  45, 4),

  ('ath-perform', 1, 'Barbell Bench Press',     4,  5, 180, 0),
  ('ath-perform', 1, 'Pull-Up',                 4,  6, 120, 1),
  ('ath-perform', 1, 'Overhead Press',          3,  8, 120, 2),
  ('ath-perform', 1, 'Barbell Row',             3, 10,  90, 3),
  ('ath-perform', 1, 'Face Pull',               3, 15,  45, 4),

  ('ath-perform', 3, 'Kettlebell Swing',        4, 20,  45, 0),
  ('ath-perform', 3, 'Box Jump',                4,  8,  60, 1),
  ('ath-perform', 3, 'Push-Up',                 4, 15,  45, 2),
  ('ath-perform', 3, 'Walking Lunge',           3, 20,  45, 3),
  ('ath-perform', 3, 'Cable Crunch',            3, 20,  40, 4),

  ('ath-perform', 4, 'Deadlift',                4,  5, 180, 0),
  ('ath-perform', 4, 'Front Squat',             3,  8, 150, 1),
  ('ath-perform', 4, 'Dip',                     3, 10,  90, 2),
  ('ath-perform', 4, 'Cable Row',               3, 12,  60, 3),
  ('ath-perform', 4, 'Kettlebell Swing',        3, 15,  45, 4)
) AS v(template_id, dow, ex_name, sets, reps, rest, ord)
JOIN template_workouts tw
  ON tw.template_id = v.template_id AND tw.day_of_week = v.dow
JOIN exercises e ON e.name = v.ex_name
ON CONFLICT (template_workout_id, order_index) DO NOTHING;
