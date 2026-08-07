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

// I template sono periodizzati a blocchi dalla 010: lo stesso giorno esiste in
// più settimane. La copia in CreateFromTemplate riaggancia gli esercizi con un
// join, e finché i template avevano un blocco solo quel join poteva stare sul
// solo day_of_week senza che si vedesse. Con due blocchi lo stesso lunedì
// esiste due volte e un join senza settimana diventa un prodotto cartesiano:
// ogni giorno si prenderebbe gli esercizi di tutti i blocchi.
//
// Serve un database vero: è SQL, non logica Go.
func TestCreateFromTemplateKeepsBlocksSeparate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)
	userID := makeUser(t, pool, uniqueEmail("blocchi"))

	// str-5x5: 12 settimane, tre blocchi (1, 5, 9), tre giorni ciascuno.
	tmpl := templateByID(t, pool, "str-5x5")
	programID, err := repo.CreateFromTemplate(ctx, userID, tmpl, 0)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	// Il conteggio è la spia del cartesiano: 9 giorni in tutto, e ogni giorno
	// del 5×5 ha esattamente 3 esercizi. Con il join sbagliato ognuno ne
	// prenderebbe 9.
	type row struct{ week, day, exercises int }
	rows, err := pool.Query(ctx, `
		SELECT pw.week_number, pw.day_of_week, count(pwe.id)
		FROM program_workouts pw
		LEFT JOIN program_workout_exercises pwe ON pwe.workout_id = pw.id
		WHERE pw.program_id = $1
		GROUP BY 1,2 ORDER BY 1,2`, programID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.week, &r.day, &r.exercises); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	want := []row{
		{1, 0, 3}, {1, 2, 3}, {1, 4, 3},
		{5, 0, 3}, {5, 2, 3}, {5, 4, 3},
		{9, 0, 3}, {9, 2, 3}, {9, 4, 3},
	}
	if len(got) != len(want) {
		t.Fatalf("giorni copiati = %d, ne volevo %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("giorno %d = %+v, volevo %+v", i, got[i], want[i])
		}
	}
}

// Ogni blocco deve arrivare con lo schema che gli spetta, non con quello del
// blocco 1: è il punto di tutta la periodizzazione. str-5x5 va 5×5 -> 5×3 ->
// 3×3 sul Back Squat.
func TestCreateFromTemplateCopiesEachBlockScheme(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)
	userID := makeUser(t, pool, uniqueEmail("schemi"))

	tmpl := templateByID(t, pool, "str-5x5")
	programID, err := repo.CreateFromTemplate(ctx, userID, tmpl, 0)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	for _, tc := range []struct{ week, sets, reps int }{
		{1, 5, 5},
		{5, 5, 3},
		{9, 3, 3},
	} {
		var sets, reps int
		err := pool.QueryRow(ctx, `
			SELECT pwe.sets, pwe.target_reps
			FROM program_workouts pw
			JOIN program_workout_exercises pwe ON pwe.workout_id = pw.id
			JOIN exercises e ON e.id = pwe.exercise_id
			WHERE pw.program_id = $1 AND pw.week_number = $2
			  AND pw.day_of_week = 0 AND e.name = 'Back Squat'`,
			programID, tc.week).Scan(&sets, &reps)
		if err != nil {
			t.Fatalf("settimana %d: %v", tc.week, err)
		}
		if sets != tc.sets || reps != tc.reps {
			t.Errorf("settimana %d: Back Squat %d×%d, volevo %d×%d",
				tc.week, sets, reps, tc.sets, tc.reps)
		}
	}
}

// GetWorkoutsForWeek risolve con MAX(week_number) <= settimana chiesta: un
// blocco vale da dove è definito fino al successivo. È quello che permette di
// descrivere 12 settimane con tre blocchi invece che con dodici copie.
func TestGetWorkoutsForWeekCarriesBlockForward(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)
	userID := makeUser(t, pool, uniqueEmail("riporto"))

	tmpl := templateByID(t, pool, "str-5x5")
	programID, err := repo.CreateFromTemplate(ctx, userID, tmpl, 0)
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}

	// settimana chiesta -> blocco che deve rispondere
	for _, tc := range []struct{ asked, block int }{
		{1, 1}, {4, 1}, // ultimo giorno prima del cambio
		{5, 5}, {8, 5},
		{9, 9}, {12, 9}, // fino in fondo al programma
	} {
		workouts, err := repo.GetWorkoutsForWeek(ctx, programID, tc.asked)
		if err != nil {
			t.Fatalf("settimana %d: %v", tc.asked, err)
		}
		if len(workouts) != tmpl.DaysPerWeek {
			t.Fatalf("settimana %d: %d allenamenti, ne volevo %d",
				tc.asked, len(workouts), tmpl.DaysPerWeek)
		}
		for _, w := range workouts {
			if w.WeekNumber != tc.block {
				t.Errorf("settimana %d: %q dal blocco %d, volevo il %d",
					tc.asked, w.Name, w.WeekNumber, tc.block)
			}
		}
	}
}

// Un template a blocco unico deve continuare a comportarsi come prima della
// 009: si copia una volta e vale per tutta la durata. Nessuno dei dieci lo è
// più dopo la 010, quindi si costruisce il caso a mano.
func TestGetWorkoutsForWeekRepeatsASingleBlock(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)
	userID := makeUser(t, pool, uniqueEmail("bloccounico"))

	var programID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO user_programs (user_id, name, goal, level, days_per_week, total_weeks)
		 VALUES ($1,'Solo settimana 1','strength','beginner',1,8) RETURNING id`,
		userID).Scan(&programID); err != nil {
		t.Fatalf("programma: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO program_workouts (program_id, name, focus, day_of_week, week_number, order_in_week)
		 VALUES ($1,'Full Body','Tutto',0,1,0)`, programID); err != nil {
		t.Fatalf("allenamento: %v", err)
	}

	for _, week := range []int{1, 2, 5, 8, 99} {
		workouts, err := repo.GetWorkoutsForWeek(ctx, programID, week)
		if err != nil {
			t.Fatalf("settimana %d: %v", week, err)
		}
		if len(workouts) != 1 || workouts[0].WeekNumber != 1 {
			t.Errorf("settimana %d: %+v, volevo l'unico blocco della settimana 1", week, workouts)
		}
	}
}

// uniqueEmail tiene i test rieseguibili sullo stesso database: users.email è
// UNIQUE, e un indirizzo fisso fa fallire la seconda esecuzione sulla creazione
// dell'utente invece che su ciò che il test verifica davvero.
func uniqueEmail(prefix string) string {
	return prefix + "-" + time.Now().Format("150405.000000") + "@example.com"
}

// templateByID pesca un template dal catalogo vero. CreateFromTemplate copia i
// campi che riceve dentro user_programs, quindi vanno letti da lì: un template
// costruito a mano nel test non direbbe niente sui dati che vengono seminati.
func templateByID(t *testing.T, pool *pgxpool.Pool, id string) model.PlanTemplate {
	t.Helper()
	templates, err := repository.NewProgramRepo(pool).GetTemplates(context.Background())
	if err != nil {
		t.Fatalf("lettura template: %v", err)
	}
	for _, tmpl := range templates {
		if tmpl.ID == id {
			return tmpl
		}
	}
	t.Fatalf("template %q non è nel catalogo", id)
	return model.PlanTemplate{}
}

// FindOrCreateExercise è la porta da cui entrano gli esercizi generati dall'AI.
// La categoria che scrive decide di quanto salirà il carico, quindi deve
// arrivare in tabella — e un valore che non è né compound né isolation non deve
// arrivarci affatto.
func TestFindOrCreateExerciseStoresCategory(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)
	suffix := time.Now().Format("150405.000000")

	for _, tc := range []struct{ name, given, want string }{
		{"Finta Alzata " + suffix, "isolation", "isolation"},
		{"Finto Stacco " + suffix, "compound", "compound"},
		{"Finta Cosa " + suffix, "", "compound"},      // non detto
		{"Finta Altra " + suffix, "asdf", "compound"}, // non riconosciuto
	} {
		id, err := repo.FindOrCreateExercise(ctx, pool, tc.name, "Test", tc.given)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var got string
		if err := pool.QueryRow(ctx, `SELECT category FROM exercises WHERE id = $1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("categoria %q -> in tabella %q, volevo %q", tc.given, got, tc.want)
		}
	}
}

// Il ciclo di squat prescrive il carico invece di lasciarlo autoregolare. Il
// conto si fa una volta sola, alla creazione: percentuale del massimale più la
// quota assoluta. Qui si verifica contro i numeri dei fogli del coach.
func TestCreateFromTemplateMaterializesPrescribedLoads(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)

	for _, tc := range []struct {
		template string
		oneRM    float64
		// settimana, giorno, ordine -> carico atteso (dai PDF)
		want map[[3]int]float64
	}{
		{
			template: "squat-170", oneRM: 170,
			want: map[[3]int]float64{
				{1, 0, 0}: 110.5, {1, 3, 0}: 119,
				{4, 3, 0}: 149, {5, 3, 0}: 155.5, {6, 0, 0}: 155.5,
				{7, 0, 0}: 165.5, {7, 3, 0}: 170.5,
				{8, 0, 0}: 140, {8, 0, 1}: 160,
				{8, 3, 0}: 145, {8, 3, 1}: 155, {8, 3, 2}: 170,
			},
		},
		{
			template: "squat-180", oneRM: 180,
			want: map[[3]int]float64{
				{1, 0, 0}: 117, {1, 3, 0}: 126,
				{5, 3, 0}: 167, {6, 0, 0}: 162, {6, 3, 0}: 172,
				{7, 0, 0}: 177, {7, 3, 0}: 182,
				{8, 0, 0}: 150, {8, 0, 1}: 170, {8, 3, 2}: 180,
			},
		},
	} {
		t.Run(tc.template, func(t *testing.T) {
			userID := makeUser(t, pool, uniqueEmail(tc.template))
			programID, err := repo.CreateFromTemplate(ctx, userID,
				templateByID(t, pool, tc.template), tc.oneRM)
			if err != nil {
				t.Fatalf("CreateFromTemplate: %v", err)
			}

			for key, want := range tc.want {
				var got *float64
				if err := pool.QueryRow(ctx, `
					SELECT pwe.load_kg
					FROM program_workouts pw
					JOIN program_workout_exercises pwe ON pwe.workout_id = pw.id
					WHERE pw.program_id = $1 AND pw.week_number = $2
					  AND pw.day_of_week = $3 AND pwe.order_index = $4`,
					programID, key[0], key[1], key[2]).Scan(&got); err != nil {
					t.Fatalf("w%dd%d#%d: %v", key[0], key[1], key[2], err)
				}
				if got == nil || *got != want {
					t.Errorf("w%dd%d#%d = %v, volevo %v", key[0], key[1], key[2], got, want)
				}
			}

			// L'ultima singola del test è aperta ("proceed to maximum"): niente
			// carico prescritto, lo sceglie l'atleta.
			var open *float64
			if err := pool.QueryRow(ctx, `
				SELECT pwe.load_kg FROM program_workouts pw
				JOIN program_workout_exercises pwe ON pwe.workout_id = pw.id
				WHERE pw.program_id = $1 AND pw.week_number = 8
				  AND pw.day_of_week = 3 AND pwe.order_index = 3`,
				programID).Scan(&open); err != nil {
				t.Fatalf("singola aperta: %v", err)
			}
			if open != nil {
				t.Errorf("singola finale = %v, volevo nessun carico prescritto", *open)
			}

			// Il complementare resta autoregolato.
			var assist *float64
			if err := pool.QueryRow(ctx, `
				SELECT pwe.load_kg FROM program_workouts pw
				JOIN program_workout_exercises pwe ON pwe.workout_id = pw.id
				WHERE pw.program_id = $1 AND pw.week_number = 1
				  AND pw.day_of_week = 0 AND pwe.order_index = 1`,
				programID).Scan(&assist); err != nil {
				t.Fatalf("complementare: %v", err)
			}
			if assist != nil {
				t.Errorf("complementare = %v, volevo nessun carico prescritto", *assist)
			}
		})
	}
}

// Gli incrementi sono in chili pieni e non si riscalano: fuori finestra la
// template va rifiutata, non adattata.
func TestCreateFromTemplateRejectsAOneRMOutsideTheWindow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := repository.NewProgramRepo(pool)
	tmpl := templateByID(t, pool, "squat-170")

	for _, oneRM := range []float64{120, 144.9, 171} {
		userID := makeUser(t, pool, uniqueEmail("fuorifinestra"))
		if _, err := repo.CreateFromTemplate(ctx, userID, tmpl, oneRM); !errors.Is(err, repository.ErrOneRMOutOfRange) {
			t.Errorf("massimale %v: errore = %v, volevo ErrOneRMOutOfRange", oneRM, err)
		}
	}
	// Dentro finestra passa.
	userID := makeUser(t, pool, uniqueEmail("infinestra"))
	if _, err := repo.CreateFromTemplate(ctx, userID, tmpl, 145); err != nil {
		t.Errorf("massimale 145: %v", err)
	}
}
