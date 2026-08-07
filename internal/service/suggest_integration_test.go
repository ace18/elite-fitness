package service_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/db"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Il passaggio di blocco visto da fuori, con str-5x5 vero: si chiude il blocco
// 5×5 a 100 kg, si entra nella settimana 5 (che è 5×3) e si guarda cosa viene
// proposto.
//
// Prima di questo lavoro la proposta erano i 100 kg dei cinque, al più 102.5
// con l'RPE — per dei tripli, cioè molto sotto quel che l'atleta reggeva. Il
// test fissa che la proposta ora venga stimata dal massimale e stia sopra il
// carico del blocco precedente.
func TestSuggestionCrossesBlockBoundary(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL non impostata — serve un Postgres di prova")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connessione: %v", err)
	}
	defer pool.Close()
	if _, err := db.Connect(ctx, url); err != nil {
		t.Fatalf("migrazioni: %v", err)
	}

	programs := repository.NewProgramRepo(pool)
	svc := service.NewWorkoutService(programs, repository.NewSessionRepo(pool))

	suffix := time.Now().Format("150405.000000")
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1) RETURNING id`,
		"blocco-"+suffix+"@example.com").Scan(&userID); err != nil {
		t.Fatal(err)
	}

	tmpl := templateNamed(t, programs, "str-5x5")
	programID, err := programs.CreateFromTemplate(ctx, userID, tmpl)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	// Il programma è partito quattro settimane fa: siamo nella settimana 5, la
	// prima del blocco a tripli.
	if _, err := pool.Exec(ctx,
		`UPDATE user_programs SET started_at = NOW() - INTERVAL '4 weeks' WHERE id = $1`,
		programID); err != nil {
		t.Fatal(err)
	}

	squatID := exerciseNamed(t, pool, "Back Squat")

	// Una seduta del blocco 5×5, chiusa a 100 kg per cinque con RPE 8: peso
	// giusto, l'autoregolazione da sola non lo muoverebbe.
	var sessionID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO session_logs (user_id, program_id, name, completed_at)
		 VALUES ($1,$2,'Strength A', NOW() - INTERVAL '3 days') RETURNING id`,
		userID, programID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	for setNo := 1; setNo <= 5; setNo++ {
		if _, err := pool.Exec(ctx,
			`INSERT INTO set_logs (session_id, exercise_id, exercise_name, set_number, weight, reps, rpe)
			 VALUES ($1,$2,'Back Squat',$3,100,5,8)`, sessionID, squatID, setNo); err != nil {
			t.Fatal(err)
		}
	}

	workout, err := svc.BuildTodayWorkout(ctx, userID)
	if err != nil {
		t.Fatalf("BuildTodayWorkout: %v", err)
	}
	if workout == nil {
		t.Fatal("nessun allenamento proposto")
	}

	var squat *struct {
		reps      int
		suggested float64
		last      float64
	}
	for _, ex := range workout.Exercises {
		if ex.Name == "Back Squat" {
			squat = &struct {
				reps      int
				suggested float64
				last      float64
			}{ex.TargetReps, ex.Suggested, ex.Last}
		}
	}
	if squat == nil {
		t.Fatalf("Back Squat non è nell'allenamento proposto: %+v", workout.Exercises)
	}

	// Siamo davvero nel blocco dei tripli, se no il test non prova niente.
	if squat.reps != 3 {
		t.Fatalf("target = %d ripetizioni, mi aspettavo il blocco 5×3", squat.reps)
	}
	// Niente storico a tre ripetizioni: `last` resta vuoto invece di mostrare
	// il peso dei cinque, che verrebbe letto come "l'ultima volta hai fatto
	// questo" e non è vero per questo schema.
	if squat.last != 0 {
		t.Errorf("last = %v, volevo vuoto: a 3 ripetizioni non si è mai lavorato", squat.last)
	}
	// 100 kg × 5 -> 1RM stimato ~116.7 -> per un triplo ~105.
	if squat.suggested != 105 {
		t.Errorf("proposta = %v kg, volevo 105", squat.suggested)
	}
	if squat.suggested <= 102.5 {
		t.Errorf("proposta = %v kg: è il comportamento vecchio, il peso dei cinque", squat.suggested)
	}
}

// Dentro il blocco, invece, non deve cambiare niente rispetto a prima: c'è una
// serie confrontabile e si autoregola su quella.
func TestSuggestionStaysAutoregulatedInsideABlock(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL non impostata — serve un Postgres di prova")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connessione: %v", err)
	}
	defer pool.Close()
	if _, err := db.Connect(ctx, url); err != nil {
		t.Fatalf("migrazioni: %v", err)
	}

	programs := repository.NewProgramRepo(pool)
	svc := service.NewWorkoutService(programs, repository.NewSessionRepo(pool))

	suffix := time.Now().Format("150405.000000")
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1) RETURNING id`,
		"dentro-"+suffix+"@example.com").Scan(&userID); err != nil {
		t.Fatal(err)
	}

	programID, err := programs.CreateFromTemplate(ctx, userID, templateNamed(t, programs, "str-5x5"))
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	squatID := exerciseNamed(t, pool, "Back Squat")

	// Settimana 1, blocco 5×5. Ultima serie a 100 kg per cinque con RPE 6.5:
	// facile, quindi si sale di uno scatto.
	var sessionID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO session_logs (user_id, program_id, name, completed_at)
		 VALUES ($1,$2,'Strength A', NOW() - INTERVAL '2 days') RETURNING id`,
		userID, programID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO set_logs (session_id, exercise_id, exercise_name, set_number, weight, reps, rpe)
		 VALUES ($1,$2,'Back Squat',1,100,5,6.5)`, sessionID, squatID); err != nil {
		t.Fatal(err)
	}

	workout, err := svc.BuildTodayWorkout(ctx, userID)
	if err != nil {
		t.Fatalf("BuildTodayWorkout: %v", err)
	}
	if workout == nil {
		t.Fatal("nessun allenamento proposto")
	}
	for _, ex := range workout.Exercises {
		if ex.Name != "Back Squat" {
			continue
		}
		if ex.TargetReps != 5 {
			t.Fatalf("target = %d ripetizioni, mi aspettavo il blocco 5×5", ex.TargetReps)
		}
		if ex.Last != 100 {
			t.Errorf("last = %v, volevo 100", ex.Last)
		}
		if ex.Suggested != 102.5 {
			t.Errorf("proposta = %v, volevo 102.5 (RPE 6.5 -> uno scatto in su)", ex.Suggested)
		}
		return
	}
	t.Fatal("Back Squat non è nell'allenamento proposto")
}

func templateNamed(t *testing.T, programs *repository.ProgramRepo, id string) model.PlanTemplate {
	t.Helper()
	templates, err := programs.GetTemplates(context.Background())
	if err != nil {
		t.Fatalf("lettura template: %v", err)
	}
	for _, candidate := range templates {
		if candidate.ID == id {
			return candidate
		}
	}
	t.Fatalf("template %q non è nel catalogo", id)
	return model.PlanTemplate{}
}

func exerciseNamed(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM exercises WHERE name = $1`, name).Scan(&id); err != nil {
		t.Fatalf("esercizio %q: %v", name, err)
	}
	return id
}

// La categoria deve arrivare fino in fondo: sta sulla tabella exercises, passa
// per GetExercisesForWorkout e decide il passo. Se si perdesse per strada
// l'alzata laterale tornerebbe a salire di 2.5 kg alla volta, cioè del 12%.
//
// ppl-hyp, Push Day A: 'Lateral Raise' è isolation nel catalogo (002).
func TestIsolationWorkUsesTheSmallIncrement(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL non impostata — serve un Postgres di prova")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connessione: %v", err)
	}
	defer pool.Close()
	if _, err := db.Connect(ctx, url); err != nil {
		t.Fatalf("migrazioni: %v", err)
	}

	programs := repository.NewProgramRepo(pool)
	svc := service.NewWorkoutService(programs, repository.NewSessionRepo(pool))

	suffix := time.Now().Format("150405.000000")
	var userID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ($1) RETURNING id`,
		"isolamento-"+suffix+"@example.com").Scan(&userID); err != nil {
		t.Fatal(err)
	}

	programID, err := programs.CreateFromTemplate(ctx, userID, templateNamed(t, programs, "ppl-hyp"))
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	// 20 kg per 15 con RPE 6.5: facile, quindi si sale — ma di 1.25, non di 2.5.
	var sessionID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO session_logs (user_id, program_id, name, completed_at)
		 VALUES ($1,$2,'Push Day A', NOW() - INTERVAL '3 days') RETURNING id`,
		userID, programID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO set_logs (session_id, exercise_id, exercise_name, set_number, weight, reps, rpe)
		 VALUES ($1,$2,'Lateral Raise',1,20,15,6.5)`,
		sessionID, exerciseNamed(t, pool, "Lateral Raise")); err != nil {
		t.Fatal(err)
	}

	workout, err := svc.BuildTodayWorkout(ctx, userID)
	if err != nil {
		t.Fatalf("BuildTodayWorkout: %v", err)
	}
	if workout == nil {
		t.Fatal("nessun allenamento proposto")
	}
	for _, ex := range workout.Exercises {
		if ex.Name != "Lateral Raise" {
			continue
		}
		if ex.Category != "isolation" {
			t.Fatalf("category = %q, volevo isolation: non è arrivata fin qui", ex.Category)
		}
		if ex.Suggested != 21.25 {
			t.Errorf("proposta = %v, volevo 21.25 (+1.25, non +2.5)", ex.Suggested)
		}
		return
	}
	t.Fatal("Lateral Raise non è nell'allenamento proposto")
}
