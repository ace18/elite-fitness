package repository

import (
	"context"
	"errors"

	"github.com/elitecoach/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier è il sottoinsieme di pgx implementato sia da *pgxpool.Pool che da
// pgx.Tx. Permette ai metodi del repo di partecipare a una transazione del
// chiamante invece di aprire una connessione propria dal pool.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

var (
	_ Querier = (*pgxpool.Pool)(nil)
	_ Querier = (pgx.Tx)(nil)
)

type ProgramRepo struct {
	db *pgxpool.Pool
}

func NewProgramRepo(db *pgxpool.Pool) *ProgramRepo { return &ProgramRepo{db: db} }

func (r *ProgramRepo) DB() *pgxpool.Pool { return r.db }

func (r *ProgramRepo) GetTemplates(ctx context.Context) ([]model.PlanTemplate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, goal, focus, level, days_per_week, session_min, total_weeks, glyph, tag
		 FROM plan_templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []model.PlanTemplate
	for rows.Next() {
		var t model.PlanTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Goal, &t.Focus, &t.Level,
			&t.DaysPerWeek, &t.SessionMin, &t.TotalWeeks, &t.Glyph, &t.Tag); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func (r *ProgramRepo) GetActiveProgram(ctx context.Context, userID string) (*model.UserProgram, error) {
	p := &model.UserProgram{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, template_id, name, goal, level, days_per_week,
		        total_weeks, current_week, is_active, started_at
		 FROM user_programs WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY started_at DESC LIMIT 1`,
		userID,
	).Scan(&p.ID, &p.UserID, &p.TemplateID, &p.Name, &p.Goal, &p.Level,
		&p.DaysPerWeek, &p.TotalWeeks, &p.CurrentWeek, &p.IsActive, &p.StartedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *ProgramRepo) DeactivateAll(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE user_programs SET is_active = FALSE WHERE user_id = $1`, userID)
	return err
}

// CreateFromTemplate disattiva i programmi esistenti, crea quello nuovo e ci
// copia dentro i workout della template (settimana 1) — tutto in una
// transazione, così non può restare un programma a metà senza allenamenti.
// Restituisce l'id del programma creato.
func (r *ProgramRepo) CreateFromTemplate(ctx context.Context, userID string, t model.PlanTemplate) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE user_programs SET is_active = FALSE WHERE user_id = $1`, userID); err != nil {
		return "", err
	}

	var programID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO user_programs (user_id, template_id, name, goal, level, days_per_week, total_weeks)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		userID, t.ID, t.Name, t.Goal, t.Level, t.DaysPerWeek, t.TotalWeeks,
	).Scan(&programID); err != nil {
		return "", err
	}

	// Copia i giorni e i rispettivi esercizi. La CTE data-modifying restituisce
	// gli id appena creati, su cui si aggancia l'insert degli esercizi.
	if _, err := tx.Exec(ctx, `
		WITH new_workouts AS (
		  INSERT INTO program_workouts
		    (program_id, name, focus, day_of_week, week_number, order_in_week)
		  SELECT $1, tw.name, tw.focus, tw.day_of_week, 1, tw.order_in_week
		  FROM template_workouts tw
		  WHERE tw.template_id = $2
		  RETURNING id, day_of_week
		)
		INSERT INTO program_workout_exercises
		  (workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
		SELECT nw.id, twe.exercise_id, twe.sets, twe.target_reps, twe.rest_seconds, twe.order_index
		FROM new_workouts nw
		JOIN template_workouts tw
		  ON tw.template_id = $2 AND tw.day_of_week = nw.day_of_week
		JOIN template_workout_exercises twe
		  ON twe.template_workout_id = tw.id
	`, programID, t.ID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return programID, nil
}

func (r *ProgramRepo) GetWorkoutsForProgram(ctx context.Context, programID string) ([]model.ProgramWorkout, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, program_id, name, focus, day_of_week, week_number, order_in_week
		 FROM program_workouts WHERE program_id = $1 ORDER BY week_number, order_in_week`,
		programID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var workouts []model.ProgramWorkout
	for rows.Next() {
		var w model.ProgramWorkout
		if err := rows.Scan(&w.ID, &w.ProgramID, &w.Name, &w.Focus,
			&w.DayOfWeek, &w.WeekNumber, &w.OrderInWeek); err != nil {
			return nil, err
		}
		workouts = append(workouts, w)
	}
	return workouts, nil
}

func (r *ProgramRepo) GetExercisesForWorkout(ctx context.Context, workoutID string) ([]model.WorkoutExercise, error) {
	rows, err := r.db.Query(ctx,
		`SELECT pwe.id, pwe.exercise_id, e.name, e.muscle_group,
		        pwe.sets, pwe.target_reps, pwe.rest_seconds, pwe.order_index
		 FROM program_workout_exercises pwe
		 JOIN exercises e ON e.id = pwe.exercise_id
		 WHERE pwe.workout_id = $1
		 ORDER BY pwe.order_index`,
		workoutID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var exercises []model.WorkoutExercise
	for rows.Next() {
		var ex model.WorkoutExercise
		if err := rows.Scan(&ex.ID, &ex.ExerciseID, &ex.Name, &ex.Muscle,
			&ex.Sets, &ex.TargetReps, &ex.RestSeconds, &ex.OrderIndex); err != nil {
			return nil, err
		}
		exercises = append(exercises, ex)
	}
	return exercises, nil
}

// GetLastSetForExercise returns the most recent weight+rpe for an exercise for a user.
func (r *ProgramRepo) GetLastSetForExercise(ctx context.Context, userID, exerciseID string) (weight float64, rpe *float64, pr float64, err error) {
	err = r.db.QueryRow(ctx,
		`SELECT sl.weight, sl.rpe
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1 AND sl.exercise_id = $2
		 ORDER BY sess.completed_at DESC, sl.set_number DESC
		 LIMIT 1`,
		userID, exerciseID,
	).Scan(&weight, &rpe)
	if err != nil {
		weight = 0
		err = nil
	}
	// best 1RM estimate
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(weight * (1 + reps::float / 30)), 0)
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1 AND sl.exercise_id = $2`,
		userID, exerciseID,
	).Scan(&pr)
	return
}

// FindOrCreateExercise gira su q: passare la tx del chiamante quando fa parte
// di una transazione, altrimenti r.DB(). Usare il pool qui dentro mentre il
// chiamante è in transazione lascerebbe righe orfane in caso di rollback.
func (r *ProgramRepo) FindOrCreateExercise(ctx context.Context, q Querier, name, muscleGroup string) (string, error) {
	var id string
	err := q.QueryRow(ctx,
		`INSERT INTO exercises (name, muscle_group)
		 VALUES ($1, $2)
		 ON CONFLICT DO NOTHING
		 RETURNING id`,
		name, muscleGroup,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	// Solo ErrNoRows significa "conflitto, esiste già". Su un errore vero la tx
	// è ormai abortita: rilanciarlo invece di mascherarlo con la SELECT.
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = q.QueryRow(ctx,
		`SELECT id FROM exercises WHERE name = $1 ORDER BY id LIMIT 1`, name,
	).Scan(&id)
	return id, err
}
