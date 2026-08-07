package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

func uniqueName(prefix string) string {
	return prefix + " " + time.Now().Format("150405.000000000")
}

func newTemplate(t *testing.T, pool *pgxpool.Pool, repo *repository.CatalogRepo) string {
	t.Helper()
	ctx := context.Background()
	id := "tmpl-" + time.Now().Format("150405.000000000")
	in := repository.TemplateInput{
		ID: id, Name: uniqueName("Prova"), Goal: "Forza", Focus: "Test",
		Level: "Intermedio", DaysPerWeek: 3, SessionMin: 60, TotalWeeks: 8, Glyph: "🏋️",
	}
	if err := repo.CreateTemplate(ctx, in); err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM plan_templates WHERE id = $1`, id) })
	return id
}

// ---- esercizi --------------------------------------------------------------

func TestExerciseNamesAreUnique(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewCatalogRepo(pool)

	name := uniqueName("Front Squat")
	id, err := repo.CreateExercise(ctx, name, "Quadricipiti", "compound")
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, id) })

	if _, err := repo.CreateExercise(ctx, name, "Quadricipiti", "compound"); !errors.Is(err, repository.ErrExerciseExists) {
		t.Fatalf("secondo inserimento: err = %v, atteso ErrExerciseExists", err)
	}
}

// Qualsiasi categoria non riconosciuta diventa 'compound'. Non è pignoleria:
// la categoria decide l'incremento di carico (loadIncrement), e un valore
// inventato che finisse nel database renderebbe il confronto `== "isolation"`
// falso senza che nessuno se ne accorga.
func TestExerciseCategoryIsNormalized(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewCatalogRepo(pool)

	for _, in := range []string{"", "ISOLATION", "isolation", "qualcosa", "compound"} {
		id, err := repo.CreateExercise(ctx, uniqueName("Cat "+in), "Test", in)
		if err != nil {
			t.Fatalf("CreateExercise(%q): %v", in, err)
		}
		t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, id) })

		e, err := repo.FindExercise(ctx, id)
		if err != nil {
			t.Fatalf("FindExercise: %v", err)
		}
		want := "compound"
		if in == "isolation" || in == "ISOLATION" {
			want = "isolation"
		}
		if e.Category != want {
			t.Errorf("categoria per %q = %q, attesa %q", in, e.Category, want)
		}
	}
}

// Un esercizio usato da una template non si cancella, e la chiave esterna non
// deve nemmeno arrivare a lamentarsi: l'esito dev'essere un errore riconoscibile
// su cui il pannello sa scrivere una frase.
func TestDeleteExerciseBlockedWhenUsedByTemplate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewCatalogRepo(pool)

	exID, err := repo.CreateExercise(ctx, uniqueName("Usato"), "Test", "compound")
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, exID) })

	tmplID := newTemplate(t, pool, repo)
	dayID, err := repo.CreateDay(ctx, tmplID, 1, 0, "Giorno 1", "Test")
	if err != nil {
		t.Fatalf("CreateDay: %v", err)
	}
	if err := repo.AddExerciseRow(ctx, dayID, repository.ExerciseRowInput{
		ExerciseID: exID, Sets: 3, TargetReps: 8, RestSeconds: 90}); err != nil {
		t.Fatalf("AddExerciseRow: %v", err)
	}

	if err := repo.DeleteExercise(ctx, exID); !errors.Is(err, repository.ErrExerciseInUse) {
		t.Fatalf("DeleteExercise: err = %v, atteso ErrExerciseInUse", err)
	}

	// Tolto dalla template, si cancella.
	day, err := repo.FindDay(ctx, tmplID, dayID)
	if err != nil {
		t.Fatalf("FindDay: %v", err)
	}
	if err := repo.DeleteExerciseRow(ctx, dayID, day.Exercises[0].ID); err != nil {
		t.Fatalf("DeleteExerciseRow: %v", err)
	}
	if err := repo.DeleteExercise(ctx, exID); err != nil {
		t.Fatalf("DeleteExercise dopo averlo tolto: %v", err)
	}
}

// ---- template --------------------------------------------------------------

// Archiviare toglie la template dal catalogo assegnabile senza toccare niente
// altro. È la ragione per cui l'archiviazione esiste: cancellare non si può
// quando qualcuno l'ha già ricevuta.
func TestArchivedTemplateLeavesTheAssignableCatalog(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	catalog := repository.NewCatalogRepo(pool)
	programs := repository.NewProgramRepo(pool)

	id := newTemplate(t, pool, catalog)

	inCatalog := func() bool {
		list, err := programs.GetTemplates(ctx)
		if err != nil {
			t.Fatalf("GetTemplates: %v", err)
		}
		for _, tm := range list {
			if tm.ID == id {
				return true
			}
		}
		return false
	}

	if !inCatalog() {
		t.Fatal("una template nuova non compare fra quelle assegnabili")
	}
	if err := catalog.SetTemplateArchived(ctx, id, true); err != nil {
		t.Fatalf("SetTemplateArchived: %v", err)
	}
	if inCatalog() {
		t.Error("una template archiviata compare ancora fra quelle assegnabili")
	}

	// Ma resta nel catalogo del pannello, se no non ci sarebbe modo di
	// ripristinarla.
	all, err := catalog.ListTemplates(ctx)
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	var found bool
	for _, tm := range all {
		if tm.ID == id {
			found = true
			if !tm.Archived() {
				t.Error("Archived() = false per una template archiviata")
			}
		}
	}
	if !found {
		t.Error("la template archiviata è sparita anche dal pannello")
	}

	if err := catalog.SetTemplateArchived(ctx, id, false); err != nil {
		t.Fatalf("ripristino: %v", err)
	}
	if !inCatalog() {
		t.Error("la template ripristinata non è tornata fra quelle assegnabili")
	}
}

// Una template da cui è nato un programma non si cancella: user_programs la
// referenzia senza ON DELETE. L'errore dev'essere riconoscibile, non un
// messaggio della chiave esterna.
func TestDeleteTemplateBlockedWhenAssigned(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	catalog := repository.NewCatalogRepo(pool)
	programs := repository.NewProgramRepo(pool)

	id := newTemplate(t, pool, catalog)
	if _, err := catalog.CreateDay(ctx, id, 1, 0, "Giorno 1", ""); err != nil {
		t.Fatalf("CreateDay: %v", err)
	}

	// Cancellabile finché nessuno l'ha ricevuta.
	summary, err := catalog.FindTemplate(ctx, id)
	if err != nil {
		t.Fatalf("FindTemplate: %v", err)
	}
	if !summary.Deletable() {
		t.Fatal("Deletable() = false per una template mai assegnata")
	}

	userID := makeUser(t, pool, uniqueEmail("catalogo"))
	if _, err := programs.CreateFromTemplate(ctx, userID, summary.PlanTemplate, 0); err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	if err := catalog.DeleteTemplate(ctx, id); !errors.Is(err, repository.ErrTemplateInUse) {
		t.Fatalf("DeleteTemplate: err = %v, atteso ErrTemplateInUse", err)
	}
	summary, err = catalog.FindTemplate(ctx, id)
	if err != nil {
		t.Fatalf("FindTemplate: %v", err)
	}
	if summary.Deletable() {
		t.Error("Deletable() = true per una template già assegnata")
	}
	if summary.InUse != 1 {
		t.Errorf("InUse = %d, atteso 1", summary.InUse)
	}
}

// GetTemplates deve segnalare le template che ricavano i carichi dal massimale,
// perché è il segnale su cui sia il pannello sia la schermata dei piani
// decidono di chiedere il massimale.
//
// Non coincide con l'avere una finestra: i cicli di squat hanno entrambe le
// cose, ma una template scritta nel pannello può avere solo la percentuale.
func TestGetTemplatesReportsPrescribedLoads(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	catalog := repository.NewCatalogRepo(pool)
	programs := repository.NewProgramRepo(pool)

	id := newTemplate(t, pool, catalog)
	dayID, err := catalog.CreateDay(ctx, id, 1, 0, "Giorno 1", "")
	if err != nil {
		t.Fatalf("CreateDay: %v", err)
	}
	exID, err := catalog.CreateExercise(ctx, uniqueName("Segnale"), "Test", "compound")
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, exID) })

	prescribes := func() bool {
		list, err := programs.GetTemplates(ctx)
		if err != nil {
			t.Fatalf("GetTemplates: %v", err)
		}
		for _, tm := range list {
			if tm.ID == id {
				return tm.Prescribes
			}
		}
		t.Fatal("template non trovata")
		return false
	}

	// Una riga senza carico prescritto non deve accendere il segnale.
	if err := catalog.AddExerciseRow(ctx, dayID, repository.ExerciseRowInput{
		ExerciseID: exID, Sets: 3, TargetReps: 8, RestSeconds: 90}); err != nil {
		t.Fatalf("AddExerciseRow: %v", err)
	}
	if prescribes() {
		t.Error("Prescribes = true senza nessuna riga a carico prescritto")
	}

	// Aggiungendone una con la percentuale, sì.
	ex2, err := catalog.CreateExercise(ctx, uniqueName("Segnale 2"), "Test", "compound")
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, ex2) })
	pct := 0.65
	if err := catalog.AddExerciseRow(ctx, dayID, repository.ExerciseRowInput{
		ExerciseID: ex2, Sets: 5, TargetReps: 5, RestSeconds: 180, LoadPct: &pct}); err != nil {
		t.Fatalf("AddExerciseRow: %v", err)
	}
	if !prescribes() {
		t.Error("Prescribes = false con una riga a carico prescritto")
	}

	// E le template seminate senza carichi prescritti restano a false: un
	// segnale acceso su tutto costringerebbe a inserire il massimale ovunque.
	list, err := programs.GetTemplates(ctx)
	if err != nil {
		t.Fatalf("GetTemplates: %v", err)
	}
	for _, tm := range list {
		if tm.ID == "str-5x5" && tm.Prescribes {
			t.Error("str-5x5 risulta a carico prescritto")
		}
		if tm.ID == "squat-170" && !tm.Prescribes {
			t.Error("squat-170 non risulta a carico prescritto")
		}
	}
}

// ---- giorni e blocchi ------------------------------------------------------

// Lo stesso giorno della settimana può esistere una volta per blocco, non di
// più: è il vincolo che la 009 ha introdotto e che tiene separati i blocchi
// quando CreateFromTemplate li copia.
func TestDayIsUniquePerBlock(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewCatalogRepo(pool)
	id := newTemplate(t, pool, repo)

	if _, err := repo.CreateDay(ctx, id, 1, 2, "Mercoledì blocco 1", ""); err != nil {
		t.Fatalf("primo giorno: %v", err)
	}
	if _, err := repo.CreateDay(ctx, id, 1, 2, "Di nuovo mercoledì", ""); !errors.Is(err, repository.ErrDayExists) {
		t.Fatalf("stesso giorno nello stesso blocco: err = %v, atteso ErrDayExists", err)
	}
	// In un blocco diverso invece è esattamente ciò che si vuole.
	if _, err := repo.CreateDay(ctx, id, 5, 2, "Mercoledì blocco 2", ""); err != nil {
		t.Fatalf("stesso giorno in un altro blocco: %v", err)
	}

	blocks, err := repo.ListDays(ctx, id)
	if err != nil {
		t.Fatalf("ListDays: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("blocchi = %d, attesi 2", len(blocks))
	}
	if blocks[0].WeekNumber != 1 || blocks[1].WeekNumber != 5 {
		t.Errorf("settimane dei blocchi = %d e %d, attese 1 e 5",
			blocks[0].WeekNumber, blocks[1].WeekNumber)
	}
}

// order_in_week si assegna da solo, in coda al blocco: è l'ordine con cui gli
// allenamenti si propongono nella settimana.
func TestDayOrderIsAssignedInSequence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewCatalogRepo(pool)
	id := newTemplate(t, pool, repo)

	for i, dow := range []int{0, 2, 4} {
		dayID, err := repo.CreateDay(ctx, id, 1, dow, "Giorno", "")
		if err != nil {
			t.Fatalf("CreateDay: %v", err)
		}
		d, err := repo.FindDay(ctx, id, dayID)
		if err != nil {
			t.Fatalf("FindDay: %v", err)
		}
		if d.OrderInWeek != i {
			t.Errorf("order_in_week = %d, atteso %d", d.OrderInWeek, i)
		}
	}
}

// ---- righe di esercizio ----------------------------------------------------

// Lo scambio di posizione passa da un indice temporaneo perché il vincolo
// UNIQUE (template_workout_id, order_index) è immediato: assegnare alla prima
// riga l'indice della seconda, prima che la seconda si sposti, va in conflitto.
func TestMoveExerciseRowSwapsPositions(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewCatalogRepo(pool)
	id := newTemplate(t, pool, repo)
	dayID, err := repo.CreateDay(ctx, id, 1, 0, "Giorno 1", "")
	if err != nil {
		t.Fatalf("CreateDay: %v", err)
	}

	var names []string
	for _, n := range []string{"Primo", "Secondo", "Terzo"} {
		name := uniqueName(n)
		names = append(names, name)
		exID, err := repo.CreateExercise(ctx, name, "Test", "compound")
		if err != nil {
			t.Fatalf("CreateExercise: %v", err)
		}
		t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, exID) })
		if err := repo.AddExerciseRow(ctx, dayID, repository.ExerciseRowInput{
			ExerciseID: exID, Sets: 3, TargetReps: 8, RestSeconds: 90}); err != nil {
			t.Fatalf("AddExerciseRow: %v", err)
		}
	}

	order := func() []string {
		d, err := repo.FindDay(ctx, id, dayID)
		if err != nil {
			t.Fatalf("FindDay: %v", err)
		}
		var out []string
		for _, x := range d.Exercises {
			out = append(out, x.ExerciseName)
		}
		return out
	}

	got := order()
	if got[0] != names[0] || got[2] != names[2] {
		t.Fatalf("ordine iniziale = %v, atteso %v", got, names)
	}

	// Il terzo sale di uno.
	d, _ := repo.FindDay(ctx, id, dayID)
	if err := repo.MoveExerciseRow(ctx, dayID, d.Exercises[2].ID, true); err != nil {
		t.Fatalf("MoveExerciseRow: %v", err)
	}
	got = order()
	if got[1] != names[2] || got[2] != names[1] {
		t.Fatalf("dopo lo spostamento = %v, atteso il terzo in mezzo", got)
	}

	// In cima non si muove oltre, e non è un errore.
	d, _ = repo.FindDay(ctx, id, dayID)
	if err := repo.MoveExerciseRow(ctx, dayID, d.Exercises[0].ID, true); err != nil {
		t.Fatalf("spostare in su il primo: %v", err)
	}
	if got2 := order(); got2[0] != got[0] {
		t.Errorf("il primo si è mosso: %v -> %v", got, got2)
	}
}

// Il carico prescritto deve sopravvivere al giro completo: si scrive come
// frazione, si rilegge come frazione, e da lì CreateFromTemplate ricava i chili.
func TestExerciseRowKeepsPrescribedLoad(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewCatalogRepo(pool)
	id := newTemplate(t, pool, repo)
	dayID, err := repo.CreateDay(ctx, id, 1, 0, "Giorno 1", "")
	if err != nil {
		t.Fatalf("CreateDay: %v", err)
	}
	exID, err := repo.CreateExercise(ctx, uniqueName("Squat prescritto"), "Gambe", "compound")
	if err != nil {
		t.Fatalf("CreateExercise: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM exercises WHERE id = $1`, exID) })

	pct, off := 0.65, 5.0
	if err := repo.AddExerciseRow(ctx, dayID, repository.ExerciseRowInput{
		ExerciseID: exID, Sets: 4, TargetReps: 10, RestSeconds: 180,
		LoadPct: &pct, LoadOffsetKg: &off}); err != nil {
		t.Fatalf("AddExerciseRow: %v", err)
	}

	d, err := repo.FindDay(ctx, id, dayID)
	if err != nil {
		t.Fatalf("FindDay: %v", err)
	}
	row := d.Exercises[0]
	if !row.Prescribed() {
		t.Fatal("Prescribed() = false per una riga con load_pct")
	}
	if *row.LoadPct != pct || *row.LoadOffsetKg != off {
		t.Errorf("carico = %v/%v, atteso %v/%v", *row.LoadPct, *row.LoadOffsetKg, pct, off)
	}

	// E il giro fino al programma dell'atleta: 65% di 100 più 5 = 70.
	programs := repository.NewProgramRepo(pool)
	summary, err := repo.FindTemplate(ctx, id)
	if err != nil {
		t.Fatalf("FindTemplate: %v", err)
	}
	userID := makeUser(t, pool, uniqueEmail("prescritto"))
	programID, err := programs.CreateFromTemplate(ctx, userID, summary.PlanTemplate, 100)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	var load float64
	if err := pool.QueryRow(ctx,
		`SELECT pwe.load_kg FROM program_workout_exercises pwe
		 JOIN program_workouts pw ON pw.id = pwe.workout_id
		 WHERE pw.program_id = $1`, programID).Scan(&load); err != nil {
		t.Fatalf("lettura carico: %v", err)
	}
	if load != 70 {
		t.Errorf("carico materializzato = %v, atteso 70 (65%% di 100 + 5)", load)
	}
}
