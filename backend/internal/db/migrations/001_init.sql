CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS magic_link_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL,
  token TEXT UNIQUE NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS exercises (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  muscle_group TEXT NOT NULL,
  category TEXT NOT NULL DEFAULT 'compound'
);

CREATE TABLE IF NOT EXISTS plan_templates (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  goal TEXT NOT NULL,
  focus TEXT NOT NULL,
  level TEXT NOT NULL,
  days_per_week INT NOT NULL,
  session_min INT NOT NULL,
  total_weeks INT NOT NULL,
  glyph TEXT NOT NULL DEFAULT '💪',
  tag TEXT
);

CREATE TABLE IF NOT EXISTS user_programs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  template_id TEXT REFERENCES plan_templates(id),
  name TEXT NOT NULL,
  goal TEXT NOT NULL,
  level TEXT NOT NULL,
  days_per_week INT NOT NULL,
  total_weeks INT NOT NULL,
  current_week INT NOT NULL DEFAULT 1,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS program_workouts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  program_id UUID NOT NULL REFERENCES user_programs(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  focus TEXT NOT NULL,
  day_of_week INT NOT NULL,
  week_number INT NOT NULL DEFAULT 1,
  order_in_week INT NOT NULL
);

CREATE TABLE IF NOT EXISTS program_workout_exercises (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workout_id UUID NOT NULL REFERENCES program_workouts(id) ON DELETE CASCADE,
  exercise_id UUID NOT NULL REFERENCES exercises(id),
  sets INT NOT NULL DEFAULT 3,
  target_reps INT NOT NULL DEFAULT 8,
  rest_seconds INT NOT NULL DEFAULT 90,
  order_index INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS session_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workout_id UUID REFERENCES program_workouts(id),
  program_id UUID REFERENCES user_programs(id),
  name TEXT NOT NULL,
  duration_min INT,
  total_volume NUMERIC(10,2),
  total_sets INT,
  avg_rpe NUMERIC(3,1),
  session_rpe INT,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS set_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id UUID NOT NULL REFERENCES session_logs(id) ON DELETE CASCADE,
  exercise_id UUID NOT NULL REFERENCES exercises(id),
  exercise_name TEXT NOT NULL,
  set_number INT NOT NULL,
  weight NUMERIC(6,2),
  reps INT,
  rpe NUMERIC(3,1),
  is_pr BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS body_weight_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  weight NUMERIC(5,2) NOT NULL,
  unit TEXT NOT NULL DEFAULT 'kg',
  logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
