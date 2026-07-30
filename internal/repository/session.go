package repository

import (
	"context"
	"errors"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepo struct {
	db *pgxpool.Pool
}

func NewSessionRepo(db *pgxpool.Pool) *SessionRepo { return &SessionRepo{db: db} }

// CountSessionsForProgramSince conta le sessioni registrate per un programma a
// partire da `since`. Serve a capire se l'ultima settimana è stata completata.
func (r *SessionRepo) CountSessionsForProgramSince(ctx context.Context, programID string, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM session_logs WHERE program_id = $1 AND completed_at >= $2`,
		programID, since,
	).Scan(&n)
	return n, err
}

// SaveSession registra la sessione e dice se l'ha inserita davvero.
//
// `inserted == false` significa che il client aveva già mandato questo
// ClientSessionID: non si scrive niente, s.ID viene riempito con la sessione
// esistente e chi chiama deve saltare tutto ciò che non va rifatto due volte
// (in particolare il controllo di completamento del programma).
//
// Il ritentativo NON deve riaggiungere le serie: appenderle alla sessione
// originale sarebbe peggio di una sessione duplicata, perché falserebbe volume,
// PR e conteggi senza che si veda una riga in più.
func (r *SessionRepo) SaveSession(ctx context.Context, s *model.SessionLog) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO session_logs
		 (user_id, client_session_id, workout_id, program_id, name, duration_min,
		  total_volume, total_sets, avg_rpe, session_rpe, completed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 ON CONFLICT (user_id, client_session_id) DO NOTHING
		 RETURNING id`,
		s.UserID, s.ClientSessionID, s.WorkoutID, s.ProgramID, s.Name, s.DurationMin,
		s.TotalVolume, s.TotalSets, s.AvgRPE, s.SessionRPE, s.CompletedAt,
	).Scan(&s.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Nessuna riga restituita = conflitto: la sessione c'è già. Si recupera
		// il suo id e si esce senza toccare set_logs.
		if err := r.db.QueryRow(ctx,
			`SELECT id FROM session_logs WHERE user_id = $1 AND client_session_id = $2`,
			s.UserID, s.ClientSessionID,
		).Scan(&s.ID); err != nil {
			return false, err
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}

	for i := range s.Sets {
		set := &s.Sets[i]
		set.SessionID = s.ID
		// check PR
		var prevMax float64
		_ = tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(sl.weight), 0)
			 FROM set_logs sl
			 JOIN session_logs sess ON sess.id = sl.session_id
			 WHERE sess.user_id = $1 AND sl.exercise_id = $2`,
			s.UserID, set.ExerciseID,
		).Scan(&prevMax)
		if set.Weight > prevMax {
			set.IsPR = true
		}
		err = tx.QueryRow(ctx,
			`INSERT INTO set_logs
			 (session_id, exercise_id, exercise_name, set_number, weight, reps, rpe, is_pr)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			s.ID, set.ExerciseID, set.ExerciseName, set.SetNumber,
			set.Weight, set.Reps, set.RPE, set.IsPR,
		).Scan(&set.ID)
		if err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *SessionRepo) GetLastSession(ctx context.Context, userID string) (*model.SessionLog, error) {
	s := &model.SessionLog{}
	err := r.db.QueryRow(ctx,
		`SELECT id, workout_id, program_id, name, duration_min, total_volume,
		        total_sets, avg_rpe, session_rpe, completed_at
		 FROM session_logs WHERE user_id = $1
		 ORDER BY completed_at DESC LIMIT 1`,
		userID,
	).Scan(&s.ID, &s.WorkoutID, &s.ProgramID, &s.Name, &s.DurationMin,
		&s.TotalVolume, &s.TotalSets, &s.AvgRPE, &s.SessionRPE, &s.CompletedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *SessionRepo) LogBodyWeight(ctx context.Context, userID string, weight float64, unit string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO body_weight_logs (user_id, weight, unit) VALUES ($1,$2,$3)`,
		userID, weight, unit)
	return err
}

func (r *SessionRepo) GetProgressMetrics(ctx context.Context, userID string, weekGoal int) (*model.ProgressMetrics, error) {
	// Slice inizializzate vuote, non nil: in Go una slice nil diventa `null` in
	// JSON, e il client fa .filter()/.slice() su questi campi. Un utente appena
	// registrato non ha PR né pesate, quindi era esattamente il caso più comune
	// a rompersi. Il contratto è "array, eventualmente vuoto" — mai null.
	m := &model.ProgressMetrics{
		WeekGoal:   weekGoal,
		PRs:        []model.PR{},
		BodyWeight: model.BodyWeightStats{Series: []float64{}},
	}

	// streak
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT DATE(completed_at) AS d
		 FROM session_logs WHERE user_id = $1
		 ORDER BY d DESC LIMIT 60`,
		userID)
	if err != nil {
		return nil, err
	}
	var dates []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	rows.Close()
	m.Streak = calcStreak(dates)

	// week sessions
	_ = r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_logs
		 WHERE user_id = $1 AND completed_at >= date_trunc('week', NOW())`,
		userID,
	).Scan(&m.WeekSessions)

	// weekly volume (this week vs last)
	var thisVol, lastVol float64
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(sl.weight * sl.reps), 0)
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1
		   AND sess.completed_at >= date_trunc('week', NOW())`,
		userID,
	).Scan(&thisVol)
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(sl.weight * sl.reps), 0)
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1
		   AND sess.completed_at >= date_trunc('week', NOW()) - INTERVAL '1 week'
		   AND sess.completed_at < date_trunc('week', NOW())`,
		userID,
	).Scan(&lastVol)
	var volDelta float64
	if lastVol > 0 {
		volDelta = ((thisVol - lastVol) / lastVol) * 100
	}
	m.Volume = model.VolumeStats{
		Value:  roundTo1(thisVol / 1000),
		Unit:   "k kg",
		Delta:  roundTo1(volDelta),
		Series: []float64{},
	}

	// est 1RM — best across bench/squat/deadlift
	var best1RM float64
	var best1RMLift string
	for _, lift := range []string{"Barbell Bench Press", "Back Squat", "Deadlift"} {
		var v float64
		_ = r.db.QueryRow(ctx,
			`SELECT COALESCE(MAX(sl.weight * (1 + sl.reps::float / 30)), 0)
			 FROM set_logs sl
			 JOIN session_logs sess ON sess.id = sl.session_id
			 JOIN exercises e ON e.id = sl.exercise_id
			 WHERE sess.user_id = $1 AND e.name = $2`,
			userID, lift,
		).Scan(&v)
		if v > best1RM {
			best1RM = v
			best1RMLift = lift
		}
	}
	m.Est1RM = model.Est1RMStats{
		Lift:   best1RMLift,
		Value:  roundTo1(best1RM),
		Unit:   "kg",
		Series: []float64{},
	}

	// body weight
	bwRows, err := r.db.Query(ctx,
		`SELECT weight, unit FROM body_weight_logs
		 WHERE user_id = $1 ORDER BY logged_at DESC LIMIT 7`,
		userID)
	if err != nil {
		return nil, err
	}
	var weights []float64
	var unit string
	for bwRows.Next() {
		var w float64
		if err := bwRows.Scan(&w, &unit); err != nil {
			return nil, err
		}
		weights = append(weights, w)
	}
	bwRows.Close()
	if len(weights) > 0 {
		var delta float64
		if len(weights) >= 2 {
			delta = roundTo1(weights[0] - weights[len(weights)-1])
		}
		// reverse so series is oldest→newest
		series := make([]float64, len(weights))
		for i, w := range weights {
			series[len(weights)-1-i] = w
		}
		m.BodyWeight = model.BodyWeightStats{
			Value:  weights[0],
			Unit:   unit,
			Delta:  delta,
			Series: series,
		}
	}

	// PRs
	prRows, err := r.db.Query(ctx,
		`SELECT sl.exercise_name, sl.weight, sl.reps, sess.completed_at
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1 AND sl.is_pr = TRUE
		 ORDER BY sess.completed_at DESC LIMIT 5`,
		userID)
	if err != nil {
		return nil, err
	}
	for prRows.Next() {
		var name string
		var weight float64
		var reps int
		var when time.Time
		if err := prRows.Scan(&name, &weight, &reps, &when); err != nil {
			return nil, err
		}
		m.PRs = append(m.PRs, model.PR{
			Lift:       name,
			Weight:     weight,
			Reps:       reps,
			AchievedAt: when,
			// "Recente" = entro due giorni. Resta una soglia del server perché
			// è la stessa che decide cosa contare come PR nuovo su Home.
			Fresh: time.Since(when) < 48*time.Hour,
		})
	}
	prRows.Close()

	return m, nil
}

func calcStreak(dates []time.Time) int {
	if len(dates) == 0 {
		return 0
	}
	today := time.Now().Truncate(24 * time.Hour)
	streak := 0
	expected := today
	for _, d := range dates {
		day := d.Truncate(24 * time.Hour)
		if day.Equal(expected) || day.Equal(expected.Add(-24*time.Hour)) {
			if day.Equal(expected.Add(-24 * time.Hour)) {
				expected = day
			}
			streak++
			expected = expected.Add(-24 * time.Hour)
		} else {
			break
		}
	}
	return streak
}

func roundTo1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// pgx helpers
var _ = pgx.ErrNoRows
