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
		`"focus":"squat","dayOfWeek":0,"exercises":[{"name":"Back Squat",`,
		`"muscleGroup":"legs","sets":5,"targetReps":5,"restSeconds":180}]}]}`,
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
	if len(plan.Workouts) != 1 || len(plan.Workouts[0].Exercises) != 1 {
		t.Fatalf("workouts = %+v, want 1 workout with 1 exercise", plan.Workouts)
	}
	if ex := plan.Workouts[0].Exercises[0]; ex.Name != "Back Squat" || ex.Sets != 5 {
		t.Errorf("exercise = %+v, want Back Squat 5 sets", ex)
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

// jsonString incapsula s come stringa JSON, virgolette comprese.
func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
