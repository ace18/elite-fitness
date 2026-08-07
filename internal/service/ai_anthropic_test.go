package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// sseServer è un finto endpoint Messages che rimanda gli eventi passati e
// registra la richiesta ricevuta, per poterci fare asserzioni sopra.
type sseServer struct {
	*httptest.Server
	body   map[string]any
	betas  string
	closed chan struct{}
}

func newSSEServer(t *testing.T, events [][2]string) *sseServer {
	t.Helper()
	s := &sseServer{closed: make(chan struct{})}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &s.body)
		s.betas = r.Header.Get("anthropic-beta")

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, ev := range events {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev[0], ev[1])
			flusher.Flush()
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func plannerFor(t *testing.T, url string) *AnthropicPlanner {
	t.Helper()
	return &AnthropicPlanner{client: anthropic.NewClient(
		option.WithBaseURL(url),
		option.WithAPIKey("test"),
		option.WithMaxRetries(0),
	)}
}

const minimalPlanJSON = `{"name":"P","goal":"g","level":"l","totalWeeks":8,"daysPerWeek":3,"workouts":[]}`

// toolUseEvents è uno stream completo che consegna un piano dentro un tool_use.
func toolUseEvents(stopReason string) [][2]string {
	return [][2]string{
		{"message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1}}}`},
		{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"create_training_plan","input":{}}}`},
		{"content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":%s}}`, jsonString(minimalPlanJSON))},
		{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		{"message_delta", fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q,"stop_sequence":null},"usage":{"output_tokens":10}}`, stopReason)},
		{"message_stop", `{"type":"message_stop"}`},
	}
}

// Il fallback è tutto nella richiesta: se `fallbacks` o l'header beta non
// partono, il rifiuto non viene riservito da nessuno e la generazione fallisce
// — senza che niente, lato risposta, lo faccia notare. Quindi si guarda il
// corpo che esce, non solo quello che entra.
func TestRequestPlanSendsFallbackConfiguration(t *testing.T) {
	srv := newSSEServer(t, toolUseEvents("tool_use"))

	if _, err := plannerFor(t, srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "intermediate", Days: 3, Length: 60,
	}); err != nil {
		t.Fatalf("RequestPlan: %v", err)
	}

	// L'header beta: senza, l'API ignora (o rifiuta) il parametro.
	if want := "server-side-fallback-2026-06-01"; srv.betas != want {
		t.Errorf("anthropic-beta = %q, volevo %q", srv.betas, want)
	}

	fallbacks, ok := srv.body["fallbacks"].([]any)
	if !ok || len(fallbacks) != 1 {
		t.Fatalf("fallbacks = %#v, volevo una lista con un modello", srv.body["fallbacks"])
	}
	entry, _ := fallbacks[0].(map[string]any)
	if entry["model"] != planFallbackModel {
		t.Errorf("modello di ripiego = %v, volevo %q", entry["model"], planFallbackModel)
	}
	// Il modello richiesto resta quello buono: il ripiego è un ripiego, non
	// una sostituzione.
	if srv.body["model"] != planModel {
		t.Errorf("model = %v, volevo %q", srv.body["model"], planModel)
	}
}

// Quando il primo modello rifiuta, l'API riserve la richiesta con quello di
// ripiego dentro la stessa chiamata: nello stream compare un blocco `fallback`
// e poi il piano. Per chi chiama non è un errore.
func TestRequestPlanAcceptsAPlanServedByTheFallbackModel(t *testing.T) {
	events := [][2]string{
		{"message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-4-8","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1}}}`},
		{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"fallback","from":{"model":"claude-opus-5"},"to":{"model":"claude-opus-4-8"}}}`},
		{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		{"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"create_training_plan","input":{}}}`},
		{"content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":%s}}`, jsonString(minimalPlanJSON))},
		{"content_block_stop", `{"type":"content_block_stop","index":1}`},
		{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":10}}`},
		{"message_stop", `{"type":"message_stop"}`},
	}
	srv := newSSEServer(t, events)

	plan, err := plannerFor(t, srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "intermediate", Days: 3, Length: 60,
	})
	if err != nil {
		t.Fatalf("RequestPlan: %v", err)
	}
	if plan.Name != "P" {
		t.Errorf("piano = %+v, volevo quello servito dal modello di ripiego", plan)
	}
}

// Se rifiuta anche il modello di ripiego la catena è finita. La risposta è
// HTTP 200 con contenuto vuoto, quindi senza guardare stop_reason l'errore
// sarebbe "non ha restituito un piano" — vero ma fuorviante.
func TestRequestPlanReportsARefusalAsSuch(t *testing.T) {
	events := [][2]string{
		{"message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1}}}`},
		{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"refusal","stop_sequence":null},"usage":{"output_tokens":0}}`},
		{"message_stop", `{"type":"message_stop"}`},
	}
	srv := newSSEServer(t, events)

	_, err := plannerFor(t, srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "beginner", Days: 3, Length: 45,
	})
	if !errors.Is(err, ErrPlanRefused) {
		t.Errorf("errore = %v, volevo ErrPlanRefused", err)
	}
}
