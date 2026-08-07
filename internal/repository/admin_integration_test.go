package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// La tabella `admins` è condivisa fra i test di questo file, che girano contro
// lo stesso database. Ogni test si crea i suoi amministratori con indirizzi
// unici e ripulisce dietro di sé: senza, la protezione sull'ultimo
// amministratore attivo dipenderebbe da cosa hanno lasciato i test precedenti.
func makeAdmin(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO admins (email) VALUES ($1) RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("creazione amministratore: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM admins WHERE id = $1`, id)
	})
	return id
}

func uniqueAdminEmail(prefix string) string {
	return "admin-" + prefix + "-" + time.Now().Format("150405.000000") + "@example.com"
}

// Bootstrap deve creare solo il primo amministratore. Se creasse ogni volta che
// la variabile d'ambiente è impostata, un amministratore disattivato dal
// pannello tornerebbe attivo al riavvio successivo e non ci sarebbe modo di
// toglierlo davvero.
func TestBootstrapOnlyOnEmptyTable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)

	// Se il database di prova ha già amministratori (altri test, esecuzioni
	// precedenti), Bootstrap non deve fare niente: è già il caso da verificare.
	var existing int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&existing); err != nil {
		t.Fatalf("conteggio: %v", err)
	}

	if existing == 0 {
		email := uniqueAdminEmail("bootstrap")
		created, err := repo.Bootstrap(ctx, email)
		if err != nil {
			t.Fatalf("Bootstrap: %v", err)
		}
		if !created {
			t.Fatal("il primo Bootstrap non ha creato niente su una tabella vuota")
		}
		t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM admins WHERE email = $1`, email) })
	} else {
		makeAdmin(t, pool, uniqueAdminEmail("preesistente"))
	}

	// Secondo giro: la tabella non è più vuota, quindi non deve creare nulla —
	// nemmeno con un indirizzo diverso.
	created, err := repo.Bootstrap(ctx, uniqueAdminEmail("secondo"))
	if err != nil {
		t.Fatalf("secondo Bootstrap: %v", err)
	}
	if created {
		t.Fatal("Bootstrap ha creato un amministratore su una tabella non vuota")
	}
}

func TestCreateAdminRejectsDuplicate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)

	creator := makeAdmin(t, pool, uniqueAdminEmail("creatore"))
	email := uniqueAdminEmail("doppione")

	a, err := repo.Create(ctx, email, creator)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM admins WHERE id = $1`, a.ID) })
	if a.CreatedBy == nil || *a.CreatedBy != creator {
		t.Errorf("created_by = %v, atteso %s", a.CreatedBy, creator)
	}

	if _, err := repo.Create(ctx, email, creator); !errors.Is(err, repository.ErrAdminExists) {
		t.Fatalf("secondo Create: err = %v, atteso ErrAdminExists", err)
	}

	// Anche in maiuscolo: l'indirizzo viene normalizzato prima dell'inserimento,
	// se no lo stesso amministratore entrerebbe due volte con due scritture
	// diverse e la revoca ne toglierebbe solo una.
	upper := "ADMIN-" + email[6:]
	if _, err := repo.Create(ctx, upper, creator); !errors.Is(err, repository.ErrAdminExists) {
		t.Fatalf("Create con maiuscole: err = %v, atteso ErrAdminExists", err)
	}
}

// La protezione che conta: nessuno deve poter chiudere fuori tutti quanti.
func TestCannotDisableLastActiveAdmin(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)

	// Il database di prova può avere già amministratori attivi da altri test.
	// Vanno messi da parte, se no "l'ultimo" non è quello che crediamo.
	if _, err := pool.Exec(ctx,
		`UPDATE admins SET disabled_at = NOW() WHERE disabled_at IS NULL`); err != nil {
		t.Fatalf("azzeramento: %v", err)
	}

	first := makeAdmin(t, pool, uniqueAdminEmail("ultimo-1"))
	second := makeAdmin(t, pool, uniqueAdminEmail("ultimo-2"))

	// Con due attivi, disattivarne uno si può.
	if err := repo.SetDisabled(ctx, second, true); err != nil {
		t.Fatalf("disattivazione del secondo: %v", err)
	}

	// Rimasto uno solo, non si può più.
	if err := repo.SetDisabled(ctx, first, true); !errors.Is(err, repository.ErrLastAdmin) {
		t.Fatalf("disattivazione dell'ultimo: err = %v, atteso ErrLastAdmin", err)
	}

	// E infatti è ancora attivo.
	a, err := repo.FindByID(ctx, first)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !a.Active() {
		t.Fatal("l'ultimo amministratore risulta disattivato")
	}

	// Riattivare il secondo deve funzionare, e a quel punto il primo si può
	// disattivare: la protezione è sul numero di attivi, non su chi è.
	if err := repo.SetDisabled(ctx, second, false); err != nil {
		t.Fatalf("riattivazione: %v", err)
	}
	if err := repo.SetDisabled(ctx, first, true); err != nil {
		t.Fatalf("disattivazione dopo la riattivazione: %v", err)
	}
}

// Il token vale una volta sola. Un client di posta che fa prefetch dei link
// apre l'URL prima dell'utente: se il consumo non fosse atomico, il secondo
// tentativo — quello vero — troverebbe il token già usato oppure passerebbero
// entrambi.
func TestAdminLoginTokenIsSingleUse(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)

	id := makeAdmin(t, pool, uniqueAdminEmail("token"))
	token, err := repo.StoreLoginToken(ctx, id)
	if err != nil {
		t.Fatalf("StoreLoginToken: %v", err)
	}

	a, err := repo.VerifyLoginToken(ctx, token)
	if err != nil {
		t.Fatalf("primo VerifyLoginToken: %v", err)
	}
	if a.ID != id {
		t.Errorf("id = %s, atteso %s", a.ID, id)
	}

	if _, err := repo.VerifyLoginToken(ctx, token); err == nil {
		t.Fatal("lo stesso token è stato accettato due volte")
	}
}

// Un link emesso e poi revocato non deve funzionare. È il caso in cui la revoca
// serve davvero: si toglie l'accesso a qualcuno che ha appena chiesto il link.
func TestAdminLoginTokenRejectedAfterDisable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)

	// Un secondo amministratore attivo, se no SetDisabled rifiuta per via della
	// protezione sull'ultimo.
	makeAdmin(t, pool, uniqueAdminEmail("compagno"))
	id := makeAdmin(t, pool, uniqueAdminEmail("revocato"))

	token, err := repo.StoreLoginToken(ctx, id)
	if err != nil {
		t.Fatalf("StoreLoginToken: %v", err)
	}
	if err := repo.SetDisabled(ctx, id, true); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}

	if _, err := repo.VerifyLoginToken(ctx, token); err == nil {
		t.Fatal("un link di un amministratore disattivato è stato accettato")
	}
	// E non deve trovarsi nemmeno per chiedere un link nuovo.
	if _, err := repo.FindActiveByEmail(ctx, "qualsiasi@example.com"); err == nil {
		t.Fatal("FindActiveByEmail ha trovato un indirizzo inesistente")
	}
}

func TestAdminLoginTokenExpires(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)

	id := makeAdmin(t, pool, uniqueAdminEmail("scaduto"))
	token, err := repo.StoreLoginToken(ctx, id)
	if err != nil {
		t.Fatalf("StoreLoginToken: %v", err)
	}
	// Spostare l'orologio non si può, la scadenza sì.
	if _, err := pool.Exec(ctx,
		`UPDATE admin_login_tokens SET expires_at = NOW() - INTERVAL '1 minute'
		 WHERE token = $1`, token); err != nil {
		t.Fatalf("scadenza: %v", err)
	}

	if _, err := repo.VerifyLoginToken(ctx, token); err == nil {
		t.Fatal("un token scaduto è stato accettato")
	}
}

// L'elenco atleti si costruisce con una query sola e due LATERAL. Il rischio è
// che un utente senza sessioni o senza programma sparisca dalla lista, perché
// un JOIN normale al posto di un LEFT JOIN lo escluderebbe — e sarebbe proprio
// l'iscritto nuovo, quello che un allenatore vuole vedere per primo.
func TestListAthletesIncludesUsersWithoutData(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)

	email := uniqueEmail("nudo")
	userID := makeUser(t, pool, email)

	athletes, err := repo.ListAthletes(ctx)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}

	for _, a := range athletes {
		if a.ID != userID {
			continue
		}
		if a.Program != nil {
			t.Errorf("Program = %+v, atteso nil per un utente senza programma", a.Program)
		}
		if a.TotalSessions != 0 || a.Sessions7d != 0 {
			t.Errorf("sessioni = %d/%d, attese 0", a.TotalSessions, a.Sessions7d)
		}
		if a.LastSessionAt != nil {
			t.Errorf("LastSessionAt = %v, atteso nil", a.LastSessionAt)
		}
		if a.Display() != email {
			t.Errorf("Display() = %q, atteso l'email %q", a.Display(), email)
		}
		return
	}
	t.Fatal("un utente senza programma né allenamenti non compare nell'elenco")
}

// Con un programma attivo la riga deve portarne nome e settimana. La settimana
// è calcolata da started_at, non letta: qui si verifica che il calcolo arrivi
// fino alla tabella.
func TestListAthletesReportsProgramWeek(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)
	programs := repository.NewProgramRepo(pool)

	userID := makeUser(t, pool, uniqueEmail("conprogramma"))
	tmpl := templateByID(t, pool, "str-5x5")
	programID, err := programs.CreateFromTemplate(ctx, userID, tmpl, 0)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	// Indietro di tre settimane esatte: siamo nella quarta.
	if _, err := pool.Exec(ctx,
		`UPDATE user_programs SET started_at = NOW() - INTERVAL '21 days' WHERE id = $1`,
		programID); err != nil {
		t.Fatalf("started_at: %v", err)
	}

	athletes, err := repo.ListAthletes(ctx)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	for _, a := range athletes {
		if a.ID != userID {
			continue
		}
		if a.Program == nil {
			t.Fatal("Program = nil per un utente con programma attivo")
		}
		if a.Program.Week != 4 {
			t.Errorf("Week = %d, attesa 4", a.Program.Week)
		}
		if a.Program.Name != tmpl.Name {
			t.Errorf("Name = %q, atteso %q", a.Program.Name, tmpl.Name)
		}
		if a.Program.TotalWeeks != tmpl.TotalWeeks {
			t.Errorf("TotalWeeks = %d, atteso %d", a.Program.TotalWeeks, tmpl.TotalWeeks)
		}
		return
	}
	t.Fatal("l'atleta non compare nell'elenco")
}

// Un massimale con i mezzi chili deve arrivare intero nel database.
//
// Non è un caso di scuola: la schermata del piano arrotonda al mezzo chilo, e
// il ciclo di squat ricava ogni carico prescritto da questo numero. Prima il
// parametro finiva dentro NULLIF($8, 0) senza cast, e Postgres deduceva il tipo
// dallo zero — un letterale intero — troncando 162,5 a 162 senza errori.
func TestCreateFromTemplateKeepsHalfKilos(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)

	userID := makeUser(t, pool, uniqueEmail("mezzochilo"))
	tmpl := templateByID(t, pool, "squat-170")
	programID, err := repo.CreateFromTemplate(ctx, userID, tmpl, 162.5)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	var oneRM float64
	if err := pool.QueryRow(ctx,
		`SELECT one_rm_kg FROM user_programs WHERE id = $1`, programID).Scan(&oneRM); err != nil {
		t.Fatalf("lettura massimale: %v", err)
	}
	if oneRM != 162.5 {
		t.Fatalf("one_rm_kg = %v, atteso 162.5 (troncamento del parametro)", oneRM)
	}

	// E i carichi che ne derivano devono seguirlo: il primo giorno è al 65%,
	// cioè 105,625 → 105,5 arrotondato al mezzo chilo.
	var load float64
	if err := pool.QueryRow(ctx,
		`SELECT pwe.load_kg
		 FROM program_workout_exercises pwe
		 JOIN program_workouts pw ON pw.id = pwe.workout_id
		 WHERE pw.program_id = $1 AND pw.week_number = 1 AND pw.day_of_week = (
		   SELECT MIN(day_of_week) FROM program_workouts WHERE program_id = $1 AND week_number = 1)
		   AND pwe.load_kg IS NOT NULL
		 ORDER BY pwe.order_index LIMIT 1`, programID).Scan(&load); err != nil {
		t.Fatalf("lettura carico: %v", err)
	}
	if load != 105.5 {
		t.Errorf("carico del primo giorno = %v, atteso 105.5 (65%% di 162,5)", load)
	}
}

// Un programma scelto dall'atleta non deve risultare assegnato da nessuno.
func TestCreateFromTemplateLeavesAssignedByEmpty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)

	userID := makeUser(t, pool, uniqueEmail("scelta-atleta"))
	if _, err := repo.CreateFromTemplate(ctx, userID, templateByID(t, pool, "str-5x5"), 0); err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	p, err := repo.GetActiveProgram(ctx, userID)
	if err != nil {
		t.Fatalf("GetActiveProgram: %v", err)
	}
	if p.AssignedByCoach() {
		t.Errorf("AssignedByCoach() = true per un programma scelto dall'atleta (email %v)",
			p.AssignedByEmail)
	}
	if p.AssignedBy != nil {
		t.Errorf("AssignedBy = %v, atteso nil", *p.AssignedBy)
	}
}

func TestAssignFromTemplateRecordsTheAdmin(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)

	email := uniqueAdminEmail("assegnatore")
	adminID := makeAdmin(t, pool, email)
	admin, err := repository.NewAdminRepo(pool).FindByID(ctx, adminID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	userID := makeUser(t, pool, uniqueEmail("assegnato"))
	if _, err := repo.AssignFromTemplate(ctx, userID, templateByID(t, pool, "str-5x5"), 0, admin); err != nil {
		t.Fatalf("AssignFromTemplate: %v", err)
	}

	p, err := repo.GetActiveProgram(ctx, userID)
	if err != nil {
		t.Fatalf("GetActiveProgram: %v", err)
	}
	if !p.AssignedByCoach() {
		t.Fatal("AssignedByCoach() = false per un programma assegnato dal pannello")
	}
	if p.AssignedBy == nil || *p.AssignedBy != adminID {
		t.Errorf("AssignedBy = %v, atteso %s", p.AssignedBy, adminID)
	}
	if p.AssignedByEmail == nil || *p.AssignedByEmail != email {
		t.Errorf("AssignedByEmail = %v, atteso %s", p.AssignedByEmail, email)
	}
}

// Il caso per cui l'indirizzo si registra invece di ricavarlo dal join.
//
// Cancellando l'amministratore, assigned_by va a NULL per via della chiave
// esterna — e NULL è anche il valore che significa "scelto dall'atleta".
// Basandosi su quello, un programma assegnato dall'allenatore comincerebbe a
// risultare scelto dall'atleta: non un dato mancante, un dato falso.
func TestAssignedBySurvivesAdminDeletion(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)

	email := uniqueAdminEmail("cancellato")
	adminID := makeAdmin(t, pool, email)
	admin, err := repository.NewAdminRepo(pool).FindByID(ctx, adminID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	userID := makeUser(t, pool, uniqueEmail("orfano"))
	if _, err := repo.AssignFromTemplate(ctx, userID, templateByID(t, pool, "str-5x5"), 0, admin); err != nil {
		t.Fatalf("AssignFromTemplate: %v", err)
	}

	// Cancellazione vera, non la disattivazione del pannello.
	if _, err := pool.Exec(ctx, `DELETE FROM admins WHERE id = $1`, adminID); err != nil {
		t.Fatalf("cancellazione: %v", err)
	}

	p, err := repo.GetActiveProgram(ctx, userID)
	if err != nil {
		t.Fatalf("GetActiveProgram: %v", err)
	}
	if p.AssignedBy != nil {
		t.Errorf("AssignedBy = %v, atteso nil dopo la cancellazione", *p.AssignedBy)
	}
	if !p.AssignedByCoach() {
		t.Fatal("il programma risulta scelto dall'atleta dopo la cancellazione dell'amministratore")
	}
	if p.AssignedByEmail == nil || *p.AssignedByEmail != email {
		t.Errorf("AssignedByEmail = %v, atteso %s", p.AssignedByEmail, email)
	}
}

// GetAthleteSession filtra per utente oltre che per sessione: un id copiato
// dalla scheda di un altro atleta non deve produrre una pagina con
// l'intestazione sbagliata.
func TestGetAthleteSessionIsScopedToUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewAdminRepo(pool)
	sessions := repository.NewSessionRepo(pool)

	owner := makeUser(t, pool, uniqueEmail("proprietario"))
	other := makeUser(t, pool, uniqueEmail("altro"))

	log := &model.SessionLog{
		UserID: owner, Name: "Panca", CompletedAt: time.Now(),
	}
	if _, err := sessions.SaveSession(ctx, log); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	if _, err := repo.GetAthleteSession(ctx, owner, log.ID); err != nil {
		t.Fatalf("il proprietario non riesce a leggere la propria sessione: %v", err)
	}
	if _, err := repo.GetAthleteSession(ctx, other, log.ID); err == nil {
		t.Fatal("la sessione di un atleta è stata letta sotto l'id di un altro")
	}
}
