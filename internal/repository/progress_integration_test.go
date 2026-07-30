package repository_test

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/repository"
)

// In Go una slice nil si serializza come `null`, non come `[]`. Il client fa
// .filter() / .slice() / .reduce() su questi campi, quindi un `null` è un
// TypeError a schermo — e capita all'utente appena registrato, che è il caso
// più comune di tutti.
//
// È già successo due volte: prima con la serie dello Sparkline (rattoppata nel
// componente), poi con `prs` su Home e Progressi. Questo test guarda la forma
// del JSON invece di un singolo campo, così la prossima aggiunta che dimentica
// l'inizializzazione fallisce qui e non nel browser.
func TestProgressMetricsNeverSerializesNullArrays(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewSessionRepo(pool)
	ctx := context.Background()

	// Utente nuovo: nessuna sessione, nessun PR, nessuna pesata. È lo stato in
	// cui tutte le slice restano vuote.
	userID := makeUser(t, pool, "nulls-"+time.Now().Format("150405.000000")+"@example.com")

	m, err := repo.GetProgressMetrics(ctx, userID, 3)
	if err != nil {
		t.Fatalf("GetProgressMetrics: %v", err)
	}

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	// Qualsiasi campo che vale `null` in questa risposta è sospetto: sono tutti
	// array o numeri.
	nulls := regexp.MustCompile(`"([a-zA-Z0-9_]+)":\s*null`).FindAllStringSubmatch(body, -1)
	if len(nulls) > 0 {
		var names []string
		for _, m := range nulls {
			names = append(names, m[1])
		}
		t.Errorf("campi null nella risposta /api/progress: %s\nil client ci chiama .filter()/.slice() sopra e va in TypeError\nJSON: %s",
			strings.Join(names, ", "), body)
	}

	// Controlli espliciti sui campi che hanno già rotto l'app, così il
	// messaggio d'errore dice quale.
	if m.PRs == nil {
		t.Error("PRs è nil: rompe home/+page.svelte (p.prs.filter) e progress/+page.svelte (p.prs.slice)")
	}
	if m.BodyWeight.Series == nil {
		t.Error("BodyWeight.Series è nil: lo Sparkline si difende, ma il contratto resta sbagliato")
	}
	if m.Volume.Series == nil {
		t.Error("Volume.Series è nil")
	}
	if m.Est1RM.Series == nil {
		t.Error("Est1RM.Series è nil")
	}
}

// Stessa classe di bug sul percorso dell'allenamento: TodayWorkout.Exercises non
// ha omitempty, quindi una slice nil arriva al client come `exercises: null` e
// train/+page.svelte fa .reduce() su quello.
func TestWorkoutExercisesNeverNil(t *testing.T) {
	pool := testPool(t)
	programs := repository.NewProgramRepo(pool)
	ctx := context.Background()

	// Un workout senza esercizi: caso limite, ma è quello che produce il nil.
	userID := makeUser(t, pool, "noex-"+time.Now().Format("150405.000000")+"@example.com")
	var programID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO user_programs (user_id, name, goal, level, days_per_week, total_weeks, is_active, started_at)
		 VALUES ($1,'Vuoto','strength','beginner',3,4,TRUE,NOW()) RETURNING id`, userID).Scan(&programID); err != nil {
		t.Fatalf("programma: %v", err)
	}
	var workoutID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO program_workouts (program_id, name, focus, day_of_week, order_in_week)
		 VALUES ($1,'Senza esercizi','none',0,0) RETURNING id`, programID).Scan(&workoutID); err != nil {
		t.Fatalf("workout: %v", err)
	}

	exercises, err := programs.GetExercisesForWorkout(ctx, workoutID)
	if err != nil {
		t.Fatalf("GetExercisesForWorkout: %v", err)
	}
	if exercises == nil {
		t.Fatal("slice nil: diventa `exercises: null` e train/+page.svelte:50 fa .reduce() su null")
	}

	raw, _ := json.Marshal(map[string]any{"exercises": exercises})
	if strings.Contains(string(raw), "null") {
		t.Errorf("serializzato come null: %s", raw)
	}
}
