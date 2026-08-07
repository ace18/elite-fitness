package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Il piano arriva dentro un tool_use, il cui input viaggia a pezzi in
// input_json_delta: nessun singolo frammento è JSON valido. Se un domani si
// tornasse a leggere i delta invece del Message ricomposto, il piano
// risulterebbe vuoto senza che niente segnali un errore — l'utente vedrebbe
// solo "claude did not return a training plan". Questo test tiene ferma la
// ricomposizione.
func TestRequestPlanAccumulatesStreamedToolUse(t *testing.T) {
	// L'input del tool spezzato a metà, come lo manda l'API.
	chunks := []string{
		`{"name":"Forza 5x5","goal":"strength","level":"intermediate",`,
		`"totalWeeks":12,"daysPerWeek":3,"workouts":[{"name":"Full Body A",`,
		`"focus":"squat","dayOfWeek":0,"weekNumber":5,"exercises":[{"name":"Back Squat",`,
		`"muscleGroup":"legs","category":"compound","sets":5,"targetReps":5,"restSeconds":180},`,
		`{"name":"Lateral Raise","muscleGroup":"shoulders","category":"isolation",`,
		`"sets":3,"targetReps":15,"restSeconds":45}]}]}`,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		send := func(event, data string) {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			flusher.Flush()
		}

		send("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1}}}`)
		send("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"create_training_plan","input":{}}}`)
		for _, c := range chunks {
			send("content_block_delta", fmt.Sprintf(
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":%s}}`,
				jsonString(c)))
		}
		send("content_block_stop", `{"type":"content_block_stop","index":0}`)
		send("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":120}}`)
		send("message_stop", `{"type":"message_stop"}`)
	}))
	defer srv.Close()

	svc := &AIService{client: anthropic.NewClient(
		option.WithBaseURL(srv.URL),
		option.WithAPIKey("test"),
		option.WithMaxRetries(0),
	)}

	plan, err := svc.requestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "intermediate", Days: 3, Length: 60,
	})
	if err != nil {
		t.Fatalf("requestPlan: %v", err)
	}

	if plan.Name != "Forza 5x5" {
		t.Errorf("name = %q, want %q", plan.Name, "Forza 5x5")
	}
	if plan.TotalWeeks != 12 || plan.DaysPerWeek != 3 {
		t.Errorf("totalWeeks/daysPerWeek = %d/%d, want 12/3", plan.TotalWeeks, plan.DaysPerWeek)
	}
	if len(plan.Workouts) != 1 || len(plan.Workouts[0].Exercises) != 2 {
		t.Fatalf("workouts = %+v, want 1 workout with 2 exercises", plan.Workouts)
	}
	if ex := plan.Workouts[0].Exercises[0]; ex.Name != "Back Squat" || ex.Sets != 5 {
		t.Errorf("exercise = %+v, want Back Squat 5 sets", ex)
	}
	// La settimana del blocco viaggia negli stessi delta: se il tag JSON non
	// combaciasse resterebbe 0, e il piano finirebbe tutto nel blocco 1 senza
	// che niente lo segnali.
	if w := plan.Workouts[0].WeekNumber; w != 5 {
		t.Errorf("weekNumber = %d, want 5", w)
	}
	// Stessa storia per la categoria, che decide il passo di progressione: un
	// tag che non combacia la lascia vuota, e da lì in poi ogni esercizio
	// generato passa per complementare — cioè sale a scatti da 2.5 kg.
	// Serve leggerle entrambe: 'compound' da solo coinciderebbe con il ripiego
	// di FindOrCreateExercise e non proverebbe che il valore è arrivato.
	for i, want := range []string{"compound", "isolation"} {
		if got := plan.Workouts[0].Exercises[i].Category; got != want {
			t.Errorf("esercizio %d: category = %q, want %q", i, got, want)
		}
	}
}

// Una risposta senza tool_use non deve passare per buona: senza questo
// controllo savePlan scriverebbe un programma vuoto.
func TestRequestPlanRejectsResponseWithoutPlan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		send := func(event, data string) {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			flusher.Flush()
		}
		send("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1}}}`)
		send("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		send("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"non posso aiutarti"}}`)
		send("content_block_stop", `{"type":"content_block_stop","index":0}`)
		send("message_stop", `{"type":"message_stop"}`)
	}))
	defer srv.Close()

	svc := &AIService{client: anthropic.NewClient(
		option.WithBaseURL(srv.URL),
		option.WithAPIKey("test"),
		option.WithMaxRetries(0),
	)}

	if _, err := svc.requestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "beginner", Days: 3, Length: 45,
	}); err == nil {
		t.Fatal("expected an error when the response carries no training plan")
	}
}

// Un piano periodizzato arriva come blocchi: gli stessi giorni ripetuti alla
// settimana in cui cambia lo schema. Ogni blocco deve numerarsi da capo, se no
// order_in_week smette di voler dire "posizione nella settimana".
func TestLayoutWorkoutsNumbersEachBlockFromZero(t *testing.T) {
	got := layoutWorkouts([]aiWorkout{
		{Name: "Full Body A", DayOfWeek: 0, WeekNumber: 1},
		{Name: "Full Body B", DayOfWeek: 2, WeekNumber: 1},
		{Name: "Full Body C", DayOfWeek: 4, WeekNumber: 1},
		{Name: "Full Body A", DayOfWeek: 0, WeekNumber: 5},
		{Name: "Full Body B", DayOfWeek: 2, WeekNumber: 5},
		{Name: "Full Body C", DayOfWeek: 4, WeekNumber: 5},
	})

	want := []workoutPlacement{
		{Week: 1, OrderInWeek: 0}, {Week: 1, OrderInWeek: 1}, {Week: 1, OrderInWeek: 2},
		{Week: 5, OrderInWeek: 0}, {Week: 5, OrderInWeek: 1}, {Week: 5, OrderInWeek: 2},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("workout %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// I blocchi possono arrivare interlacciati (tutte le varianti di un giorno di
// fila): l'ordine dentro la settimana si conta per settimana, non sull'indice
// globale, quindi deve reggere comunque.
func TestLayoutWorkoutsHandlesInterleavedBlocks(t *testing.T) {
	got := layoutWorkouts([]aiWorkout{
		{DayOfWeek: 0, WeekNumber: 1},
		{DayOfWeek: 0, WeekNumber: 4},
		{DayOfWeek: 2, WeekNumber: 1},
		{DayOfWeek: 2, WeekNumber: 4},
	})

	want := []workoutPlacement{
		{Week: 1, OrderInWeek: 0},
		{Week: 4, OrderInWeek: 0},
		{Week: 1, OrderInWeek: 1},
		{Week: 4, OrderInWeek: 1},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("workout %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// weekNumber è `required` nello schema, ma la risposta del modello non è un
// contratto: se manca, JSON lo lascia a 0. Righe a settimana 0 verrebbero
// scelte lo stesso per la settimana 1 (0 <= 1 in GetWorkoutsForWeek), quindi il
// piano sembrerebbe a posto fino al primo blocco vero, che si ritroverebbe il
// blocco 0 incollato davanti.
func TestLayoutWorkoutsLiftsMissingWeekToOne(t *testing.T) {
	got := layoutWorkouts([]aiWorkout{
		{DayOfWeek: 0},                 // weekNumber assente -> 0
		{DayOfWeek: 2, WeekNumber: -3}, // e nemmeno negativo
		{DayOfWeek: 4, WeekNumber: 1},
	})

	for i, p := range got {
		if p.Week != 1 {
			t.Errorf("workout %d: week = %d, want 1", i, p.Week)
		}
		if p.OrderInWeek != i {
			t.Errorf("workout %d: orderInWeek = %d, want %d", i, p.OrderInWeek, i)
		}
	}
}

// jsonString incapsula s come stringa JSON, virgolette comprese.
func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
