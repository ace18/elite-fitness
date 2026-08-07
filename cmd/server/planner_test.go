package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/elitecoach/backend/internal/config"
)

// La scelta del fornitore fallisce all'avvio, non alla prima generazione: un
// PLAN_PROVIDER scritto male scoperto dopo che un utente ha aspettato minuti è
// il modo peggiore di accorgersene.
func TestNewPlannerSelectsTheConfiguredProvider(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cfg      config.Config
		wantErr  string
		wantType string
	}{
		{
			name:     "anthropic",
			cfg:      config.Config{PlanProvider: "anthropic", AnthropicKey: "k"},
			wantType: "*service.AnthropicPlanner",
		},
		{
			// L'SDK di Anthropic legge la chiave dall'ambiente per conto suo,
			// quindi qui una chiave vuota non è un errore.
			name:     "anthropic senza chiave esplicita",
			cfg:      config.Config{PlanProvider: "anthropic"},
			wantType: "*service.AnthropicPlanner",
		},
		{
			name:     "deepseek",
			cfg:      config.Config{PlanProvider: "deepseek", DeepSeekKey: "k"},
			wantType: "*service.DeepSeekPlanner",
		},
		{
			// Qui invece la chiave serve: non c'è nessun fallback d'ambiente.
			name:    "deepseek senza chiave",
			cfg:     config.Config{PlanProvider: "deepseek"},
			wantErr: "DEEPSEEK_API_KEY",
		},
		{
			name:    "fornitore sconosciuto",
			cfg:     config.Config{PlanProvider: "chatgpt"},
			wantErr: "non riconosciuto",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			planner, err := newPlanner(&tc.cfg)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("nessun errore, ne volevo uno che citasse %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("errore = %v, volevo che citasse %q", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("newPlanner: %v", err)
			}
			if got := fmt.Sprintf("%T", planner); got != tc.wantType {
				t.Errorf("fornitore = %s, volevo %s", got, tc.wantType)
			}
		})
	}
}
