INSERT INTO plan_templates (id, name, goal, focus, level, days_per_week, session_min, total_weeks, glyph, tag)
VALUES
  ('ppl-hyp',     'Push / Pull / Legs',    'Hypertrophy',     'Build size with high weekly volume',   'Intermediate', 6, 60, 8,  '💪', 'Most popular'),
  ('str-5x5',     '5×5 Strength',          'Max strength',    'Heavy compounds, simple progression',  'Beginner',     3, 45, 12, '🏋', NULL),
  ('upper-lower', 'Upper / Lower',         'Strength + size', 'Balanced split, four focused days',    'Intermediate', 4, 55, 8,  '🔁', NULL),
  ('lean-cond',   'Lean & Conditioned',    'Fat loss',        'Supersets and short rest to stay lean','All levels',   4, 40, 6,  '🔥', NULL),
  ('full-body',   'Full-Body Foundations', 'General fitness', 'Master the basics, train full body',   'Beginner',     3, 35, 8,  '🌱', NULL),
  ('bro-split',   'Classic Bro Split',     'Hypertrophy',     'One muscle group per day, high volume','Intermediate', 5, 55, 8,  '🏆', NULL),
  ('power-build', 'Powerbuilding',         'Strength + size', 'Heavy main lift, then hypertrophy',    'Advanced',     4, 70, 10, '⚡', NULL),
  ('glute-lower', 'Glutes & Lower Body',   'Lower body focus','Hip-dominant work three times a week', 'All levels',   3, 45, 8,  '🦵', NULL),
  ('min-2day',    'Minimalist 2-Day',      'General fitness', 'Two full-body days for busy weeks',    'All levels',   2, 40, 8,  '⏱', 'Time-saver'),
  ('ath-perform', 'Athletic Performance',  'Power + conditioning','Explosive lifts and conditioning', 'Intermediate', 4, 50, 6,  '🏃', NULL)
ON CONFLICT (id) DO NOTHING;

INSERT INTO exercises (name, muscle_group, category) VALUES
  ('Barbell Bench Press',      'Chest',          'compound'),
  ('Incline Dumbbell Press',   'Upper Chest',    'compound'),
  ('Cable Fly',                'Chest',          'isolation'),
  ('Overhead Press',           'Shoulders',      'compound'),
  ('Lateral Raise',            'Shoulders',      'isolation'),
  ('Triceps Pushdown',         'Triceps',        'isolation'),
  ('Skull Crushers',           'Triceps',        'isolation'),
  ('Barbell Row',              'Back',           'compound'),
  ('Pull-Up',                  'Back',           'compound'),
  ('Lat Pulldown',             'Back',           'compound'),
  ('Cable Row',                'Back',           'compound'),
  ('Barbell Curl',             'Biceps',         'isolation'),
  ('Hammer Curl',              'Biceps',         'isolation'),
  ('Back Squat',               'Quads',          'compound'),
  ('Romanian Deadlift',        'Hamstrings',     'compound'),
  ('Leg Press',                'Quads',          'compound'),
  ('Leg Curl',                 'Hamstrings',     'isolation'),
  ('Calf Raise',               'Calves',         'isolation'),
  ('Deadlift',                 'Full Body',      'compound'),
  ('Hip Thrust',               'Glutes',         'compound'),
  ('Dumbbell Shoulder Press',  'Shoulders',      'compound'),
  ('Face Pull',                'Rear Delts',     'isolation'),
  ('Seated Shoulder Press',    'Shoulders',      'compound'),
  ('Dip',                      'Chest/Triceps',  'compound'),
  ('Chin-Up',                  'Back/Biceps',    'compound'),
  -- aggiunti per i template bro-split / power-build / glute-lower /
  -- min-2day / ath-perform. Solo esercizi a ripetizioni: lo schema ha
  -- target_reps, quindi niente plank o farmer carry (a tempo).
  ('Front Squat',              'Quads',          'compound'),
  ('Bulgarian Split Squat',    'Quads/Glutes',   'compound'),
  ('Walking Lunge',            'Quads/Glutes',   'compound'),
  ('Push-Up',                  'Chest',          'compound'),
  ('Close-Grip Bench Press',   'Triceps',        'compound'),
  ('Preacher Curl',            'Biceps',         'isolation'),
  ('T-Bar Row',                'Back',           'compound'),
  ('Hanging Leg Raise',        'Core',           'isolation'),
  ('Cable Crunch',             'Core',           'isolation'),
  ('Kettlebell Swing',         'Full Body',      'compound'),
  ('Box Jump',                 'Power',          'compound')
ON CONFLICT DO NOTHING;
