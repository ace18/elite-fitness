package repository

import (
	"context"
	"errors"
	"time"

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
	// Vuota, non nil: una slice nil diventa `null` in JSON e il client ci fa
	// sopra .map()/.reduce(). Il contratto è sempre un array.
	templates := []model.PlanTemplate{}
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
		        total_weeks, is_active, started_at
		 FROM user_programs WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY started_at DESC LIMIT 1`,
		userID,
	).Scan(&p.ID, &p.UserID, &p.TemplateID, &p.Name, &p.Goal, &p.Level,
		&p.DaysPerWeek, &p.TotalWeeks, &p.IsActive, &p.StartedAt)
	if err != nil {
		return nil, err
	}
	// Calcolata, non letta dal database: vedi UserProgram.WeekAt.
	p.CurrentWeek = p.WeekAt(time.Now())
	return p, nil
}

// Archive chiude un programma portato a termine. A differenza di
// DeactivateAll segna anche completed_at, che è ciò che distingue "finito" da
// "sostituito con un altro piano".
func (r *ProgramRepo) Archive(ctx context.Context, programID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE user_programs
		 SET is_active = FALSE, completed_at = NOW()
		 WHERE id = $1 AND is_active = TRUE`,
		programID)
	return err
}

func (r *ProgramRepo) DeactivateAll(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE user_programs SET is_active = FALSE WHERE user_id = $1`, userID)
	return err
}

// CreateFromTemplate disattiva i programmi esistenti, crea quello nuovo e ci
// copia dentro i workout della template, blocchi compresi — tutto in una
// transazione, così non può restare un programma a metà senza allenamenti.
// Restituisce l'id del programma creato.
//
// Una template a blocco unico (tutte le righe a week_number = 1, com'era prima
// della 009) si copia identica a prima e si ripete per tutto il programma:
// GetWorkoutsForWeek ripiega sull'ultimo blocco definito.
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
	//
	// week_number arriva dalla template invece di essere 1 fisso: una template
	// periodizzata definisce un giorno per ogni blocco, e ogni blocco deve
	// finire nella settimana giusta del programma.
	//
	// Il join che riaggancia gli esercizi va per (day_of_week, week_number), non
	// per il solo giorno: con più blocchi lo stesso lunedì esiste in ciascuno, e
	// senza la settimana il join diventa un prodotto cartesiano che infila gli
	// esercizi di ogni blocco in tutti gli altri.
	if _, err := tx.Exec(ctx, `
		WITH new_workouts AS (
		  INSERT INTO program_workouts
		    (program_id, name, focus, day_of_week, week_number, order_in_week)
		  SELECT $1, tw.name, tw.focus, tw.day_of_week, tw.week_number, tw.order_in_week
		  FROM template_workouts tw
		  WHERE tw.template_id = $2
		  RETURNING id, day_of_week, week_number
		)
		INSERT INTO program_workout_exercises
		  (workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
		SELECT nw.id, twe.exercise_id, twe.sets, twe.target_reps, twe.rest_seconds, twe.order_index
		FROM new_workouts nw
		JOIN template_workouts tw
		  ON tw.template_id = $2
		 AND tw.day_of_week = nw.day_of_week
		 AND tw.week_number = nw.week_number
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

// GetWorkoutsForWeek restituisce gli allenamenti che valgono per la settimana
// `week`.
//
// Le template definiscono una sola settimana, che si ripete: chiedere la
// settimana 5 di un programma che ne descrive solo una deve restituire quella,
// non il vuoto. Perciò si usa la settimana definita più recente fra quelle
// minori o uguali a `week`, invece del confronto esatto — che è anche il modo
// in cui funzionerebbe un programma periodizzato vero, dove una settimana di
// scarico definita alla 4 varrebbe fino alla successiva definita.
func (r *ProgramRepo) GetWorkoutsForWeek(ctx context.Context, programID string, week int) ([]model.ProgramWorkout, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, program_id, name, focus, day_of_week, week_number, order_in_week
		 FROM program_workouts
		 WHERE program_id = $1
		   AND week_number = COALESCE(
		         (SELECT MAX(week_number) FROM program_workouts
		          WHERE program_id = $1 AND week_number <= $2), 1)
		 ORDER BY order_in_week`,
		programID, week)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Vuota, non nil: una slice nil diventa `null` in JSON e il client ci fa
	// sopra .map()/.reduce(). Il contratto è sempre un array.
	workouts := []model.ProgramWorkout{}
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
		`SELECT pwe.id, pwe.exercise_id, e.name, e.muscle_group, e.category,
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
	// Vuota, non nil: una slice nil diventa `null` in JSON e il client ci fa
	// sopra .map()/.reduce(). Il contratto è sempre un array.
	exercises := []model.WorkoutExercise{}
	for rows.Next() {
		var ex model.WorkoutExercise
		if err := rows.Scan(&ex.ID, &ex.ExerciseID, &ex.Name, &ex.Muscle, &ex.Category,
			&ex.Sets, &ex.TargetReps, &ex.RestSeconds, &ex.OrderIndex); err != nil {
			return nil, err
		}
		exercises = append(exercises, ex)
	}
	return exercises, nil
}

// ExerciseHistory è quel che si sa di un esercizio al momento di proporre il
// carico del prossimo allenamento.
type ExerciseHistory struct {
	// Last è l'ultimo peso sollevato a ripetizioni confrontabili con quelle di
	// oggi, con il suo RPE. Zero se a questo schema non si è mai lavorato — il
	// caso della prima seduta di un blocco nuovo.
	Last    float64
	LastRPE *float64

	// Recent1RM è il massimale stimato sull'ultima seduta in cui l'esercizio è
	// comparso, a qualunque numero di ripetizioni. Serve da ponte quando Last
	// non c'è: da lì si ricava un peso per lo schema di oggi.
	Recent1RM float64

	// PR è il miglior massimale stimato di sempre, quello mostrato come record
	// da battere.
	PR float64
}

// GetExerciseHistory raccoglie lo storico che serve a proporre il carico.
//
// `targetReps` non è un dettaglio: il peso dell'ultima serie vuol dire qualcosa
// solo se quella serie somigliava a quella di oggi. Prima si prendeva l'ultima
// in assoluto, e finché i programmi erano una settimana sola ripetuta le
// ripetizioni non cambiavano mai, quindi la domanda non si poneva. Con i
// blocchi (migration 010) si passa per esempio da 5×5 a 5×3: l'ultimo peso è
// quello di un cinque, proporlo per un triplo lo sottostima di parecchio.
//
// Quindi si cerca l'ultima serie nella stessa fascia di ripetizioni (vedi
// RepBandBounds). Alla prima seduta di un blocco nuovo non ce n'è ancora una, e
// lì subentra Recent1RM.
func (r *ProgramRepo) GetExerciseHistory(ctx context.Context, userID, exerciseID string, targetReps int) (ExerciseHistory, error) {
	var h ExerciseHistory
	loReps, hiReps := RepBandBounds(targetReps)

	err := r.db.QueryRow(ctx,
		`SELECT sl.weight, sl.rpe
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1 AND sl.exercise_id = $2
		   AND sl.reps BETWEEN $3 AND $4
		 ORDER BY sess.completed_at DESC, sl.set_number DESC
		 LIMIT 1`,
		userID, exerciseID, loReps, hiReps,
	).Scan(&h.Last, &h.LastRPE)
	if err != nil {
		// Nessuna serie confrontabile: non è un errore, è il primo giorno di un
		// blocco (o dell'esercizio).
		h.Last, h.LastRPE = 0, nil
	}

	// Il massimale recente si stima sull'ultima seduta intera, non sull'ultima
	// serie: con un top set seguito da serie in scarico l'ultima serie è la più
	// leggera, e prenderla farebbe sembrare che l'atleta sia calato.
	//
	// Le ripetizioni entrano nella formula di Epley limitate a 12: sopra quel
	// numero la stima si gonfia in fretta, e siccome da qui esce un peso da
	// mettere sul bilanciere conviene sbagliare per difetto.
	if err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(sl.weight * (1 + LEAST(sl.reps, 12)::float / 30)), 0)
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1 AND sl.exercise_id = $2
		   AND sess.completed_at = (
		         SELECT MAX(s2.completed_at)
		         FROM set_logs l2
		         JOIN session_logs s2 ON s2.id = l2.session_id
		         WHERE s2.user_id = $1 AND l2.exercise_id = $2)`,
		userID, exerciseID,
	).Scan(&h.Recent1RM); err != nil {
		return h, err
	}

	// Il PR resta sulle ripetizioni vere e su tutto lo storico: è un record da
	// mostrare, non un peso da caricare, e limitarlo lo farebbe scendere sotto
	// quello che l'utente ha già visto.
	if err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(weight * (1 + reps::float / 30)), 0)
		 FROM set_logs sl
		 JOIN session_logs sess ON sess.id = sl.session_id
		 WHERE sess.user_id = $1 AND sl.exercise_id = $2`,
		userID, exerciseID,
	).Scan(&h.PR); err != nil {
		return h, err
	}
	return h, nil
}

// RepBandBounds dà l'intervallo di ripetizioni confrontabili con `reps`.
//
// Le fasce seguono il modo in cui si allena, non una percentuale: sotto le tre
// ripetizioni si è nella forza massimale, intorno alle dieci nell'ipertrofia,
// sopra le quindici nella resistenza. Due serie nella stessa fascia si fanno a
// carichi simili, due in fasce diverse no — ed è esattamente la differenza fra
// un blocco e il successivo.
func RepBandBounds(reps int) (lo, hi int) {
	switch {
	case reps <= 3:
		return 1, 3
	case reps <= 6:
		return 4, 6
	case reps <= 10:
		return 7, 10
	case reps <= 15:
		return 11, 15
	default:
		// Senza limite superiore: a ripetizioni molto alte il carico cambia
		// poco, quindi tenerle insieme non fa danni.
		return 16, 1 << 30
	}
}

// FindOrCreateExercise gira su q: passare la tx del chiamante quando fa parte
// di una transazione, altrimenti r.DB(). Usare il pool qui dentro mentre il
// chiamante è in transazione lascerebbe righe orfane in caso di rollback.
func (r *ProgramRepo) FindOrCreateExercise(ctx context.Context, q Querier, name, muscleGroup, category string) (string, error) {
	// La colonna ha un default 'compound', ma qui un valore non riconosciuto
	// arriverebbe comunque dritto in tabella: la categoria decide di quanto
	// sale il carico, e "" verrebbe letto come "non è un complementare".
	if category != "compound" && category != "isolation" {
		category = "compound"
	}

	var id string
	err := q.QueryRow(ctx,
		`INSERT INTO exercises (name, muscle_group, category)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING
		 RETURNING id`,
		name, muscleGroup, category,
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
