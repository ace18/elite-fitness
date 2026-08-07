package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	planModel = "claude-opus-5"

	// planFallbackModel è il modello a cui passare quando i classificatori di
	// sicurezza rifiutano la richiesta. Su Opus 5 il rifiuto non è un errore
	// HTTP: la risposta arriva con stop_reason "refusal" e — senza fallback —
	// il piano semplicemente non si genera.
	//
	// Un programma di allenamento non è materia di frontiera, ma i falsi
	// positivi esistono e qui costano un lavoro intero: l'utente ha aspettato
	// minuti e si ritrova un errore. Opus 4.8 è il bersaglio raccomandato per
	// questa categoria di rifiuti.
	//
	// Da notare: il fallback scatta SOLO sul rifiuto. Rate limit, sovraccarico
	// e errori del server tornano com'erano — non è una protezione contro i
	// disservizi.
	planFallbackModel = "claude-opus-4-8"

	// Il budget è alto perché su Opus 5 il thinking è attivo di default e
	// condivide MaxTokens con la risposta. Da lì discende lo streaming: sopra
	// ~16k token una richiesta non-streaming rischia di sbattere nei timeout
	// HTTP dell'SDK prima ancora di avere una risposta.
	planMaxTokens = 32000
)

// AnthropicPlanner genera i piani con Claude. È l'implementazione di
// PlanGenerator usata in produzione.
type AnthropicPlanner struct {
	client anthropic.Client
}

func NewAnthropicPlanner(apiKey string) *AnthropicPlanner {
	// senza opts il client legge ANTHROPIC_API_KEY dall'ambiente
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return &AnthropicPlanner{client: anthropic.NewClient(opts...)}
}

// planTool incarta lo schema neutro nel tipo dell'SDK. Le proprietà non si
// riscrivono qui: arrivano da planToolSchema() e sono le stesse che userebbe
// un altro fornitore.
func planTool() anthropic.BetaToolParam {
	properties, required := planToolSchema()
	return anthropic.BetaToolParam{
		Name:        planToolName,
		Description: anthropic.String(planToolDescription),
		InputSchema: anthropic.BetaToolInputSchemaParam{
			Properties: properties,
			Required:   required,
		},
	}
}

func (p *AnthropicPlanner) RequestPlan(ctx context.Context, input GeneratePlanInput) (aiPlanOutput, error) {
	var planData aiPlanOutput
	tool := planTool()

	// Si passa dall'endpoint beta perché è l'unico che accetta `fallbacks`.
	// Costa il prefisso Beta su tutti i tipi della richiesta; in cambio un
	// rifiuto viene riservito da un altro modello dentro la stessa chiamata,
	// senza logica di riprova da questa parte.
	stream := p.client.Beta.Messages.NewStreaming(ctx, anthropic.BetaMessageNewParams{
		Model:     planModel,
		MaxTokens: planMaxTokens,
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_06_01},
		Fallbacks: []anthropic.BetaFallbackParam{{Model: planFallbackModel}},
		Tools: []anthropic.BetaToolUnionParam{
			{OfTool: &tool},
		},
		ToolChoice: anthropic.BetaToolChoiceParamOfTool(planToolName),
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(buildPlanPrompt(input))),
		},
	})

	// Accumulate ricompone i delta in un Message completo, come se fosse
	// arrivato in un colpo solo: l'input del tool_use arriva a pezzi
	// (input_json_delta) ed è JSON valido solo a blocco chiuso.
	var resp anthropic.BetaMessage
	for stream.Next() {
		if err := resp.Accumulate(stream.Current()); err != nil {
			return planData, fmt.Errorf("claude API: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return planData, fmt.Errorf("claude API: %w", err)
	}

	for _, block := range resp.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.BetaFallbackBlock:
			// Il piano l'ha scritto il modello di ripiego. Non è un errore, ma
			// vale la pena saperlo: se compare spesso, è il prompt a innescare
			// i classificatori, non un caso isolato.
			//
			// In streaming questo blocco è il segnale completo: il routing
			// "appiccicoso" dei turni successivi non viene consultato sugli
			// stream, quindi non c'è un altro posto dove guardare.
			log.Printf("piano: %s ha rifiutato, ha risposto %s",
				variant.From.Model, variant.To.Model)
		case anthropic.BetaToolUseBlock:
			// Nel namespace beta Input è `any`, non json.RawMessage: si rilegge
			// il JSON grezzo del campo invece di ri-serializzare la mappa che
			// l'SDK ha già decodificato.
			if err := json.Unmarshal([]byte(variant.JSON.Input.Raw()), &planData); err != nil {
				return planData, err
			}
		}
	}

	// Rifiuto anche dopo il fallback: distinguerlo serve nei log, perché la
	// risposta è a tutti gli effetti riuscita — HTTP 200, contenuto vuoto.
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return planData, ErrPlanRefused
	}
	if planData.Name == "" {
		return planData, fmt.Errorf("claude did not return a training plan")
	}
	return planData, nil
}
