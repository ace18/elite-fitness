package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/db"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Test contro un Postgres vero: la logica di ON CONFLICT non è verificabile con
// un mock, ed è esattamente il punto in cui una regressione passerebbe
// inosservata (un duplicato in più non fa rumore, ma falsa volume, PR e
// archiviazione del programma).
//
//	TEST_DATABASE_URL="postgres://postgres:test@localhost:55432/elite?sslmode=disable" go test ./internal/repository/
//
// Senza la variabile il test si salta, così `go test ./...` resta eseguibile
// senza dipendenze esterne.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL non impostata — serve un Postgres di prova")
	}
	pool, err := db.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connessione: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func makeUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email) VALUES ($1) RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("creazione utente: %v", err)
	}
	return id
}

func countSets(t *testing.T, pool *pgxpool.Pool, sessionID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM set_logs WHERE session_id = $1`, sessionID).Scan(&n); err != nil {
		t.Fatalf("conteggio set: %v", err)
	}
	return n
}

func session(clientID *string, completedAt time.Time) *model.SessionLog {
	return &model.SessionLog{
		ClientSessionID: clientID,
		Name:            "Strength A",
		CompletedAt:     completedAt,
		Sets: []model.SetLog{
			{ExerciseName: "Back Squat", SetNumber: 1, Weight: 80, Reps: 5},
			{ExerciseName: "Back Squat", SetNumber: 2, Weight: 80, Reps: 5},
		},
	}
}

// exerciseID prende un esercizio reale dal catalogo: set_logs.exercise_id ha una
// FK, quindi un id inventato farebbe fallire l'insert per il motivo sbagliato.
func exerciseID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM exercises LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("nessun esercizio nel catalogo: %v", err)
	}
	return id
}

func TestSaveSessionIsIdempotentPerClientSessionID(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewSessionRepo(pool)
	ctx := context.Background()

	userID := makeUser(t, pool, "idem-"+time.Now().Format("150405.000000")+"@example.com")
	exID := exerciseID(t, pool)
	clientID := "client-" + time.Now().Format("150405.000000")

	first := session(&clientID, time.Now().Add(-2*time.Hour))
	first.UserID = userID
	for i := range first.Sets {
		first.Sets[i].ExerciseID = exID
	}

	inserted, err := repo.SaveSession(ctx, first)
	if err != nil {
		t.Fatalf("primo salvataggio: %v", err)
	}
	if !inserted {
		t.Fatal("il primo salvataggio dovrebbe inserire")
	}
	if first.ID == "" {
		t.Fatal("il primo salvataggio non ha restituito un id")
	}
	if n := countSets(t, pool, first.ID); n != 2 {
		t.Fatalf("set inseriti = %d, attesi 2", n)
	}

	// Stesso clientSessionId: è il ritentativo.
	retry := session(&clientID, time.Now().Add(-2*time.Hour))
	retry.UserID = userID
	for i := range retry.Sets {
		retry.Sets[i].ExerciseID = exID
	}

	inserted, err = repo.SaveSession(ctx, retry)
	if err != nil {
		t.Fatalf("ritentativo: %v", err)
	}
	if inserted {
		t.Error("il ritentativo non deve inserire una seconda sessione")
	}
	if retry.ID != first.ID {
		t.Errorf("il ritentativo ha restituito id %q, atteso quello esistente %q", retry.ID, first.ID)
	}
	// Il controllo che conta di più: le serie non devono essere state appese
	// alla sessione originale, altrimenti volume e PR raddoppiano in silenzio.
	if n := countSets(t, pool, first.ID); n != 2 {
		t.Errorf("dopo il ritentativo i set sono %d, attesi ancora 2 — le serie sono state duplicate", n)
	}

	var sessions int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM session_logs WHERE user_id = $1`, userID).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Errorf("sessioni per l'utente = %d, attesa 1", sessions)
	}
}

// I client che non mandano l'id devono continuare a funzionare: in Postgres due
// NULL non sono uguali, quindi l'indice unico non li blocca.
func TestSaveSessionWithoutClientIDStillAllowsMultiple(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewSessionRepo(pool)
	ctx := context.Background()

	userID := makeUser(t, pool, "nullid-"+time.Now().Format("150405.000000")+"@example.com")
	exID := exerciseID(t, pool)

	for i := 0; i < 2; i++ {
		s := session(nil, time.Now())
		s.UserID = userID
		for j := range s.Sets {
			s.Sets[j].ExerciseID = exID
		}
		inserted, err := repo.SaveSession(ctx, s)
		if err != nil {
			t.Fatalf("salvataggio %d: %v", i, err)
		}
		if !inserted {
			t.Errorf("salvataggio %d: senza clientSessionId ogni invio deve inserire", i)
		}
	}

	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM session_logs WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("sessioni = %d, attese 2", n)
	}
}

// Il completed_at dichiarato dal client deve arrivare nel database così com'è:
// è ciò su cui CountSessionsForProgramSince decide se l'ultima settimana è
// completa, quindi se il server lo sovrascrivesse con NOW() la
// sincronizzazione differita sfaserebbe il programma.
func TestSaveSessionStoresClientCompletedAt(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewSessionRepo(pool)
	ctx := context.Background()

	userID := makeUser(t, pool, "backdate-"+time.Now().Format("150405.000000")+"@example.com")
	exID := exerciseID(t, pool)

	want := time.Now().Add(-50 * time.Hour).UTC().Truncate(time.Second)
	s := session(nil, want)
	s.UserID = userID
	for i := range s.Sets {
		s.Sets[i].ExerciseID = exID
	}
	if _, err := repo.SaveSession(ctx, s); err != nil {
		t.Fatalf("salvataggio: %v", err)
	}

	var got time.Time
	if err := pool.QueryRow(ctx,
		`SELECT completed_at FROM session_logs WHERE id = $1`, s.ID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if diff := got.UTC().Sub(want); diff > time.Second || diff < -time.Second {
		t.Errorf("completed_at = %v, atteso %v (scarto %v) — il server lo sta sovrascrivendo", got.UTC(), want, diff)
	}
}
