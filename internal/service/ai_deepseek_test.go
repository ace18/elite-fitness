package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// deepseekStub è un finto /chat/completions: risponde con quello che gli si
// dice e conserva la richiesta ricevuta.
type deepseekStub struct {
	*httptest.Server
	body map[string]any
	auth string
}

func newDeepSeekStub(t *testing.T, status int, response string) *deepseekStub {
	t.Helper()
	s := &deepseekStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &s.body)
		s.auth = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, volevo /chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, response)
	}))
	t.Cleanup(s.Close)
	return s
}

func deepseekPlannerFor(url string) *DeepSeekPlanner {
	return &DeepSeekPlanner{APIKey: "test-key", BaseURL: url}
}

// toolCallResponse costruisce una risposta in cui il piano arriva come stringa
// dentro `arguments`, che è la forma vera dell'API.
func toolCallResponse(t *testing.T, finishReason, argumentsJSON string) string {
	t.Helper()
	encoded, err := json.Marshal(argumentsJSON) // la stringa, non l'oggetto
	if err != nil {
		t.Fatal(err)
	}
	return `{"choices":[{"finish_reason":"` + finishReason + `","message":{"tool_calls":[{"function":{"name":"create_training_plan","arguments":` + string(encoded) + `}}]}}]}`
}

// La richiesta è in formato OpenAI: se `tools`/`tool_choice` non partono nella
// forma giusta il modello risponde in prosa e il piano non arriva mai.
func TestDeepSeekSendsAForcedFunctionCall(t *testing.T) {
	srv := newDeepSeekStub(t, http.StatusOK, toolCallResponse(t, "tool_calls", minimalPlanJSON))

	if _, err := deepseekPlannerFor(srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "intermediate", Days: 3, Length: 60,
	}); err != nil {
		t.Fatalf("RequestPlan: %v", err)
	}

	if srv.auth != "Bearer test-key" {
		t.Errorf("Authorization = %q, volevo un bearer token", srv.auth)
	}
	if srv.body["model"] != deepseekDefaultModel {
		t.Errorf("model = %v, volevo %q", srv.body["model"], deepseekDefaultModel)
	}

	tools, _ := srv.body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, volevo una funzione sola", srv.body["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	if tool["type"] != "function" {
		t.Errorf("tools[0].type = %v, volevo \"function\"", tool["type"])
	}
	fn, _ := tool["function"].(map[string]any)
	if fn["name"] != planToolName {
		t.Errorf("nome della funzione = %v, volevo %q", fn["name"], planToolName)
	}
	// Lo schema neutro finisce sotto `parameters`, non `input_schema`.
	params, _ := fn["parameters"].(map[string]any)
	if params["type"] != "object" || params["properties"] == nil {
		t.Errorf("parameters = %#v, volevo lo schema JSON del piano", params)
	}

	// Chiamata forzata: è ciò che impedisce una risposta in prosa.
	choice, _ := srv.body["tool_choice"].(map[string]any)
	if choice["type"] != "function" {
		t.Fatalf("tool_choice = %#v, volevo una chiamata forzata", srv.body["tool_choice"])
	}
	if f, _ := choice["function"].(map[string]any); f["name"] != planToolName {
		t.Errorf("tool_choice.function.name = %v, volevo %q", f["name"], planToolName)
	}
}

// La differenza di forma che rompe di più rispetto ad Anthropic: qui il piano
// è una stringa dentro `arguments`, non un oggetto. Chi lo trattasse come
// oggetto otterrebbe un piano vuoto senza errori.
func TestDeepSeekParsesArgumentsDeliveredAsAString(t *testing.T) {
	plan := `{"name":"Forza","goal":"strength","level":"intermediate","totalWeeks":12,"daysPerWeek":3,` +
		`"workouts":[{"name":"Full Body A","focus":"squat","dayOfWeek":0,"weekNumber":5,` +
		`"exercises":[{"name":"Back Squat","muscleGroup":"legs","category":"compound","sets":5,"targetReps":3,"restSeconds":240}]}]}`
	srv := newDeepSeekStub(t, http.StatusOK, toolCallResponse(t, "tool_calls", plan))

	got, err := deepseekPlannerFor(srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "intermediate", Days: 3, Length: 60,
	})
	if err != nil {
		t.Fatalf("RequestPlan: %v", err)
	}
	if got.Name != "Forza" || got.TotalWeeks != 12 {
		t.Fatalf("piano = %+v", got)
	}
	// I campi che contano per la periodizzazione devono sopravvivere al doppio
	// passaggio (stringa -> JSON -> struct) come su Anthropic.
	if w := got.Workouts[0]; w.WeekNumber != 5 || w.Exercises[0].Category != "compound" {
		t.Errorf("workout = %+v, volevo weekNumber 5 e categoria compound", w)
	}
}

// Il tetto di output di DeepSeek è basso e un piano periodizzato grosso ci
// arriva. Troncato, `arguments` è JSON a metà: senza questo controllo l'errore
// sarebbe "unexpected end of JSON input", che non dice a nessuno di alleggerire
// il programma.
func TestDeepSeekReportsATruncatedPlanClearly(t *testing.T) {
	truncated := `{"name":"Forza","workouts":[{"name":"Full Bo`
	srv := newDeepSeekStub(t, http.StatusOK, toolCallResponse(t, "length", truncated))

	_, err := deepseekPlannerFor(srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "intermediate", Days: 6, Length: 60,
	})
	if err == nil {
		t.Fatal("un piano troncato deve dare errore")
	}
	if !strings.Contains(err.Error(), "troncato") {
		t.Errorf("errore = %v, volevo che dicesse che il piano è troncato", err)
	}
}

// L'equivalente del rifiuto di Anthropic, così il chiamante distingue i due
// casi allo stesso modo qualunque sia il fornitore.
func TestDeepSeekMapsContentFilterToRefusal(t *testing.T) {
	srv := newDeepSeekStub(t, http.StatusOK,
		`{"choices":[{"finish_reason":"content_filter","message":{"tool_calls":[]}}]}`)

	_, err := deepseekPlannerFor(srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "beginner", Days: 3, Length: 45,
	})
	if !errors.Is(err, ErrPlanRefused) {
		t.Errorf("errore = %v, volevo ErrPlanRefused", err)
	}
}

// Un errore dell'API deve arrivare con il suo messaggio: è quello che finisce
// nei log quando una generazione fallisce.
func TestDeepSeekSurfacesAPIErrors(t *testing.T) {
	srv := newDeepSeekStub(t, http.StatusUnauthorized,
		`{"error":{"message":"Authentication Fails","type":"authentication_error"}}`)

	_, err := deepseekPlannerFor(srv.URL).RequestPlan(context.Background(), GeneratePlanInput{
		Goal: "strength", Level: "beginner", Days: 3, Length: 45,
	})
	if err == nil {
		t.Fatal("un 401 deve dare errore")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Authentication Fails") {
		t.Errorf("errore = %v, volevo stato e messaggio dell'API", err)
	}
}

// DeepSeekPlanner deve soddisfare l'interfaccia esattamente come quello
// Anthropic — è il senso di averla estratta.
var _ PlanGenerator = (*DeepSeekPlanner)(nil)
var _ PlanGenerator = (*AnthropicPlanner)(nil)
