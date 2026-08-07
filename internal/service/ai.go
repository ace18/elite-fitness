package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlanGenerator è la parte che parla con un modello. Tutto il resto del file —
// il prompt, lo schema del piano, il salvataggio — non sa da quale fornitore
// arrivi la risposta.
//
// Il confine sta qui e non più in basso perché è qui che i fornitori
// divergono davvero: il modo di chiedere una chiamata a strumento e di
// ricomporre lo stream cambia da uno all'altro, mentre il JSON che ne esce, una
// volta ricomposto, è lo stesso. Da RequestPlan in poi si lavora su
// aiPlanOutput e basta.
//
// Le implementazioni stanno in questo package, perché aiPlanOutput non è
// esportato: per aggiungerne una si affianca un file ai_<fornitore>.go.
type PlanGenerator interface {
	RequestPlan(ctx context.Context, input GeneratePlanInput) (aiPlanOutput, error)
}

// ErrPlanRefused è il rifiuto del modello, distinto da un errore di rete o di
// formato. Il client vede comunque un errore solo, ma nei log la differenza fra
// "non ha risposto" e "si è rifiutato" è la prima cosa che si vuole sapere.
var ErrPlanRefused = errors.New("il modello ha rifiutato di generare il piano")

type AIService struct {
	planner  PlanGenerator
	programs *repository.ProgramRepo
	db       *pgxpool.Pool
}

func NewAIService(planner PlanGenerator, programs *repository.ProgramRepo, db *pgxpool.Pool) *AIService {
	return &AIService{planner: planner, programs: programs, db: db}
}

type GeneratePlanInput struct {
	Goal   string `json:"goal"`
	Level  string `json:"level"`
	Days   int    `json:"days"`
	Length int    `json:"length"`
	Notes  string `json:"notes"`
}

type aiPlanOutput struct {
	Name        string      `json:"name"`
	Goal        string      `json:"goal"`
	Level       string      `json:"level"`
	TotalWeeks  int         `json:"totalWeeks"`
	DaysPerWeek int         `json:"daysPerWeek"`
	Workouts    []aiWorkout `json:"workouts"`
}

type aiWorkout struct {
	Name      string `json:"name"`
	Focus     string `json:"focus"`
	DayOfWeek int    `json:"dayOfWeek"`
	// WeekNumber è la settimana da cui questo allenamento entra in vigore, non
	// l'unica in cui vale: resta valido finché non ne arriva uno definito più
	// avanti (vedi GetWorkoutsForWeek). Un piano che non periodizza mette 1
	// ovunque e si ripete per tutta la durata, come prima della 009.
	WeekNumber int          `json:"weekNumber"`
	Exercises  []aiExercise `json:"exercises"`
}

type aiExercise struct {
	Name        string `json:"name"`
	MuscleGroup string `json:"muscleGroup"`
	// Category distingue i fondamentali dai complementari. Non è cosmetica:
	// da lì si ricava di quanto alzare il carico fra una seduta e l'altra
	// (vedi loadIncrement). Senza, tutto quel che genera il modello finirebbe
	// sul default 'compound' della colonna e i complementari salirebbero a
	// scatti da 2.5 kg.
	Category string `json:"category"`

	Sets        int `json:"sets"`
	TargetReps  int `json:"targetReps"`
	RestSeconds int `json:"restSeconds"`
}

// Nome e descrizione dello strumento che il modello deve chiamare. La chiamata
// a strumento con schema JSON ce l'hanno tutti i fornitori interessanti,
// quindi questi valori si riusano tali e quali.
const (
	planToolName        = "create_training_plan"
	planToolDescription = "Create a complete structured weekly training program"
)

// planToolSchema è JSON Schema puro: lo stesso oggetto va bene come
// `input_schema` di Anthropic o come `function.parameters` di un'API in stile
// OpenAI. Chi implementa PlanGenerator lo incarta nel tipo del suo SDK senza
// riscriverlo.
func planToolSchema() (properties map[string]any, required []string) {
	return map[string]any{
			"name":        map[string]any{"type": "string"},
			"goal":        map[string]any{"type": "string"},
			"level":       map[string]any{"type": "string"},
			"totalWeeks":  map[string]any{"type": "integer"},
			"daysPerWeek": map[string]any{"type": "integer"},
			"workouts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"name", "focus", "dayOfWeek", "weekNumber", "exercises"},
					"properties": map[string]any{
						"name":      map[string]any{"type": "string"},
						"focus":     map[string]any{"type": "string"},
						"dayOfWeek": map[string]any{"type": "integer", "description": "0=Monday, 6=Sunday"},
						"weekNumber": map[string]any{
							"type": "integer",
							"description": "1-based week this workout takes effect FROM. It stays in " +
								"effect until another workout for the same day is defined at a later " +
								"week, so a block is described once and repeats. Use 1 for a program " +
								"with no periodization.",
						},
						"exercises": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"name", "muscleGroup", "category", "sets", "targetReps", "restSeconds"},
								"properties": map[string]any{
									"name":        map[string]any{"type": "string"},
									"muscleGroup": map[string]any{"type": "string"},
									"category": map[string]any{
										"type": "string",
										"enum": []string{"compound", "isolation"},
										"description": "compound for multi-joint lifts (squat, bench, row), " +
											"isolation for single-joint work (curls, lateral raises, pushdowns).",
									},
									"sets":        map[string]any{"type": "integer"},
									"targetReps":  map[string]any{"type": "integer"},
									"restSeconds": map[string]any{"type": "integer"},
								},
							},
						},
					},
				},
			},
		},
		[]string{"name", "goal", "level", "totalWeeks", "daysPerWeek", "workouts"}
}

// buildPlanPrompt è testo semplice, senza niente di specifico di un fornitore.
//
// Chiede blocchi, non settimane sciolte: una settimana definita vale finché non
// ne arriva un'altra, quindi un 12 settimane si descrive con due o tre blocchi
// invece che con dodici copie quasi identiche — che costerebbero token e
// uscirebbero comunque incoerenti fra loro.
func buildPlanPrompt(input GeneratePlanInput) string {
	return fmt.Sprintf(
		`You are an expert strength and conditioning coach.
Create a complete, evidence-based training program for:
- Goal: %s | Level: %s | Days/week: %d | Session length: %d min
%s
Generate a realistic, periodized program with appropriate exercise selection, volume, and rest periods.
Use standard barbell/dumbbell/cable/bodyweight exercises. DayOfWeek: 0=Monday, 1=Tuesday, ... 6=Sunday.
Pick %d consecutive weekdays starting Monday, and use the same weekdays in every block.

Structure the program as training BLOCKS, not as one week repeated and not as every
week spelled out. Each workout carries the weekNumber it takes effect from, and stays
in effect until the same weekday is defined again at a later week. So:
- Emit every training day once at weekNumber 1.
- Emit them again only at each week where the set/rep scheme actually changes — e.g.
  an accumulation block at week 1 and an intensification block partway through,
  moving from higher reps and moderate load toward lower reps and heavier load.
- Split totalWeeks into 2 or 3 blocks of a few weeks each; never more than 4 blocks,
  and no block may start at a week beyond totalWeeks.
- Keep exercise selection largely stable across blocks; it is the sets, reps and rest
  that periodize. Loads are autoregulated from the athlete's logged sets, so do not
  encode weights anywhere.`,
		input.Goal, input.Level, input.Days, input.Length,
		notesLine(input.Notes),
		input.Days,
	)
}

func (s *AIService) GeneratePlan(ctx context.Context, userID string, input GeneratePlanInput) (*model.UserProgram, error) {
	planData, err := s.planner.RequestPlan(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.savePlan(ctx, userID, planData)
}

func (s *AIService) savePlan(ctx context.Context, userID string, plan aiPlanOutput) (*model.UserProgram, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// deactivate existing programs
	if _, err := tx.Exec(ctx,
		`UPDATE user_programs SET is_active = FALSE WHERE user_id = $1`, userID); err != nil {
		return nil, err
	}

	var programID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO user_programs (user_id, name, goal, level, days_per_week, total_weeks)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		userID, plan.Name, plan.Goal, plan.Level, plan.DaysPerWeek, plan.TotalWeeks,
	).Scan(&programID); err != nil {
		return nil, err
	}

	layout := layoutWorkouts(plan.Workouts)

	for i, w := range plan.Workouts {
		var workoutID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO program_workouts (program_id, name, focus, day_of_week, week_number, order_in_week)
			 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			programID, w.Name, w.Focus, w.DayOfWeek, layout[i].Week, layout[i].OrderInWeek,
		).Scan(&workoutID); err != nil {
			return nil, err
		}

		for j, ex := range w.Exercises {
			exID, err := s.programs.FindOrCreateExercise(ctx, tx, ex.Name, ex.MuscleGroup, ex.Category)
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO program_workout_exercises
				 (workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
				 VALUES ($1,$2,$3,$4,$5,$6)`,
				workoutID, exID, ex.Sets, ex.TargetReps, ex.RestSeconds, j,
			); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.programs.GetActiveProgram(ctx, userID)
}

// workoutPlacement dice in che settimana e in che posizione va scritto un
// allenamento del piano.
type workoutPlacement struct {
	Week        int
	OrderInWeek int
}

// layoutWorkouts decide settimana e ordine di ogni allenamento restituito dal
// modello. Il risultato è parallelo a `workouts`.
//
// Due cose che la risposta del modello non garantisce da sola:
//
// La settimana viene portata ad almeno 1. Il campo è `required` nello schema,
// ma un modello che lo omettesse — o mandasse 0 — scriverebbe righe a settimana
// 0, che GetWorkoutsForWeek sceglierebbe comunque per la settimana 1 (0 <= 1).
// Il piano sembrerebbe funzionare finché qualcuno non definisce un blocco vero,
// e a quel punto il blocco 0 resterebbe appiccicato davanti.
//
// order_in_week riparte da zero a ogni blocco, perché è l'ordine dentro la
// settimana e GetWorkoutsForWeek ordina su un insieme già filtrato per
// settimana. Con un indice globale il secondo blocco avrebbe indici 3,4,5:
// funzionerebbe per caso, essendo comunque crescenti, ma il "terzo allenamento
// della settimana" avrebbe scritto 5.
func layoutWorkouts(workouts []aiWorkout) []workoutPlacement {
	placements := make([]workoutPlacement, len(workouts))
	nextOrder := map[int]int{}
	for i, w := range workouts {
		week := w.WeekNumber
		if week < 1 {
			week = 1
		}
		placements[i] = workoutPlacement{Week: week, OrderInWeek: nextOrder[week]}
		nextOrder[week]++
	}
	return placements
}

func notesLine(notes string) string {
	if notes == "" {
		return ""
	}
	return "Additional context: " + notes
}
