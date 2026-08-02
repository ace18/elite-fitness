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

// La lista "Recenti" della Home. Il volume si ricalcola dalle serie, non si
// legge da session_logs.total_volume: quel campo lo manda il client ed è
// nullable, quindi una sessione arrivata dalla coda offline potrebbe averlo
// vuoto e la Home mostrerebbe 0 kg per un allenamento vero.
func TestProgressMetricsRecentSessions(t *testing.T) {
	pool := testPool(t)
	sessions := repository.NewSessionRepo(pool)
	ctx := context.Background()

	userID := makeUser(t, pool, "recent-"+time.Now().Format("150405.000000")+"@example.com")
	exID := exerciseID(t, pool)

	// Tre sessioni in giorni diversi, inserite fuori ordine per verificare che
	// sia la query a ordinarle.
	type seed struct {
		name    string
		daysAgo int
		weight  float64
		reps    int
		sets    int
	}
	for _, s := range []seed{
		{"Media", 3, 50, 10, 2},   // 1000
		{"Recente", 1, 100, 5, 2}, // 1000
		{"Vecchia", 10, 20, 5, 1}, // 100
	} {
		var sessionID string
		if err := pool.QueryRow(ctx,
			`INSERT INTO session_logs (user_id, name, completed_at, total_volume)
			 VALUES ($1,$2,$3,NULL) RETURNING id`,
			userID, s.name, time.Now().AddDate(0, 0, -s.daysAgo)).Scan(&sessionID); err != nil {
			t.Fatalf("sessione %s: %v", s.name, err)
		}
		for i := 0; i < s.sets; i++ {
			if _, err := pool.Exec(ctx,
				`INSERT INTO set_logs (session_id, exercise_id, exercise_name, set_number, weight, reps)
				 VALUES ($1,$2,'X',$3,$4,$5)`,
				sessionID, exID, i+1, s.weight, s.reps); err != nil {
				t.Fatalf("serie: %v", err)
			}
		}
	}

	m, err := sessions.GetProgressMetrics(ctx, userID, 3)
	if err != nil {
		t.Fatalf("GetProgressMetrics: %v", err)
	}

	if len(m.Recent) != 3 {
		t.Fatalf("sessioni recenti = %d, attese 3", len(m.Recent))
	}
	// Il contatore della schermata Profilo conta da sempre, non la settimana:
	// "Vecchia" è di dieci giorni fa e deve rientrare comunque.
	if m.TotalSessions != 3 {
		t.Errorf("allenamenti totali = %d, attesi 3", m.TotalSessions)
	}
	if m.TotalSessions < m.WeekSessions {
		t.Errorf("totali (%d) < settimana (%d): i due contatori sono invertiti",
			m.TotalSessions, m.WeekSessions)
	}
	// Dalla più recente alla più vecchia.
	want := []string{"Recente", "Media", "Vecchia"}
	for i, n := range want {
		if m.Recent[i].Name != n {
			t.Errorf("posizione %d = %q, attesa %q — l'ordine non è dal più recente", i, m.Recent[i].Name, n)
		}
	}
	// total_volume era NULL su tutte: se il volume arrivasse da lì sarebbe 0.
	if m.Recent[0].Volume != 1000 {
		t.Errorf("volume = %v, atteso 1000 — non è ricalcolato dalle serie", m.Recent[0].Volume)
	}
	if m.Recent[2].Volume != 100 {
		t.Errorf("volume della più vecchia = %v, atteso 100", m.Recent[2].Volume)
	}
	if m.Recent[0].CompletedAt.IsZero() {
		t.Error("completedAt vuoto: il client non può formattare la data relativa")
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
