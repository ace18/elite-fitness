package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/elitecoach/backend/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrExerciseExists — esercizi.name è UNIQUE dalla migrazione 003, ed è quel
	// vincolo a far funzionare gli ON CONFLICT di FindOrCreateExercise.
	ErrExerciseExists = errors.New("exercise already exists")
	// ErrExerciseInUse — referenziato da una template o dallo storico.
	ErrExerciseInUse = errors.New("exercise is referenced")
	// ErrTemplateExists — l'identificativo è già preso.
	ErrTemplateExists = errors.New("template already exists")
	// ErrTemplateInUse — esistono programmi nati da questa template.
	ErrTemplateInUse = errors.New("template has programs")
	// ErrDayExists — quel giorno esiste già in quel blocco.
	ErrDayExists = errors.New("day already defined in this block")
)

type CatalogRepo struct {
	db *pgxpool.Pool
}

func NewCatalogRepo(db *pgxpool.Pool) *CatalogRepo { return &CatalogRepo{db: db} }

// ---- esercizi --------------------------------------------------------------

// ListExercises restituisce la libreria con quanto ogni voce è usata.
//
// I conteggi arrivano da due sottoquery correlate invece che da altrettanti giri
// per riga: la libreria è di qualche centinaio di voci e la pagina si disegna
// tutta in una volta.
func (r *CatalogRepo) ListExercises(ctx context.Context) ([]model.Exercise, error) {
	rows, err := r.db.Query(ctx,
		`SELECT e.id, e.name, e.muscle_group, e.category,
		        (SELECT count(*) FROM template_workout_exercises t WHERE t.exercise_id = e.id),
		        (SELECT count(*) FROM set_logs s WHERE s.exercise_id = e.id)
		 FROM exercises e
		 ORDER BY e.muscle_group, e.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Exercise
	for rows.Next() {
		var e model.Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.MuscleGroup, &e.Category,
			&e.InTemplates, &e.InHistory); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *CatalogRepo) FindExercise(ctx context.Context, id string) (*model.Exercise, error) {
	e := &model.Exercise{}
	err := r.db.QueryRow(ctx,
		`SELECT e.id, e.name, e.muscle_group, e.category,
		        (SELECT count(*) FROM template_workout_exercises t WHERE t.exercise_id = e.id),
		        (SELECT count(*) FROM set_logs s WHERE s.exercise_id = e.id)
		 FROM exercises e WHERE e.id = $1`, id,
	).Scan(&e.ID, &e.Name, &e.MuscleGroup, &e.Category, &e.InTemplates, &e.InHistory)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *CatalogRepo) CreateExercise(ctx context.Context, name, muscle, category string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`INSERT INTO exercises (name, muscle_group, category) VALUES ($1, $2, $3) RETURNING id`,
		strings.TrimSpace(name), strings.TrimSpace(muscle), normalizeCategory(category),
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrExerciseExists
		}
		return "", err
	}
	return id, nil
}

// UpdateExercise cambia i dati di un esercizio già in libreria.
//
// Rinominarlo si può anche se è già stato usato: set_logs porta con sé
// exercise_name, copiato al momento della registrazione, quindi lo storico
// continua a dire il nome che l'esercizio aveva quel giorno. È la scelta giusta
// — un allenamento fatto va raccontato com'era — ma va saputa, perché significa
// che dopo una rinomina lo storico e la libreria non combaciano più.
func (r *CatalogRepo) UpdateExercise(ctx context.Context, id, name, muscle, category string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE exercises SET name = $2, muscle_group = $3, category = $4 WHERE id = $1`,
		id, strings.TrimSpace(name), strings.TrimSpace(muscle), normalizeCategory(category))
	if err != nil {
		if isUniqueViolation(err) {
			return ErrExerciseExists
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("esercizio inesistente")
	}
	return nil
}

// DeleteExercise toglie una voce dalla libreria, se nessuno la usa.
//
// Il controllo è nella WHERE e non prima: fra un conteggio e una DELETE separati
// ci sta una richiesta che intanto aggiunge l'esercizio a una template, e la
// cancellazione fallirebbe comunque — ma con un errore del database invece che
// con una frase leggibile. Così l'esito è uno solo e viene deciso in un colpo.
func (r *CatalogRepo) DeleteExercise(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM exercises e
		 WHERE e.id = $1
		   AND NOT EXISTS (SELECT 1 FROM template_workout_exercises t WHERE t.exercise_id = e.id)
		   AND NOT EXISTS (SELECT 1 FROM set_logs s WHERE s.exercise_id = e.id)
		   AND NOT EXISTS (SELECT 1 FROM program_workout_exercises p WHERE p.exercise_id = e.id)`,
		id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrExerciseInUse
	}
	return nil
}

// normalizeCategory — solo 'compound' o 'isolation'. Qualsiasi altra cosa
// diventa compound, che è l'incremento di carico più prudente (2,5 kg invece di
// 1,25): sbagliare per eccesso su un complementare si nota subito, sbagliare per
// difetto su un fondamentale blocca la progressione senza dirlo.
func normalizeCategory(c string) string {
	if strings.TrimSpace(strings.ToLower(c)) == "isolation" {
		return "isolation"
	}
	return "compound"
}

// ---- template --------------------------------------------------------------

// ListTemplates — il catalogo completo per il pannello, archiviate comprese.
func (r *CatalogRepo) ListTemplates(ctx context.Context) ([]model.TemplateSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.id, t.name, t.goal, t.focus, t.level, t.days_per_week, t.session_min,
		        t.total_weeks, t.glyph, t.tag, t.min_one_rm_kg, t.max_one_rm_kg, t.archived_at,
		        (SELECT count(*) FROM template_workouts w WHERE w.template_id = t.id),
		        (SELECT count(DISTINCT w.week_number) FROM template_workouts w WHERE w.template_id = t.id),
		        (SELECT count(*) FROM user_programs p WHERE p.template_id = t.id)
		 FROM plan_templates t
		 ORDER BY t.archived_at IS NOT NULL, t.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.TemplateSummary
	for rows.Next() {
		var t model.TemplateSummary
		if err := rows.Scan(&t.ID, &t.Name, &t.Goal, &t.Focus, &t.Level, &t.DaysPerWeek,
			&t.SessionMin, &t.TotalWeeks, &t.Glyph, &t.Tag, &t.MinOneRM, &t.MaxOneRM,
			&t.ArchivedAt, &t.Days, &t.Blocks, &t.InUse); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *CatalogRepo) FindTemplate(ctx context.Context, id string) (*model.TemplateSummary, error) {
	t := &model.TemplateSummary{}
	err := r.db.QueryRow(ctx,
		`SELECT t.id, t.name, t.goal, t.focus, t.level, t.days_per_week, t.session_min,
		        t.total_weeks, t.glyph, t.tag, t.min_one_rm_kg, t.max_one_rm_kg, t.archived_at,
		        (SELECT count(*) FROM template_workouts w WHERE w.template_id = t.id),
		        (SELECT count(DISTINCT w.week_number) FROM template_workouts w WHERE w.template_id = t.id),
		        (SELECT count(*) FROM user_programs p WHERE p.template_id = t.id)
		 FROM plan_templates t WHERE t.id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Goal, &t.Focus, &t.Level, &t.DaysPerWeek, &t.SessionMin,
		&t.TotalWeeks, &t.Glyph, &t.Tag, &t.MinOneRM, &t.MaxOneRM, &t.ArchivedAt,
		&t.Days, &t.Blocks, &t.InUse)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// TemplateInput — i campi modificabili di una template. Raggruppati in una
// struct perché sono tanti e passarli come parametri posizionali significa
// prima o poi scambiare goal con focus senza che il compilatore se ne accorga:
// sono entrambi stringhe.
type TemplateInput struct {
	ID          string
	Name        string
	Goal        string
	Focus       string
	Level       string
	DaysPerWeek int
	SessionMin  int
	TotalWeeks  int
	Glyph       string
	Tag         *string
	MinOneRM    *float64
	MaxOneRM    *float64
}

func (r *CatalogRepo) CreateTemplate(ctx context.Context, in TemplateInput) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO plan_templates
		   (id, name, goal, focus, level, days_per_week, session_min, total_weeks, glyph, tag,
		    min_one_rm_kg, max_one_rm_kg)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		in.ID, in.Name, in.Goal, in.Focus, in.Level, in.DaysPerWeek, in.SessionMin,
		in.TotalWeeks, in.Glyph, in.Tag, in.MinOneRM, in.MaxOneRM)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrTemplateExists
		}
		return err
	}
	return nil
}

// UpdateTemplate cambia i dati di una template. L'identificativo non si tocca:
// è la chiave che i programmi già creati referenziano.
//
// Le modifiche non arrivano ai programmi già in corso — sono copie fatte al
// momento dell'assegnazione (vedi CreateFromTemplate) — e vale solo per quelli
// nuovi. È il comportamento voluto: cambiare una template non deve riscrivere
// l'allenamento di domani a chi sta già seguendo quel piano.
func (r *CatalogRepo) UpdateTemplate(ctx context.Context, in TemplateInput) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE plan_templates
		 SET name = $2, goal = $3, focus = $4, level = $5, days_per_week = $6,
		     session_min = $7, total_weeks = $8, glyph = $9, tag = $10,
		     min_one_rm_kg = $11, max_one_rm_kg = $12
		 WHERE id = $1`,
		in.ID, in.Name, in.Goal, in.Focus, in.Level, in.DaysPerWeek, in.SessionMin,
		in.TotalWeeks, in.Glyph, in.Tag, in.MinOneRM, in.MaxOneRM)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("template inesistente")
	}
	return nil
}

func (r *CatalogRepo) SetTemplateArchived(ctx context.Context, id string, archived bool) error {
	var err error
	if archived {
		_, err = r.db.Exec(ctx, `UPDATE plan_templates SET archived_at = NOW() WHERE id = $1`, id)
	} else {
		_, err = r.db.Exec(ctx, `UPDATE plan_templates SET archived_at = NULL WHERE id = $1`, id)
	}
	return err
}

// DeleteTemplate cancella una template mai usata. I giorni e le loro righe se ne
// vanno da soli (ON DELETE CASCADE dalla 004).
func (r *CatalogRepo) DeleteTemplate(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM plan_templates t
		 WHERE t.id = $1
		   AND NOT EXISTS (SELECT 1 FROM user_programs p WHERE p.template_id = t.id)`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTemplateInUse
	}
	return nil
}

// ---- giorni ----------------------------------------------------------------

// ListDays restituisce i giorni di una template raggruppati per blocco.
//
// Una query sola con gli esercizi appesi, invece di una per giorno: una template
// periodizzata di dodici settimane ha una quindicina di giorni, e quindici
// query per disegnare una pagina sono quindici occasioni di dimenticarsene una.
func (r *CatalogRepo) ListDays(ctx context.Context, templateID string) ([]model.TemplateBlock, error) {
	rows, err := r.db.Query(ctx,
		`SELECT w.id, w.name, w.focus, w.day_of_week, w.week_number, w.order_in_week
		 FROM template_workouts w
		 WHERE w.template_id = $1
		 ORDER BY w.week_number, w.order_in_week, w.day_of_week`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []model.TemplateDay
	for rows.Next() {
		d := model.TemplateDay{TemplateID: templateID}
		if err := rows.Scan(&d.ID, &d.Name, &d.Focus, &d.DayOfWeek,
			&d.WeekNumber, &d.OrderInWeek); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Il numero di esercizi per giorno serve nell'elenco. Una query sola per
	// tutta la template, poi si distribuisce in memoria.
	counts, err := r.exerciseCounts(ctx, templateID)
	if err != nil {
		return nil, err
	}

	var blocks []model.TemplateBlock
	for _, d := range days {
		d.Exercises = make([]model.TemplateExercise, counts[d.ID])
		if n := len(blocks); n > 0 && blocks[n-1].WeekNumber == d.WeekNumber {
			blocks[n-1].Days = append(blocks[n-1].Days, d)
			continue
		}
		blocks = append(blocks, model.TemplateBlock{WeekNumber: d.WeekNumber,
			Days: []model.TemplateDay{d}})
	}
	return blocks, nil
}

// exerciseCounts — quanti esercizi ha ciascun giorno. Restituisce solo il
// conteggio: l'elenco della template mostra "3 esercizi", non quali.
func (r *CatalogRepo) exerciseCounts(ctx context.Context, templateID string) (map[string]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT w.id, count(twe.id)
		 FROM template_workouts w
		 LEFT JOIN template_workout_exercises twe ON twe.template_workout_id = w.id
		 WHERE w.template_id = $1
		 GROUP BY w.id`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		counts[id] = n
	}
	return counts, rows.Err()
}

// FindDay legge un giorno con i suoi esercizi, in ordine.
func (r *CatalogRepo) FindDay(ctx context.Context, templateID, dayID string) (*model.TemplateDay, error) {
	d := &model.TemplateDay{}
	// template_id nella WHERE oltre all'id del giorno: un id di un'altra
	// template non deve aprirsi sotto l'intestazione di questa.
	err := r.db.QueryRow(ctx,
		`SELECT id, template_id, name, focus, day_of_week, week_number, order_in_week
		 FROM template_workouts WHERE id = $1 AND template_id = $2`, dayID, templateID,
	).Scan(&d.ID, &d.TemplateID, &d.Name, &d.Focus, &d.DayOfWeek, &d.WeekNumber, &d.OrderInWeek)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT twe.id, twe.exercise_id, e.name, e.muscle_group,
		        twe.sets, twe.target_reps, twe.rest_seconds, twe.order_index,
		        twe.load_pct, twe.load_offset_kg
		 FROM template_workout_exercises twe
		 JOIN exercises e ON e.id = twe.exercise_id
		 WHERE twe.template_workout_id = $1
		 ORDER BY twe.order_index`, dayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var x model.TemplateExercise
		if err := rows.Scan(&x.ID, &x.ExerciseID, &x.ExerciseName, &x.MuscleGroup,
			&x.Sets, &x.TargetReps, &x.RestSeconds, &x.OrderIndex,
			&x.LoadPct, &x.LoadOffsetKg); err != nil {
			return nil, err
		}
		d.Exercises = append(d.Exercises, x)
	}
	return d, rows.Err()
}

// CreateDay aggiunge un giorno a un blocco.
//
// order_in_week si calcola da solo dalla coda del blocco: è l'ordine in cui gli
// allenamenti si propongono dentro la settimana (vedi BuildTodayWorkout, che
// sceglie il prossimo non ancora fatto), e chiederlo a chi compila il form
// significherebbe chiedergli di tenere il conto a mano.
func (r *CatalogRepo) CreateDay(ctx context.Context, templateID string, week, dayOfWeek int, name, focus string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`INSERT INTO template_workouts (template_id, name, focus, day_of_week, week_number, order_in_week)
		 VALUES ($1, $2, $3, $4, $5,
		         COALESCE((SELECT max(order_in_week) + 1 FROM template_workouts
		                   WHERE template_id = $1 AND week_number = $5), 0))
		 RETURNING id`,
		templateID, strings.TrimSpace(name), strings.TrimSpace(focus), dayOfWeek, week,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrDayExists
		}
		return "", err
	}
	return id, nil
}

func (r *CatalogRepo) UpdateDay(ctx context.Context, templateID, dayID, name, focus string, week, dayOfWeek int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE template_workouts
		 SET name = $3, focus = $4, week_number = $5, day_of_week = $6
		 WHERE id = $1 AND template_id = $2`,
		dayID, templateID, strings.TrimSpace(name), strings.TrimSpace(focus), week, dayOfWeek)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDayExists
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("giorno inesistente")
	}
	return nil
}

func (r *CatalogRepo) DeleteDay(ctx context.Context, templateID, dayID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM template_workouts WHERE id = $1 AND template_id = $2`, dayID, templateID)
	return err
}

// ---- righe di esercizio ----------------------------------------------------

// ExerciseRowInput — i campi di una riga di esercizio dentro un giorno.
type ExerciseRowInput struct {
	ExerciseID   string
	Sets         int
	TargetReps   int
	RestSeconds  int
	LoadPct      *float64
	LoadOffsetKg *float64
}

// AddExerciseRow accoda un esercizio in fondo al giorno.
//
// order_index si calcola qui dentro con la stessa istruzione che inserisce:
// la colonna ha un vincolo UNIQUE (template_workout_id, order_index), quindi
// leggere il massimo e inserire in due tempi vuol dire che due aggiunte
// contemporanee ricavano lo stesso numero e la seconda va in errore.
func (r *CatalogRepo) AddExerciseRow(ctx context.Context, dayID string, in ExerciseRowInput) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO template_workout_exercises
		   (template_workout_id, exercise_id, sets, target_reps, rest_seconds, order_index,
		    load_pct, load_offset_kg)
		 VALUES ($1, $2, $3, $4, $5,
		         COALESCE((SELECT max(order_index) + 1 FROM template_workout_exercises
		                   WHERE template_workout_id = $1), 0),
		         $6, $7)`,
		dayID, in.ExerciseID, in.Sets, in.TargetReps, in.RestSeconds,
		in.LoadPct, in.LoadOffsetKg)
	return err
}

func (r *CatalogRepo) UpdateExerciseRow(ctx context.Context, dayID, rowID string, in ExerciseRowInput) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE template_workout_exercises
		 SET exercise_id = $3, sets = $4, target_reps = $5, rest_seconds = $6,
		     load_pct = $7, load_offset_kg = $8
		 WHERE id = $1 AND template_workout_id = $2`,
		rowID, dayID, in.ExerciseID, in.Sets, in.TargetReps, in.RestSeconds,
		in.LoadPct, in.LoadOffsetKg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("riga inesistente")
	}
	return nil
}

func (r *CatalogRepo) DeleteExerciseRow(ctx context.Context, dayID, rowID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM template_workout_exercises WHERE id = $1 AND template_workout_id = $2`,
		rowID, dayID)
	return err
}

// MoveExerciseRow sposta una riga di una posizione, scambiandola con la vicina.
//
// Lo scambio è in una transazione e passa da un order_index temporaneo: il
// vincolo UNIQUE (template_workout_id, order_index) è immediato, non differito,
// quindi assegnare direttamente alla prima riga l'indice della seconda va in
// conflitto prima ancora che la seconda si sposti.
func (r *CatalogRepo) MoveExerciseRow(ctx context.Context, dayID, rowID string, up bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var idx int
	if err := tx.QueryRow(ctx,
		`SELECT order_index FROM template_workout_exercises
		 WHERE id = $1 AND template_workout_id = $2`, rowID, dayID).Scan(&idx); err != nil {
		return err
	}

	cmp, order := "<", "DESC"
	if !up {
		cmp, order = ">", "ASC"
	}
	var neighbourID string
	var neighbourIdx int
	err = tx.QueryRow(ctx, fmt.Sprintf(
		`SELECT id, order_index FROM template_workout_exercises
		 WHERE template_workout_id = $1 AND order_index %s $2
		 ORDER BY order_index %s LIMIT 1`, cmp, order), dayID, idx,
	).Scan(&neighbourID, &neighbourIdx)
	if err != nil {
		// Già in cima o in fondo: non è un errore, semplicemente non si muove.
		return nil
	}

	const parking = -1
	if _, err := tx.Exec(ctx,
		`UPDATE template_workout_exercises SET order_index = $2 WHERE id = $1`,
		rowID, parking); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE template_workout_exercises SET order_index = $2 WHERE id = $1`,
		neighbourID, idx); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE template_workout_exercises SET order_index = $2 WHERE id = $1`,
		rowID, neighbourIdx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
