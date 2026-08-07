-- Carichi prescritti, e i due cicli di squat che li usano.
--
-- Fin qui nessun programma diceva quanto caricare: sets, reps e recupero
-- stavano in tabella e il peso lo decideva suggestNext dai set registrati. Il
-- ciclo di squat funziona al contrario — il carico di ogni singola seduta è
-- scritto, e discende dal massimale dell'atleta.
--
-- Come si esprime un carico prescritto: `load_pct` per la parte proporzionale
-- al massimale e `load_offset_kg` per quella assoluta.
--
--     carico = load_pct × massimale + load_offset_kg
--
-- Due numeri invece di uno perché il ciclo mescola le due cose: la settimana 1
-- è una percentuale secca (65% il primo giorno, 70% il secondo), da lì in poi
-- si sale di chili pieni — +10, +10, +10 — e la settimana 8 riparte dal
-- massimale meno una quota fissa. Con la sola percentuale i +10 kg
-- diventerebbero incrementi proporzionali, che è esattamente ciò che questo
-- programma non è.
--
-- Le righe senza carico (load_pct e load_offset_kg a NULL) restano come prima:
-- il peso lo propone suggestNext. È il caso di tutto il complementare.
ALTER TABLE template_workout_exercises
  ADD COLUMN IF NOT EXISTS load_pct       NUMERIC(5,4),
  ADD COLUMN IF NOT EXISTS load_offset_kg NUMERIC(6,2);

-- Sul programma dell'utente il carico è già in chili: si calcola una volta
-- quando il programma viene creato, come già si copiano serie e ripetizioni.
ALTER TABLE program_workout_exercises
  ADD COLUMN IF NOT EXISTS load_kg NUMERIC(6,2);

-- Il massimale su cui è stato calcolato. Serve a poterlo rifare se cambia, e a
-- mostrare le percentuali.
ALTER TABLE user_programs
  ADD COLUMN IF NOT EXISTS one_rm_kg NUMERIC(6,2);

-- La finestra di massimali per cui una template ha senso. NULL su tutte le
-- altre: valgono per chiunque.
--
-- Serve perché qui gli incrementi sono assoluti e quindi NON si adattano verso
-- il basso. L'ultima doppia della settimana 7 sta a 65% + 60 kg: al massimale
-- di riferimento è il 100%, a 145 kg è il 106%. Il programma lo mette in conto
-- — margini larghi sono più facili da reggere su massimali leggeri, e non è
-- richiesto chiudere tutte le serie — ma sotto la finestra il conto non torna
-- più e la template va rifiutata invece che proposta.
ALTER TABLE plan_templates
  ADD COLUMN IF NOT EXISTS min_one_rm_kg NUMERIC(6,2),
  ADD COLUMN IF NOT EXISTS max_one_rm_kg NUMERIC(6,2);

-- ------------------------------------------------------------- le template
-- Due cicli, non una template sola parametrica: dalla settimana 5 in poi i due
-- fogli divergono davvero (il blocco finale è tarato per far cadere l'ultima
-- doppia appena sopra il massimale di partenza, e i chili per farlo non sono
-- gli stessi). Sono due programmi, non due tarature dello stesso.
INSERT INTO plan_templates
  (id, name, goal, focus, level, days_per_week, session_min, total_weeks, glyph, tag,
   min_one_rm_kg, max_one_rm_kg)
VALUES
  ('squat-170', 'Squat Cycle <= 170', 'Max strength',
   'Eight weeks of back squat, absolute kg progression', 'Intermediate', 2, 60, 8, '🦵', 'Peaking',
   145, 170),
  ('squat-180', 'Squat Cycle > 170', 'Max strength',
   'Eight weeks of back squat, absolute kg progression', 'Advanced', 2, 60, 8, '🦵', 'Peaking',
   170.01, 195)
ON CONFLICT (id) DO NOTHING;

-- I giorni. Day 1 lunedì, Day 2 giovedì: i fogli non fissano il giorno, serve
-- solo che i due allenamenti siano distanziati.
INSERT INTO template_workouts (template_id, name, focus, day_of_week, week_number, order_in_week)
SELECT t.id, v.name, v.focus, v.dow, v.wk, v.ord
FROM (VALUES
  ('Day 1 · Volume',       'Back Squat · assistenza', 0, 1, 0),
  ('Day 2 · Intensity',    'Back Squat',              3, 1, 1),
  ('Day 1 · Volume',       'Back Squat · assistenza', 0, 2, 0),
  ('Day 2 · Intensity',    'Back Squat',              3, 2, 1),
  ('Day 1 · Volume',       'Back Squat · assistenza', 0, 3, 0),
  ('Day 2 · Intensity',    'Back Squat',              3, 3, 1),
  ('Day 1 · Volume',       'Back Squat · assistenza', 0, 4, 0),
  ('Day 2 · Intensity',    'Back Squat',              3, 4, 1),
  ('Day 1 · Heavy',        'Back Squat · assistenza', 0, 5, 0),
  ('Day 2 · Heavy',        'Back Squat',              3, 5, 1),
  ('Day 1 · Heavy',        'Back Squat · assistenza', 0, 6, 0),
  ('Day 2 · Heavy',        'Back Squat',              3, 6, 1),
  ('Day 1 · Peak',         'Back Squat · assistenza', 0, 7, 0),
  ('Day 2 · Peak',         'Back Squat',              3, 7, 1),
  ('Day 1 · Deload',       'Back Squat · scarico',    0, 8, 0),
  ('Day 2 · Test',         'Back Squat · massimale',  3, 8, 1)
) AS v(name, focus, dow, wk, ord)
CROSS JOIN (VALUES ('squat-170'), ('squat-180')) AS t(id)
ON CONFLICT (template_id, week_number, day_of_week) DO NOTHING;

-- ------------------------------------------------------- lo squat, prescritto
-- Gli offset vengono dalla colonna in chili dei fogli, non dalle etichette
-- "plus N kg": in quattro sedute le due cose non coincidono (settimana 5 giorno
-- 2 e settimana 7 giorno 1, in entrambi i cicli, dove il riferimento è la
-- seduta precedente e non lo stesso giorno della settimana prima). I chili sono
-- quelli effettivamente sollevati, quindi vincono loro.
--
-- Verifica veloce, al massimale di riferimento:
--   squat-170: 110.5 119 120.5 129 130.5 139 140.5 149 150.5 155.5 155.5 160.5 165.5 170.5
--   squat-180: 117 126 127 136 137 146 147 156 157 167 162 172 177 182
INSERT INTO template_workout_exercises
  (template_workout_id, exercise_id, sets, target_reps, rest_seconds, order_index,
   load_pct, load_offset_kg)
SELECT tw.id, e.id, v.sets, v.reps, v.rest, v.ord, v.pct, v.off
FROM (VALUES
  -- ===================== squat-170 (riferimento 170 kg) =====================
  ('squat-170', 1, 0, 4, 10, 240, 0, 0.65, 0.0),
  ('squat-170', 1, 3, 5,  8, 240, 0, 0.70, 0.0),
  ('squat-170', 2, 0, 4, 10, 240, 0, 0.65, 10.0),
  ('squat-170', 2, 3, 5,  6, 240, 0, 0.70, 10.0),
  ('squat-170', 3, 0, 5,  8, 240, 0, 0.65, 20.0),
  ('squat-170', 3, 3, 4,  6, 240, 0, 0.70, 20.0),
  ('squat-170', 4, 0, 6,  4, 240, 0, 0.65, 30.0),
  -- 3×8 a quasi il 90%: è così nel foglio ed è voluto — non è richiesto
  -- chiudere tutte le ripetizioni di tutte le serie.
  ('squat-170', 4, 3, 3,  8, 300, 0, 0.70, 30.0),
  ('squat-170', 5, 0, 5,  5, 300, 0, 0.65, 40.0),
  ('squat-170', 5, 3, 3,  3, 300, 0, 0.70, 36.5),
  ('squat-170', 6, 0, 5,  5, 300, 0, 0.65, 45.0),
  ('squat-170', 6, 3, 5,  3, 300, 0, 0.70, 41.5),
  ('squat-170', 7, 0, 5,  3, 300, 0, 0.65, 55.0),
  ('squat-170', 7, 3, 2,  2, 300, 0, 0.70, 51.5),

  -- ===================== squat-180 (riferimento 180 kg) =====================
  ('squat-180', 1, 0, 4, 10, 240, 0, 0.65, 0.0),
  ('squat-180', 1, 3, 5,  8, 240, 0, 0.70, 0.0),
  ('squat-180', 2, 0, 4, 10, 240, 0, 0.65, 10.0),
  ('squat-180', 2, 3, 5,  6, 240, 0, 0.70, 10.0),
  ('squat-180', 3, 0, 5,  8, 240, 0, 0.65, 20.0),
  ('squat-180', 3, 3, 4,  6, 240, 0, 0.70, 20.0),
  ('squat-180', 4, 0, 6,  4, 240, 0, 0.65, 30.0),
  ('squat-180', 4, 3, 3,  4, 300, 0, 0.70, 30.0),
  ('squat-180', 5, 0, 5,  5, 300, 0, 0.65, 40.0),
  ('squat-180', 5, 3, 3,  3, 300, 0, 0.70, 41.0),
  ('squat-180', 6, 0, 5,  5, 300, 0, 0.65, 45.0),
  ('squat-180', 6, 3, 5,  3, 300, 0, 0.70, 46.0),
  ('squat-180', 7, 0, 5,  3, 300, 0, 0.65, 60.0),
  ('squat-180', 7, 3, 2,  2, 300, 0, 0.70, 56.0),

  -- ============ settimana 8: scarico e test, identica nei due cicli =========
  -- Qui il riferimento non è più la percentuale di partenza ma il massimale
  -- stesso, meno una quota fissa.
  ('squat-170', 8, 0, 3, 2, 240, 0, 1.0, -30.0),
  ('squat-170', 8, 0, 3, 1, 300, 1, 1.0, -10.0),
  ('squat-170', 8, 3, 2, 2, 300, 0, 1.0, -25.0),
  ('squat-170', 8, 3, 2, 2, 300, 1, 1.0, -15.0),
  ('squat-170', 8, 3, 1, 1, 300, 2, 1.0,   0.0),
  ('squat-180', 8, 0, 3, 2, 240, 0, 1.0, -30.0),
  ('squat-180', 8, 0, 3, 1, 300, 1, 1.0, -10.0),
  ('squat-180', 8, 3, 2, 2, 300, 0, 1.0, -25.0),
  ('squat-180', 8, 3, 2, 2, 300, 1, 1.0, -15.0),
  ('squat-180', 8, 3, 1, 1, 300, 2, 1.0,   0.0)
) AS v(template_id, wk, dow, sets, reps, rest, ord, pct, off)
JOIN template_workouts tw
  ON tw.template_id = v.template_id AND tw.week_number = v.wk AND tw.day_of_week = v.dow
JOIN exercises e ON e.name = 'Back Squat'
ON CONFLICT (template_workout_id, order_index) DO NOTHING;

-- L'ultima singola del test è aperta: "proceed to maximum". Nessun carico
-- prescritto, lo decide l'atleta sul momento.
INSERT INTO template_workout_exercises
  (template_workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
SELECT tw.id, e.id, 1, 1, 300, 3
FROM template_workouts tw
JOIN exercises e ON e.name = 'Back Squat'
WHERE tw.template_id IN ('squat-170', 'squat-180')
  AND tw.week_number = 8 AND tw.day_of_week = 3
ON CONFLICT (template_workout_id, order_index) DO NOTHING;

-- ------------------------------------------------------------- il complementare
-- I fogli hanno lavoro accessorio a tempo e per lato (hold da 45 secondi, "8es",
-- superset con banda e kettlebell). Lo schema ha target_reps e basta, quindi
-- qui c'è una sostituzione a ripetizioni con esercizi già a catalogo: stessa
-- funzione — monopodalico, catena posteriore, core — senza esercizi nuovi.
--
-- Nessun carico prescritto: il complementare resta autoregolato da suggestNext,
-- come in tutti gli altri programmi.
INSERT INTO template_workout_exercises
  (template_workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
SELECT tw.id, e.id, v.sets, v.reps, v.rest, v.ord
FROM (VALUES
  (1, 'Bulgarian Split Squat',  4,  8, 90, 1),
  (1, 'Hanging Leg Raise',      4, 12, 60, 2),
  (2, 'Bulgarian Split Squat',  4, 10, 90, 1),
  (2, 'Hanging Leg Raise',      4, 15, 60, 2),
  (3, 'Walking Lunge',          4, 10, 90, 1),
  (3, 'Cable Crunch',           4, 15, 60, 2),
  (4, 'Walking Lunge',          4, 12, 90, 1),
  (4, 'Cable Crunch',           4, 15, 60, 2),
  (5, 'Romanian Deadlift',      4,  8, 120, 1),
  (5, 'Hanging Leg Raise',      4, 15, 60, 2),
  (6, 'Kettlebell Swing',       4, 15, 60, 1),
  (6, 'Cable Crunch',           4, 20, 60, 2),
  (7, 'Kettlebell Swing',       4, 15, 60, 1),
  (7, 'Hanging Leg Raise',      4, 15, 60, 2)
) AS v(wk, ex_name, sets, reps, rest, ord)
CROSS JOIN (VALUES ('squat-170'), ('squat-180')) AS t(template_id)
JOIN template_workouts tw
  ON tw.template_id = t.template_id AND tw.week_number = v.wk AND tw.day_of_week = 0
JOIN exercises e ON e.name = v.ex_name
ON CONFLICT (template_workout_id, order_index) DO NOTHING;
