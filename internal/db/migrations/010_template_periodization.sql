-- Periodizza i dieci template della 004, che finora definivano un blocco solo
-- (settimana 1) e lo ripetevano per tutta la durata: un 12 settimane era la
-- stessa settimana dodici volte, con l'unico avanzamento nel carico suggerito
-- da suggestNext.
--
-- Qui si aggiungono i blocchi successivi. La 009 ha reso possibile definire lo
-- stesso giorno in più settimane, e GetWorkoutsForWeek risolve con
-- MAX(week_number) <= settimana richiesta: un blocco vale da dove è definito
-- fino al successivo. Quindi basta definire la settimana in cui lo schema
-- cambia, non tutte.
--
-- Struttura scelta, sulla durata di ogni template:
--   6 settimane  -> 2 blocchi (1, 4)
--   8 settimane  -> 2 blocchi (1, 5)
--   10-12 sett.  -> 3 blocchi (1, 5, 9)
--
-- Il verso della periodizzazione non è uguale per tutti: dove l'obiettivo è
-- forza o ipertrofia si va da più ripetizioni a meno, con recuperi più lunghi
-- (accumulo -> intensificazione). In lean-cond, dove l'obiettivo è il
-- dimagrimento, si va nella direzione opposta — più ripetizioni e recuperi più
-- corti — perché lì il progresso è la densità, non il carico.
--
-- La selezione degli esercizi resta la stessa fra i blocchi: cambiano serie,
-- ripetizioni e recupero. Il carico non è scritto da nessuna parte, lo decide
-- suggestNext dai set registrati.
--
-- La settimana 1 non si tocca: è già a posto dalla 004.

INSERT INTO template_workouts (template_id, name, focus, day_of_week, week_number, order_in_week) VALUES
  -- Push / Pull / Legs — 8 sett., blocco 2 di intensificazione
  ('ppl-hyp',     'Push Day A',      'Heavy pressing · Lower reps',   0, 5, 0),
  ('ppl-hyp',     'Pull Day A',      'Heavy rowing · Lower reps',     1, 5, 1),
  ('ppl-hyp',     'Leg Day A',       'Heavy squat · Lower reps',      2, 5, 2),
  ('ppl-hyp',     'Push Day B',      'Heavy overhead · Lower reps',   3, 5, 3),
  ('ppl-hyp',     'Pull Day B',      'Heavy pull · Lower reps',       4, 5, 4),
  ('ppl-hyp',     'Leg Day B',       'Heavy hinge · Lower reps',      5, 5, 5),

  -- 5×5 Strength — 12 sett., 5×5 -> 5×3 -> 3×3
  ('str-5x5',     'Strength A',      'Squat · Bench · Row · 5×3',     0, 5, 0),
  ('str-5x5',     'Strength B',      'Squat · Press · Deadlift · 5×3',2, 5, 1),
  ('str-5x5',     'Strength A',      'Squat · Bench · Row · 5×3',     4, 5, 2),

  ('str-5x5',     'Strength A',      'Squat · Bench · Row · 3×3',     0, 9, 0),
  ('str-5x5',     'Strength B',      'Squat · Press · Deadlift · 3×3',2, 9, 1),
  ('str-5x5',     'Strength A',      'Squat · Bench · Row · 3×3',     4, 9, 2),

  -- Upper / Lower — 8 sett., blocco 2 di intensificazione
  ('upper-lower', 'Upper Power',       'Heavy pressing · Pulling',    0, 5, 0),
  ('upper-lower', 'Lower Power',       'Heavy squat · Hinge',         1, 5, 1),
  ('upper-lower', 'Upper Hypertrophy', 'Dense volume · Lower reps',   3, 5, 2),
  ('upper-lower', 'Lower Hypertrophy', 'Heavy pull · Glutes',         4, 5, 3),

  -- Lean & Conditioned — 6 sett., blocco 2 di densità (verso opposto)
  ('lean-cond',   'Full Body Circuit A', 'Higher reps · Shorter rest',0, 4, 0),
  ('lean-cond',   'Upper Conditioning',  'Upper body · Density',      1, 4, 1),
  ('lean-cond',   'Lower Conditioning',  'Legs · Density',            3, 4, 2),
  ('lean-cond',   'Full Body Circuit B', 'Higher reps · Shorter rest',4, 4, 3),

  -- Full-Body Foundations — 8 sett., passo avanti misurato (principianti)
  ('full-body',   'Full Body A',     'Squat · Bench · Row · Heavier',  0, 5, 0),
  ('full-body',   'Full Body B',     'Hinge · Pull · Press · Heavier', 2, 5, 1),
  ('full-body',   'Full Body C',     'Deadlift · Press · Row · Heavier',4, 5, 2),

  -- Classic Bro Split — 8 sett., blocco 2 di intensificazione
  ('bro-split',   'Chest Day',       'Heavy pressing · Lower reps',   0, 5, 0),
  ('bro-split',   'Back Day',        'Heavy pulling · Lower reps',    1, 5, 1),
  ('bro-split',   'Shoulder Day',    'Heavy overhead · Lower reps',   2, 5, 2),
  ('bro-split',   'Arm Day',         'Heavy arms · Lower reps',       3, 5, 3),
  ('bro-split',   'Leg Day',         'Heavy squat · Lower reps',      4, 5, 4),

  -- Powerbuilding — 10 sett., 5×3 -> 6×2 con accessori su -> 4×2 in scarico
  ('power-build', 'Squat Focus',     'Squat 6×2 · Accessory volume',  0, 5, 0),
  ('power-build', 'Bench Focus',     'Bench 6×2 · Accessory volume',  1, 5, 1),
  ('power-build', 'Deadlift Focus',  'Deadlift 6×2 · Accessory volume',3, 5, 2),
  ('power-build', 'Overhead Focus',  'Press 6×2 · Accessory volume',  4, 5, 3),

  ('power-build', 'Squat Focus',     'Squat 4×2 · Peak week',         0, 9, 0),
  ('power-build', 'Bench Focus',     'Bench 4×2 · Peak week',         1, 9, 1),
  ('power-build', 'Deadlift Focus',  'Deadlift 4×2 · Peak week',      3, 9, 2),
  ('power-build', 'Overhead Focus',  'Press 4×2 · Peak week',         4, 9, 3),

  -- Glutes & Lower Body — 8 sett., blocco 2 di intensificazione
  ('glute-lower', 'Glute Focus',     'Heavy hips · Lower reps',       0, 5, 0),
  ('glute-lower', 'Quad Focus',      'Heavy squat · Lower reps',      2, 5, 1),
  ('glute-lower', 'Posterior Chain', 'Heavy deadlift · Glutes',       4, 5, 2),

  -- Minimalist 2-Day — 8 sett., blocco 2 di intensificazione
  ('min-2day',    'Full Body A',     'Squat · Bench · Row · 5×5',     0, 5, 0),
  ('min-2day',    'Full Body B',     'Deadlift · Pull-Up · Press · Heavier',3, 5, 1),

  -- Athletic Performance — 6 sett., blocco 2 tutto su velocità e forza
  ('ath-perform', 'Lower Power',     'Peak jumps · Heavy squat',      0, 4, 0),
  ('ath-perform', 'Upper Power',     'Explosive pressing · Heavy',    1, 4, 1),
  ('ath-perform', 'Conditioning',    'Circuits · Higher density',     3, 4, 2),
  ('ath-perform', 'Full Body Strength','Heavy deadlift · Compounds',  4, 4, 3)
ON CONFLICT (template_id, week_number, day_of_week) DO NOTHING;

-- ------------------------------------------------------------ gli esercizi
-- Stessa forma della 004, con in più la settimana nel join: senza, gli esercizi
-- di un blocco finirebbero anche in tutti gli altri.
INSERT INTO template_workout_exercises
  (template_workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
SELECT tw.id, e.id, v.sets, v.reps, v.rest, v.ord
FROM (VALUES
  -- ======================= ppl-hyp · blocco 2 (sett. 5) =======================
  ('ppl-hyp', 5, 0, 'Barbell Bench Press',     5,  5, 150, 0),
  ('ppl-hyp', 5, 0, 'Incline Dumbbell Press',  4,  8, 120, 1),
  ('ppl-hyp', 5, 0, 'Seated Shoulder Press',   3,  8,  90, 2),
  ('ppl-hyp', 5, 0, 'Cable Fly',               3, 10,  60, 3),
  ('ppl-hyp', 5, 0, 'Triceps Pushdown',        3, 10,  60, 4),
  ('ppl-hyp', 5, 0, 'Lateral Raise',           3, 12,  45, 5),

  ('ppl-hyp', 5, 1, 'Barbell Row',             5,  5, 150, 0),
  ('ppl-hyp', 5, 1, 'Pull-Up',                 4,  6, 120, 1),
  ('ppl-hyp', 5, 1, 'Lat Pulldown',            3,  8,  90, 2),
  ('ppl-hyp', 5, 1, 'Cable Row',               3, 10,  60, 3),
  ('ppl-hyp', 5, 1, 'Barbell Curl',            3, 10,  60, 4),
  ('ppl-hyp', 5, 1, 'Face Pull',               3, 12,  45, 5),

  ('ppl-hyp', 5, 2, 'Back Squat',              5,  5, 180, 0),
  ('ppl-hyp', 5, 2, 'Romanian Deadlift',       4,  6, 150, 1),
  ('ppl-hyp', 5, 2, 'Leg Press',               3, 10,  90, 2),
  ('ppl-hyp', 5, 2, 'Leg Curl',                3, 10,  60, 3),
  ('ppl-hyp', 5, 2, 'Calf Raise',              4, 12,  45, 4),

  ('ppl-hyp', 5, 3, 'Overhead Press',          5,  5, 150, 0),
  ('ppl-hyp', 5, 3, 'Dip',                     4,  8, 120, 1),
  ('ppl-hyp', 5, 3, 'Incline Dumbbell Press',  3,  8,  90, 2),
  ('ppl-hyp', 5, 3, 'Dumbbell Shoulder Press', 3, 10,  60, 3),
  ('ppl-hyp', 5, 3, 'Skull Crushers',          3, 10,  60, 4),
  ('ppl-hyp', 5, 3, 'Lateral Raise',           3, 12,  45, 5),

  ('ppl-hyp', 5, 4, 'Deadlift',                4,  3, 210, 0),
  ('ppl-hyp', 5, 4, 'Chin-Up',                 4,  6, 120, 1),
  ('ppl-hyp', 5, 4, 'Cable Row',               3,  8,  90, 2),
  ('ppl-hyp', 5, 4, 'Lat Pulldown',            3, 10,  60, 3),
  ('ppl-hyp', 5, 4, 'Hammer Curl',             3, 10,  60, 4),
  ('ppl-hyp', 5, 4, 'Face Pull',               3, 12,  45, 5),

  ('ppl-hyp', 5, 5, 'Hip Thrust',              4,  8, 150, 0),
  ('ppl-hyp', 5, 5, 'Back Squat',              4,  8, 150, 1),
  ('ppl-hyp', 5, 5, 'Leg Curl',                3, 10,  60, 2),
  ('ppl-hyp', 5, 5, 'Leg Press',               3, 12,  60, 3),
  ('ppl-hyp', 5, 5, 'Calf Raise',              4, 12,  45, 4),

  -- ======================= str-5x5 · blocco 2 (sett. 5) =======================
  ('str-5x5', 5, 0, 'Back Squat',              5,  3, 240, 0),
  ('str-5x5', 5, 0, 'Barbell Bench Press',     5,  3, 240, 1),
  ('str-5x5', 5, 0, 'Barbell Row',             5,  3, 210, 2),

  ('str-5x5', 5, 2, 'Back Squat',              5,  3, 240, 0),
  ('str-5x5', 5, 2, 'Overhead Press',          5,  3, 240, 1),
  ('str-5x5', 5, 2, 'Deadlift',                1,  3, 240, 2),

  ('str-5x5', 5, 4, 'Back Squat',              5,  3, 240, 0),
  ('str-5x5', 5, 4, 'Barbell Bench Press',     5,  3, 240, 1),
  ('str-5x5', 5, 4, 'Barbell Row',             5,  3, 210, 2),

  -- ======================= str-5x5 · blocco 3 (sett. 9) =======================
  -- 3×3: meno serie e stesse ripetizioni del blocco 2. Il template è marcato
  -- Beginner, quindi si chiude sui tripli invece di scendere a doppie o singole.
  ('str-5x5', 9, 0, 'Back Squat',              3,  3, 270, 0),
  ('str-5x5', 9, 0, 'Barbell Bench Press',     3,  3, 270, 1),
  ('str-5x5', 9, 0, 'Barbell Row',             3,  3, 240, 2),

  ('str-5x5', 9, 2, 'Back Squat',              3,  3, 270, 0),
  ('str-5x5', 9, 2, 'Overhead Press',          3,  3, 270, 1),
  ('str-5x5', 9, 2, 'Deadlift',                1,  3, 270, 2),

  ('str-5x5', 9, 4, 'Back Squat',              3,  3, 270, 0),
  ('str-5x5', 9, 4, 'Barbell Bench Press',     3,  3, 270, 1),
  ('str-5x5', 9, 4, 'Barbell Row',             3,  3, 240, 2),

  -- ===================== upper-lower · blocco 2 (sett. 5) =====================
  ('upper-lower', 5, 0, 'Barbell Bench Press',     5,  4, 180, 0),
  ('upper-lower', 5, 0, 'Barbell Row',             5,  4, 180, 1),
  ('upper-lower', 5, 0, 'Overhead Press',          4,  6, 150, 2),
  ('upper-lower', 5, 0, 'Lat Pulldown',            3,  8,  90, 3),
  ('upper-lower', 5, 0, 'Barbell Curl',            3,  8,  60, 4),
  ('upper-lower', 5, 0, 'Triceps Pushdown',        3,  8,  60, 5),

  ('upper-lower', 5, 1, 'Back Squat',              5,  4, 210, 0),
  ('upper-lower', 5, 1, 'Romanian Deadlift',       4,  6, 150, 1),
  ('upper-lower', 5, 1, 'Leg Press',               3,  8,  90, 2),
  ('upper-lower', 5, 1, 'Leg Curl',                3, 10,  60, 3),
  ('upper-lower', 5, 1, 'Calf Raise',              4, 12,  45, 4),

  ('upper-lower', 5, 3, 'Incline Dumbbell Press',  4,  8, 120, 0),
  ('upper-lower', 5, 3, 'Cable Row',               4,  8, 120, 1),
  ('upper-lower', 5, 3, 'Dumbbell Shoulder Press', 3, 10,  90, 2),
  ('upper-lower', 5, 3, 'Pull-Up',                 4,  6,  90, 3),
  ('upper-lower', 5, 3, 'Hammer Curl',             3, 10,  60, 4),
  ('upper-lower', 5, 3, 'Skull Crushers',          3, 10,  60, 5),

  ('upper-lower', 5, 4, 'Deadlift',                4,  3, 210, 0),
  ('upper-lower', 5, 4, 'Hip Thrust',              4,  8, 120, 1),
  ('upper-lower', 5, 4, 'Leg Press',               3, 10,  90, 2),
  ('upper-lower', 5, 4, 'Leg Curl',                3, 12,  60, 3),
  ('upper-lower', 5, 4, 'Calf Raise',              4, 15,  45, 4),

  -- ====================== lean-cond · blocco 2 (sett. 4) ======================
  -- Qui si sale di ripetizioni e si scende di recupero: l'obiettivo è il
  -- dimagrimento, quindi il progresso è la densità di lavoro, non il carico.
  ('lean-cond', 4, 0, 'Back Squat',              3, 15,  35, 0),
  ('lean-cond', 4, 0, 'Barbell Bench Press',     3, 15,  35, 1),
  ('lean-cond', 4, 0, 'Cable Row',               3, 15,  35, 2),
  ('lean-cond', 4, 0, 'Overhead Press',          3, 15,  35, 3),
  ('lean-cond', 4, 0, 'Calf Raise',              3, 25,  25, 4),

  ('lean-cond', 4, 1, 'Incline Dumbbell Press',  4, 15,  30, 0),
  ('lean-cond', 4, 1, 'Lat Pulldown',            4, 15,  30, 1),
  ('lean-cond', 4, 1, 'Dumbbell Shoulder Press', 3, 18,  30, 2),
  ('lean-cond', 4, 1, 'Triceps Pushdown',        3, 18,  25, 3),
  ('lean-cond', 4, 1, 'Barbell Curl',            3, 18,  25, 4),

  ('lean-cond', 4, 3, 'Leg Press',               4, 18,  35, 0),
  ('lean-cond', 4, 3, 'Romanian Deadlift',       3, 15,  40, 1),
  ('lean-cond', 4, 3, 'Hip Thrust',              4, 18,  30, 2),
  ('lean-cond', 4, 3, 'Leg Curl',                3, 18,  25, 3),
  ('lean-cond', 4, 3, 'Calf Raise',              3, 25,  25, 4),

  ('lean-cond', 4, 4, 'Deadlift',                3, 12,  50, 0),
  ('lean-cond', 4, 4, 'Dip',                     3, 15,  35, 1),
  ('lean-cond', 4, 4, 'Pull-Up',                 3, 12,  40, 2),
  ('lean-cond', 4, 4, 'Lateral Raise',           3, 18,  25, 3),
  ('lean-cond', 4, 4, 'Face Pull',               3, 18,  25, 4),

  -- ====================== full-body · blocco 2 (sett. 5) ======================
  -- Template per principianti: il salto è contenuto, da 8 ripetizioni a 6.
  ('full-body', 5, 0, 'Back Squat',              4,  6, 150, 0),
  ('full-body', 5, 0, 'Barbell Bench Press',     4,  6, 150, 1),
  ('full-body', 5, 0, 'Barbell Row',             3,  8,  90, 2),
  ('full-body', 5, 0, 'Overhead Press',          3,  8,  90, 3),
  ('full-body', 5, 0, 'Calf Raise',              3, 15,  45, 4),

  ('full-body', 5, 2, 'Romanian Deadlift',       4,  6, 150, 0),
  ('full-body', 5, 2, 'Lat Pulldown',            3,  8,  90, 1),
  ('full-body', 5, 2, 'Incline Dumbbell Press',  3,  8,  90, 2),
  ('full-body', 5, 2, 'Leg Press',               3, 10,  90, 3),
  ('full-body', 5, 2, 'Barbell Curl',            3, 10,  60, 4),

  ('full-body', 5, 4, 'Deadlift',                3,  4, 180, 0),
  ('full-body', 5, 4, 'Dumbbell Shoulder Press', 3,  8,  90, 1),
  ('full-body', 5, 4, 'Cable Row',               3,  8,  90, 2),
  ('full-body', 5, 4, 'Leg Curl',                3, 10,  60, 3),
  ('full-body', 5, 4, 'Triceps Pushdown',        3, 10,  60, 4),

  -- ====================== bro-split · blocco 2 (sett. 5) ======================
  ('bro-split', 5, 0, 'Barbell Bench Press',     5,  5, 150, 0),
  ('bro-split', 5, 0, 'Incline Dumbbell Press',  4,  8, 120, 1),
  ('bro-split', 5, 0, 'Dip',                     4,  8,  90, 2),
  ('bro-split', 5, 0, 'Cable Fly',               3, 10,  60, 3),
  ('bro-split', 5, 0, 'Push-Up',                 2, 12,  45, 4),

  ('bro-split', 5, 1, 'Deadlift',                4,  4, 210, 0),
  ('bro-split', 5, 1, 'Barbell Row',             4,  6, 150, 1),
  ('bro-split', 5, 1, 'Lat Pulldown',            4,  8,  90, 2),
  ('bro-split', 5, 1, 'T-Bar Row',               3,  8,  90, 3),
  ('bro-split', 5, 1, 'Cable Row',               3, 10,  60, 4),
  ('bro-split', 5, 1, 'Face Pull',               3, 12,  45, 5),

  ('bro-split', 5, 2, 'Overhead Press',          5,  5, 150, 0),
  ('bro-split', 5, 2, 'Seated Shoulder Press',   4,  8, 120, 1),
  ('bro-split', 5, 2, 'Dumbbell Shoulder Press', 3, 10,  60, 2),
  ('bro-split', 5, 2, 'Lateral Raise',           4, 12,  45, 3),
  ('bro-split', 5, 2, 'Face Pull',               3, 12,  45, 4),

  ('bro-split', 5, 3, 'Close-Grip Bench Press',  5,  6, 150, 0),
  ('bro-split', 5, 3, 'Barbell Curl',            4,  8,  90, 1),
  ('bro-split', 5, 3, 'Skull Crushers',          4, 10,  60, 2),
  ('bro-split', 5, 3, 'Preacher Curl',           3, 10,  60, 3),
  ('bro-split', 5, 3, 'Triceps Pushdown',        3, 12,  45, 4),
  ('bro-split', 5, 3, 'Hammer Curl',             3, 12,  45, 5),

  ('bro-split', 5, 4, 'Back Squat',              5,  5, 180, 0),
  ('bro-split', 5, 4, 'Romanian Deadlift',       4,  8, 150, 1),
  ('bro-split', 5, 4, 'Leg Press',               4, 10,  90, 2),
  ('bro-split', 5, 4, 'Walking Lunge',           3, 10,  60, 3),
  ('bro-split', 5, 4, 'Leg Curl',                3, 10,  60, 4),
  ('bro-split', 5, 4, 'Calf Raise',              4, 15,  45, 5),

  -- ===================== power-build · blocco 2 (sett. 5) =====================
  -- Il lift principale sale a 6×2 e gli accessori guadagnano una serie: è il
  -- punto di massimo carico settimanale del programma.
  ('power-build', 5, 0, 'Back Squat',              6,  2, 270, 0),
  ('power-build', 5, 0, 'Front Squat',             4,  5, 180, 1),
  ('power-build', 5, 0, 'Bulgarian Split Squat',   4,  8,  90, 2),
  ('power-build', 5, 0, 'Leg Curl',                4, 10,  60, 3),
  ('power-build', 5, 0, 'Hanging Leg Raise',       3, 12,  60, 4),

  ('power-build', 5, 1, 'Barbell Bench Press',     6,  2, 270, 0),
  ('power-build', 5, 1, 'Close-Grip Bench Press',  4,  5, 150, 1),
  ('power-build', 5, 1, 'Incline Dumbbell Press',  4,  8,  90, 2),
  ('power-build', 5, 1, 'Cable Fly',               3, 12,  60, 3),
  ('power-build', 5, 1, 'Triceps Pushdown',        4, 10,  60, 4),

  ('power-build', 5, 3, 'Deadlift',                6,  2, 270, 0),
  ('power-build', 5, 3, 'Romanian Deadlift',       4,  6, 150, 1),
  ('power-build', 5, 3, 'Barbell Row',             4,  6, 120, 2),
  ('power-build', 5, 3, 'Lat Pulldown',            4, 10,  90, 3),
  ('power-build', 5, 3, 'Cable Crunch',            3, 15,  60, 4),

  ('power-build', 5, 4, 'Overhead Press',          6,  2, 270, 0),
  ('power-build', 5, 4, 'Dumbbell Shoulder Press', 4,  6, 120, 1),
  ('power-build', 5, 4, 'Pull-Up',                 5,  6, 120, 2),
  ('power-build', 5, 4, 'Lateral Raise',           4, 12,  45, 3),
  ('power-build', 5, 4, 'Barbell Curl',            4, 10,  60, 4),

  -- ===================== power-build · blocco 3 (sett. 9) =====================
  -- Scarico di volume a carico invariato: meno serie ovunque, recuperi più
  -- lunghi. Le ultime due settimane servono a esprimere quello che si è
  -- costruito, non ad aggiungerci sopra.
  ('power-build', 9, 0, 'Back Squat',              4,  2, 300, 0),
  ('power-build', 9, 0, 'Front Squat',             3,  4, 180, 1),
  ('power-build', 9, 0, 'Bulgarian Split Squat',   3,  8,  90, 2),
  ('power-build', 9, 0, 'Leg Curl',                3, 10,  60, 3),
  ('power-build', 9, 0, 'Hanging Leg Raise',       3, 12,  60, 4),

  ('power-build', 9, 1, 'Barbell Bench Press',     4,  2, 300, 0),
  ('power-build', 9, 1, 'Close-Grip Bench Press',  3,  4, 150, 1),
  ('power-build', 9, 1, 'Incline Dumbbell Press',  3,  8,  90, 2),
  ('power-build', 9, 1, 'Cable Fly',               3, 12,  60, 3),
  ('power-build', 9, 1, 'Triceps Pushdown',        3, 10,  60, 4),

  ('power-build', 9, 3, 'Deadlift',                4,  2, 300, 0),
  ('power-build', 9, 3, 'Romanian Deadlift',       3,  5, 150, 1),
  ('power-build', 9, 3, 'Barbell Row',             3,  6, 120, 2),
  ('power-build', 9, 3, 'Lat Pulldown',            3, 10,  90, 3),
  ('power-build', 9, 3, 'Cable Crunch',            3, 15,  60, 4),

  ('power-build', 9, 4, 'Overhead Press',          4,  2, 300, 0),
  ('power-build', 9, 4, 'Dumbbell Shoulder Press', 3,  6, 120, 1),
  ('power-build', 9, 4, 'Pull-Up',                 4,  6, 120, 2),
  ('power-build', 9, 4, 'Lateral Raise',           3, 12,  45, 3),
  ('power-build', 9, 4, 'Barbell Curl',            3, 10,  60, 4),

  -- ===================== glute-lower · blocco 2 (sett. 5) =====================
  ('glute-lower', 5, 0, 'Hip Thrust',              5,  8, 120, 0),
  ('glute-lower', 5, 0, 'Romanian Deadlift',       4,  6, 150, 1),
  ('glute-lower', 5, 0, 'Bulgarian Split Squat',   4, 10,  90, 2),
  ('glute-lower', 5, 0, 'Leg Curl',                3, 12,  60, 3),
  ('glute-lower', 5, 0, 'Calf Raise',              3, 15,  45, 4),

  ('glute-lower', 5, 2, 'Back Squat',              5,  6, 150, 0),
  ('glute-lower', 5, 2, 'Front Squat',             4,  6, 150, 1),
  ('glute-lower', 5, 2, 'Leg Press',               4, 10,  90, 2),
  ('glute-lower', 5, 2, 'Walking Lunge',           3, 10,  60, 3),
  ('glute-lower', 5, 2, 'Calf Raise',              4, 15,  45, 4),

  ('glute-lower', 5, 4, 'Deadlift',                5,  4, 210, 0),
  ('glute-lower', 5, 4, 'Hip Thrust',              4,  8, 120, 1),
  ('glute-lower', 5, 4, 'Leg Curl',                4, 10,  60, 2),
  ('glute-lower', 5, 4, 'Kettlebell Swing',        4, 12,  60, 3),
  ('glute-lower', 5, 4, 'Hanging Leg Raise',       3, 12,  60, 4),

  -- ====================== min-2day · blocco 2 (sett. 5) =======================
  ('min-2day', 5, 0, 'Back Squat',              5,  5, 150, 0),
  ('min-2day', 5, 0, 'Barbell Bench Press',     5,  5, 150, 1),
  ('min-2day', 5, 0, 'Barbell Row',             4,  8,  90, 2),
  ('min-2day', 5, 0, 'Overhead Press',          4,  8,  90, 3),
  ('min-2day', 5, 0, 'Hanging Leg Raise',       3, 12,  60, 4),

  ('min-2day', 5, 3, 'Deadlift',                5,  4, 180, 0),
  ('min-2day', 5, 3, 'Pull-Up',                 4,  6, 120, 1),
  ('min-2day', 5, 3, 'Incline Dumbbell Press',  4,  8,  90, 2),
  ('min-2day', 5, 3, 'Bulgarian Split Squat',   3,  8,  90, 3),
  ('min-2day', 5, 3, 'Cable Crunch',            3, 15,  60, 4),

  -- ===================== ath-perform · blocco 2 (sett. 4) =====================
  -- Sui salti si scende di ripetizioni e si allunga il recupero: la qualità del
  -- singolo salto conta più del totale. Il giorno di condizionamento va invece
  -- verso più densità, come in lean-cond.
  ('ath-perform', 4, 0, 'Box Jump',                6,  3, 150, 0),
  ('ath-perform', 4, 0, 'Back Squat',              5,  3, 210, 1),
  ('ath-perform', 4, 0, 'Romanian Deadlift',       3,  6, 150, 2),
  ('ath-perform', 4, 0, 'Walking Lunge',           3, 10,  60, 3),
  ('ath-perform', 4, 0, 'Hanging Leg Raise',       3, 15,  45, 4),

  ('ath-perform', 4, 1, 'Barbell Bench Press',     5,  3, 210, 0),
  ('ath-perform', 4, 1, 'Pull-Up',                 5,  4, 150, 1),
  ('ath-perform', 4, 1, 'Overhead Press',          4,  6, 120, 2),
  ('ath-perform', 4, 1, 'Barbell Row',             3,  8,  90, 3),
  ('ath-perform', 4, 1, 'Face Pull',               3, 15,  45, 4),

  ('ath-perform', 4, 3, 'Kettlebell Swing',        5, 20,  40, 0),
  ('ath-perform', 4, 3, 'Box Jump',                5,  6,  60, 1),
  ('ath-perform', 4, 3, 'Push-Up',                 4, 20,  40, 2),
  ('ath-perform', 4, 3, 'Walking Lunge',           4, 20,  40, 3),
  ('ath-perform', 4, 3, 'Cable Crunch',            3, 20,  40, 4),

  ('ath-perform', 4, 4, 'Deadlift',                5,  3, 210, 0),
  ('ath-perform', 4, 4, 'Front Squat',             4,  6, 150, 1),
  ('ath-perform', 4, 4, 'Dip',                     4,  8,  90, 2),
  ('ath-perform', 4, 4, 'Cable Row',               3, 10,  60, 3),
  ('ath-perform', 4, 4, 'Kettlebell Swing',        3, 15,  45, 4)
) AS v(template_id, wk, dow, ex_name, sets, reps, rest, ord)
JOIN template_workouts tw
  ON tw.template_id = v.template_id
 AND tw.week_number = v.wk
 AND tw.day_of_week = v.dow
JOIN exercises e ON e.name = v.ex_name
ON CONFLICT (template_workout_id, order_index) DO NOTHING;
