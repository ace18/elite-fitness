package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/db"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Test del pannello contro un Postgres vero e attraverso il router, non
// chiamando gli handler a mano.
//
//	TEST_DATABASE_URL="postgres://postgres:test@localhost:55436/elite?sslmode=disable" go test ./internal/handler/admin/
//
// Passare dal router conta: il confine fra chi entra e chi no è fatto di
// middleware, cookie e redirect, cioè esattamente le cose che una chiamata
// diretta all'handler salta.
const testSecret = "test-secret-at-least-32-characters!!"

type harness struct {
	srv     *httptest.Server
	pool    *pgxpool.Pool
	admins  *repository.AdminRepo
	users   *repository.UserRepo
	auth    *service.AuthService
	program *repository.ProgramRepo
	catalog *repository.CatalogRepo
}

func newHarness(t *testing.T) *harness {
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

	adminRepo := repository.NewAdminRepo(pool)
	userRepo := repository.NewUserRepo(pool)
	programRepo := repository.NewProgramRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)

	catalogRepo := repository.NewCatalogRepo(pool)

	svc := service.NewAdminService(adminRepo, testSecret, true, nil, "http://panel.test")
	h, err := New(svc, adminRepo, programRepo, sessionRepo, catalogRepo, false)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	// Montato come in main.go: chi.Mount toglie il prefisso, e sbagliare qui
	// vorrebbe dire testare rotte che in produzione non esistono.
	r := chi.NewRouter()
	r.Mount("/admin", h.Routes())
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &harness{
		srv: srv, pool: pool, admins: adminRepo, users: userRepo,
		auth:    service.NewAuthService(userRepo, testSecret, true, nil, "http://app.test"),
		program: programRepo,
		catalog: catalogRepo,
	}
}

// get esegue una richiesta senza seguire i redirect: seguirli nasconderebbe
// proprio la risposta che interessa (il 303 verso l'accesso).
func (h *harness) get(t *testing.T, path, cookie string) *http.Response {
	t.Helper()
	return h.do(t, http.MethodGet, path, cookie, nil)
}

func (h *harness) do(t *testing.T, method, path, cookie string, form url.Values) *http.Response {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequest(method, h.srv.URL+path, body)
	if err != nil {
		t.Fatalf("richiesta: %v", err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie})
	}
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("esecuzione: %v", err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func uniq(prefix string) string {
	return prefix + "-" + time.Now().Format("150405.000000000") + "@example.com"
}

func (h *harness) makeAdmin(t *testing.T) (id, cookie string) {
	t.Helper()
	ctx := context.Background()
	err := h.pool.QueryRow(ctx,
		`INSERT INTO admins (email) VALUES ($1) RETURNING id`, uniq("panel-admin")).Scan(&id)
	if err != nil {
		t.Fatalf("creazione amministratore: %v", err)
	}
	t.Cleanup(func() { h.pool.Exec(ctx, `DELETE FROM admins WHERE id = $1`, id) })

	svc := service.NewAdminService(h.admins, testSecret, true, nil, "")
	a, err := h.admins.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	cookie, err = svc.IssueJWT(a)
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}
	return id, cookie
}

func (h *harness) makeAthlete(t *testing.T) (id, jwt string) {
	t.Helper()
	ctx := context.Background()
	u, err := h.users.FindOrCreate(ctx, uniq("panel-athlete"))
	if err != nil {
		t.Fatalf("creazione utente: %v", err)
	}
	jwt, err = h.auth.IssueJWT(u)
	if err != nil {
		t.Fatalf("IssueJWT atleta: %v", err)
	}
	return u.ID, jwt
}

// csrfFrom estrae il token dal form della pagina, che è l'unico modo in cui un
// amministratore vero lo ottiene.
func csrfFrom(t *testing.T, body string) string {
	t.Helper()
	const marker = `name="_csrf" value="`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatal("nessun campo _csrf nella pagina")
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatal("campo _csrf malformato")
	}
	return rest[:j]
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()
	b := make([]byte, 0, 64*1024)
	buf := make([]byte, 4096)
	for {
		n, err := res.Body.Read(buf)
		b = append(b, buf[:n]...)
		if err != nil {
			break
		}
	}
	return string(b)
}

// ---- il confine: gli atleti non entrano nel pannello ------------------------

// Il requisito, verificato su ogni rotta invece che sulla sola pagina iniziale:
// basta una rotta registrata fuori dal gruppo protetto perché il confine non
// valga più, e non si vedrebbe da nessuna altra parte.
//
// Il token dell'atleta è valido e non scaduto — è la sua sessione vera. A
// respingerlo è l'audience del JWT (vedi service/admin.go), non la sua
// invalidità.
func TestAthleteTokenCannotReachAnyPanelRoute(t *testing.T) {
	h := newHarness(t)
	athleteID, athleteJWT := h.makeAthlete(t)

	// Prova che il token è buono per la sua parte: se fosse malformato il test
	// passerebbe senza dimostrare niente.
	if _, _, err := h.auth.ParseJWT(athleteJWT); err != nil {
		t.Fatalf("il token dell'atleta dovrebbe essere valido per l'app: %v", err)
	}

	routes := []string{
		"/admin/",
		"/admin/atleti/" + athleteID,
		"/admin/atleti/" + athleteID + "/sessioni/00000000-0000-0000-0000-000000000000",
		"/admin/amministratori",
	}
	for _, path := range routes {
		res := h.get(t, path, athleteJWT)
		if res.StatusCode != http.StatusSeeOther {
			t.Errorf("GET %s con token da atleta: status %d, atteso 303", path, res.StatusCode)
			continue
		}
		if loc := res.Header.Get("Location"); loc != basePath+"/accesso" {
			t.Errorf("GET %s: redirect a %q, atteso la pagina di accesso", path, loc)
		}
	}
}

func TestNoCookieRedirectsToLogin(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{"/admin/", "/admin/amministratori"} {
		res := h.get(t, path, "")
		if res.StatusCode != http.StatusSeeOther {
			t.Errorf("GET %s senza cookie: status %d, atteso 303", path, res.StatusCode)
		}
	}
}

// Il controllo positivo: senza, i test qui sopra passerebbero anche se il
// pannello fosse rotto e rispondesse 303 a chiunque.
func TestAdminCookieReachesPanel(t *testing.T) {
	h := newHarness(t)
	_, cookie := h.makeAdmin(t)

	res := h.get(t, "/admin/", cookie)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/ con cookie valido: status %d, atteso 200", res.StatusCode)
	}
	if !strings.Contains(readBody(t, res), "Atleti") {
		t.Error("la pagina non sembra l'elenco atleti")
	}
}

// Un atleta non deve poter assegnare programmi — a sé o a chiunque altro. È la
// rotta che scrive, quindi quella su cui il confine conta davvero.
func TestAthleteCannotAssignProgram(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	athleteID, athleteJWT := h.makeAthlete(t)

	res := h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", athleteJWT,
		url.Values{"templateId": {"str-5x5"}, "_csrf": {"qualunque"}})
	if res.StatusCode != http.StatusSeeOther {
		t.Errorf("POST programma con token da atleta: status %d, atteso 303", res.StatusCode)
	}

	var n int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_programs WHERE user_id = $1`, athleteID).Scan(&n); err != nil {
		t.Fatalf("conteggio: %v", err)
	}
	if n != 0 {
		t.Fatalf("l'atleta si è assegnato %d programmi da solo", n)
	}
}

func TestDisabledAdminLosesAccessImmediately(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Ne serve un secondo attivo, se no scatta la protezione sull'ultimo.
	h.makeAdmin(t)
	id, cookie := h.makeAdmin(t)

	if res := h.get(t, "/admin/", cookie); res.StatusCode != http.StatusOK {
		t.Fatalf("prima della revoca: status %d, atteso 200", res.StatusCode)
	}
	if err := h.admins.SetDisabled(ctx, id, true); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}
	// Stesso cookie, ancora firmato e non scaduto.
	if res := h.get(t, "/admin/", cookie); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("dopo la revoca: status %d, atteso 303", res.StatusCode)
	}
}

// ---- assegnazione del programma --------------------------------------------

func TestAssignProgramRequiresCSRF(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	for name, form := range map[string]url.Values{
		"senza token":     {"templateId": {"str-5x5"}},
		"token sbagliato": {"templateId": {"str-5x5"}, "_csrf": {"0000000000000000"}},
	} {
		res := h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie, form)
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("%s: status %d, atteso 403", name, res.StatusCode)
		}
	}

	var n int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_programs WHERE user_id = $1`, athleteID).Scan(&n); err != nil {
		t.Fatalf("conteggio: %v", err)
	}
	if n != 0 {
		t.Fatalf("%d programmi creati senza un token CSRF valido", n)
	}
}

// Il comportamento richiesto: assegnare sostituisce il programma attivo.
//
// Si verifica anche che il precedente NON risulti completato: is_active a false
// senza completed_at è ciò che distingue "sostituito" da "portato a termine", e
// confonderli falserebbe lo storico dell'atleta.
func TestAssignProgramReplacesActiveOne(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	page := h.get(t, "/admin/atleti/"+athleteID, cookie)
	csrf := csrfFrom(t, readBody(t, page))

	assign := func(templateID, oneRM string) *http.Response {
		return h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie,
			url.Values{"templateId": {templateID}, "oneRm": {oneRM}, "_csrf": {csrf}})
	}

	if res := assign("str-5x5", ""); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("prima assegnazione: status %d, atteso 303", res.StatusCode)
	}
	first := activeProgram(t, h.pool, athleteID)

	// Portiamolo avanti nel tempo: se la sostituzione non azzerasse la
	// settimana non ce ne accorgeremmo su un programma appena creato.
	if _, err := h.pool.Exec(ctx,
		`UPDATE user_programs SET started_at = NOW() - INTERVAL '21 days' WHERE id = $1`,
		first); err != nil {
		t.Fatalf("started_at: %v", err)
	}

	if res := assign("ppl-hyp", ""); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("seconda assegnazione: status %d, atteso 303", res.StatusCode)
	}

	// Il vecchio: disattivato ma non completato.
	var isActive bool
	var completedAt *time.Time
	if err := h.pool.QueryRow(ctx,
		`SELECT is_active, completed_at FROM user_programs WHERE id = $1`, first,
	).Scan(&isActive, &completedAt); err != nil {
		t.Fatalf("lettura vecchio programma: %v", err)
	}
	if isActive {
		t.Error("il programma precedente è ancora attivo")
	}
	if completedAt != nil {
		t.Error("il programma sostituito risulta completato")
	}

	// Il nuovo: attivo, uno solo, e alla settimana 1.
	var active int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_programs WHERE user_id = $1 AND is_active`, athleteID,
	).Scan(&active); err != nil {
		t.Fatalf("conteggio attivi: %v", err)
	}
	if active != 1 {
		t.Fatalf("programmi attivi = %d, atteso 1", active)
	}
	p, err := h.program.GetActiveProgram(ctx, athleteID)
	if err != nil {
		t.Fatalf("GetActiveProgram: %v", err)
	}
	if p.CurrentWeek != 1 {
		t.Errorf("settimana = %d, attesa 1 per un programma appena assegnato", p.CurrentWeek)
	}

	// E il programma nuovo ha davvero i suoi allenamenti: una sostituzione che
	// lascia l'atleta con un piano vuoto è peggio di non averla fatta.
	var workouts int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM program_workouts WHERE program_id = $1`, p.ID).Scan(&workouts); err != nil {
		t.Fatalf("conteggio allenamenti: %v", err)
	}
	if workouts == 0 {
		t.Error("il programma assegnato non ha nessun allenamento")
	}
}

// Assegnando dal pannello, il programma deve portare il nome di chi l'ha
// assegnato — e la scheda deve dirlo, se no la colonna esiste ma non serve a
// nessuno.
func TestAssignProgramRecordsWhoAssignedIt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	adminID, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	admin, err := h.admins.FindByID(ctx, adminID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	page := h.get(t, "/admin/atleti/"+athleteID, cookie)
	csrf := csrfFrom(t, readBody(t, page))
	res := h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie,
		url.Values{"templateId": {"str-5x5"}, "_csrf": {csrf}})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("assegnazione: status %d, atteso 303", res.StatusCode)
	}

	p, err := h.program.GetActiveProgram(ctx, athleteID)
	if err != nil {
		t.Fatalf("GetActiveProgram: %v", err)
	}
	if p.AssignedBy == nil || *p.AssignedBy != adminID {
		t.Errorf("AssignedBy = %v, atteso %s", p.AssignedBy, adminID)
	}

	body := readBody(t, h.get(t, "/admin/atleti/"+athleteID, cookie))
	if !strings.Contains(body, "Scelto da") || !strings.Contains(body, admin.Email) {
		t.Error("la scheda non mostra chi ha assegnato il programma")
	}
}

// E il contrario: un programma che l'atleta si è scelto dall'app deve risultare
// scelto da lui, non attribuito a chi capita.
func TestSelfChosenProgramShowsAsAthletesOwn(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	// Il percorso dell'atleta, non quello del pannello.
	tmpl := templateByID(t, h.program, "str-5x5")
	if _, err := h.program.CreateFromTemplate(ctx, athleteID, tmpl, 0); err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	body := readBody(t, h.get(t, "/admin/atleti/"+athleteID, cookie))
	if !strings.Contains(body, "l'atleta") {
		t.Error("la scheda non dice che il programma se l'è scelto l'atleta")
	}
}

// Una template che prescrive i carichi senza dichiarare una finestra di
// massimali deve comunque pretendere il massimale.
//
// È il caso che nasce col pannello: prima della gestione delle template, le
// uniche a carico prescritto erano i due cicli di squat, che una finestra ce
// l'hanno. Scrivendone una a mano si può benissimo mettere un 70% senza
// finestra, e chiedendo il massimale solo in base alla finestra l'atleta si
// ritroverebbe la scheda senza pesi — CreateFromTemplate lascia NULL quando il
// massimale è zero, quindi non fallisce niente e non se ne accorge nessuno.
func TestAssignRequiresOneRMForPrescribingTemplateWithoutWindow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	// Una template col 70% sul primo esercizio e nessuna finestra.
	tmplID := "prescr-" + time.Now().Format("150405.000000000")
	if err := h.catalog.CreateTemplate(ctx, repository.TemplateInput{
		ID: tmplID, Name: "Prescrive senza finestra", Goal: "Forza", Focus: "Test",
		Level: "Intermedio", DaysPerWeek: 1, SessionMin: 60, TotalWeeks: 4, Glyph: "🏋️",
	}); err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	t.Cleanup(func() { h.pool.Exec(ctx, `DELETE FROM plan_templates WHERE id = $1`, tmplID) })

	dayID, err := h.catalog.CreateDay(ctx, tmplID, 1, 0, "Giorno 1", "")
	if err != nil {
		t.Fatalf("CreateDay: %v", err)
	}
	exID, err := h.catalog.CreateExercise(ctx,
		"Prescritto "+time.Now().Format("150405.000000000"), "Test", "compound")
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	t.Cleanup(func() { h.pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, exID) })
	pct := 0.70
	if err := h.catalog.AddExerciseRow(ctx, dayID, repository.ExerciseRowInput{
		ExerciseID: exID, Sets: 5, TargetReps: 5, RestSeconds: 180, LoadPct: &pct}); err != nil {
		t.Fatalf("AddExerciseRow: %v", err)
	}

	// GetTemplates deve accorgersene, ed è quello che il pannello legge.
	list, err := h.program.GetTemplates(ctx)
	if err != nil {
		t.Fatalf("GetTemplates: %v", err)
	}
	var found bool
	for _, tm := range list {
		if tm.ID == tmplID {
			found = true
			if !tm.Prescribes {
				t.Error("Prescribes = false per una template con load_pct")
			}
			if tm.MinOneRM != nil {
				t.Error("questa template non dovrebbe avere una finestra")
			}
		}
	}
	if !found {
		t.Fatal("la template non compare fra quelle assegnabili")
	}

	page := h.get(t, "/admin/atleti/"+athleteID, cookie)
	csrf := csrfFrom(t, readBody(t, page))

	// Senza massimale: rifiutata.
	res := h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie,
		url.Values{"templateId": {tmplID}, "oneRm": {""}, "_csrf": {csrf}})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("senza massimale: status %d, atteso 400", res.StatusCode)
	}
	var n int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_programs WHERE user_id = $1`, athleteID).Scan(&n); err != nil {
		t.Fatalf("conteggio: %v", err)
	}
	if n != 0 {
		t.Fatal("assegnata senza massimale una template che prescrive i carichi")
	}

	// Con il massimale: passa, e i carichi si materializzano davvero.
	res = h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie,
		url.Values{"templateId": {tmplID}, "oneRm": {"100"}, "_csrf": {csrf}})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("con massimale: status %d, atteso 303", res.StatusCode)
	}
	var load float64
	if err := h.pool.QueryRow(ctx,
		`SELECT pwe.load_kg FROM program_workout_exercises pwe
		 JOIN program_workouts pw ON pw.id = pwe.workout_id
		 JOIN user_programs p ON p.id = pw.program_id
		 WHERE p.user_id = $1 AND p.is_active`, athleteID).Scan(&load); err != nil {
		t.Fatalf("lettura carico: %v", err)
	}
	if load != 70 {
		t.Errorf("carico = %v, atteso 70 (70%% di 100)", load)
	}
}

func templateByID(t *testing.T, repo *repository.ProgramRepo, id string) model.PlanTemplate {
	t.Helper()
	templates, err := repo.GetTemplates(context.Background())
	if err != nil {
		t.Fatalf("GetTemplates: %v", err)
	}
	for _, tm := range templates {
		if tm.ID == id {
			return tm
		}
	}
	t.Fatalf("template %q non trovata", id)
	return model.PlanTemplate{}
}

// Un massimale fuori finestra non deve assegnare niente, e il messaggio deve
// dire quale finestra: chi assegna non ha modo di indovinarla.
func TestAssignProgramRejectsOneRMOutOfWindow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	page := h.get(t, "/admin/atleti/"+athleteID, cookie)
	csrf := csrfFrom(t, readBody(t, page))

	// squat-170 vale per massimali 145–170 kg.
	res := h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie,
		url.Values{"templateId": {"squat-170"}, "oneRm": {"200"}, "_csrf": {csrf}})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, atteso 400", res.StatusCode)
	}
	body := readBody(t, res)
	if !strings.Contains(body, "145") || !strings.Contains(body, "170") {
		t.Error("il messaggio non dice quale finestra di massimali è ammessa")
	}

	var n int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_programs WHERE user_id = $1`, athleteID).Scan(&n); err != nil {
		t.Fatalf("conteggio: %v", err)
	}
	if n != 0 {
		t.Fatalf("creati %d programmi con un massimale fuori finestra", n)
	}
}

// Il massimale scritto all'italiana deve funzionare: il pannello è in italiano
// e chi ci scrive dentro usa la virgola.
func TestAssignProgramAcceptsCommaDecimal(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	page := h.get(t, "/admin/atleti/"+athleteID, cookie)
	csrf := csrfFrom(t, readBody(t, page))

	res := h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie,
		url.Values{"templateId": {"squat-170"}, "oneRm": {"162,5"}, "_csrf": {csrf}})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d, atteso 303", res.StatusCode)
	}

	var oneRM float64
	if err := h.pool.QueryRow(ctx,
		`SELECT one_rm_kg FROM user_programs WHERE user_id = $1 AND is_active`, athleteID,
	).Scan(&oneRM); err != nil {
		t.Fatalf("lettura massimale: %v", err)
	}
	if oneRM != 162.5 {
		t.Errorf("one_rm_kg = %v, atteso 162.5", oneRM)
	}
}

// Un programma a carico prescritto senza massimale non deve passare con uno
// zero: i carichi verrebbero tutti nulli e l'atleta si troverebbe una scheda
// senza pesi.
func TestAssignProgramRequiresOneRMWhenTemplateNeedsIt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, cookie := h.makeAdmin(t)
	athleteID, _ := h.makeAthlete(t)

	page := h.get(t, "/admin/atleti/"+athleteID, cookie)
	csrf := csrfFrom(t, readBody(t, page))

	res := h.do(t, http.MethodPost, "/admin/atleti/"+athleteID+"/programma", cookie,
		url.Values{"templateId": {"squat-170"}, "oneRm": {""}, "_csrf": {csrf}})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, atteso 400", res.StatusCode)
	}

	var n int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_programs WHERE user_id = $1`, athleteID).Scan(&n); err != nil {
		t.Fatalf("conteggio: %v", err)
	}
	if n != 0 {
		t.Fatal("assegnato un programma a carico prescritto senza massimale")
	}
}

func activeProgram(t *testing.T, pool *pgxpool.Pool, userID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM user_programs WHERE user_id = $1 AND is_active`, userID).Scan(&id); err != nil {
		t.Fatalf("programma attivo: %v", err)
	}
	return id
}
