package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	deepseekDefaultBaseURL = "https://api.deepseek.com"

	// deepseekDefaultModel: il modello di chat generalista. Il "reasoner" di
	// DeepSeek non è intercambiabile — storicamente non supporta la chiamata a
	// funzione, che qui è tutto il meccanismo — quindi non lo si mette come
	// default. Il campo resta configurabile per poterlo provare.
	deepseekDefaultModel = "deepseek-chat"

	// Il tetto di output di DeepSeek è molto più basso di quello di Opus 5
	// (8192 contro 32000). Non è una manopola da alzare: è il limite del
	// modello. Vedi il commento su finish_reason "length" più sotto — un piano
	// grosso ci sta stretto, e quando non ci sta bisogna accorgersene.
	deepseekMaxTokens = 8192

	// Nessuno stream, a differenza di Anthropic. Lì lo streaming serviva
	// perché il budget alto faceva scattare i timeout HTTP dell'SDK; qui il
	// timeout lo decidiamo noi e la generazione gira comunque in background
	// (vedi planjob.go), quindi si evita di ricomporre i delta per niente.
	deepseekTimeout = 8 * time.Minute
)

// DeepSeekPlanner genera i piani con DeepSeek.
//
// Il formato di richiesta è quello di OpenAI (`/chat/completions`, `tools` con
// `function`, `tool_choice`), che DeepSeek serve compatibile: cambiando BaseURL
// e Model questo stesso codice punta a qualunque endpoint della stessa
// famiglia. Si parla HTTP a mano invece di aggiungere un SDK perché serve una
// forma di chiamata sola.
type DeepSeekPlanner struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewDeepSeekPlanner(apiKey string) *DeepSeekPlanner {
	return &DeepSeekPlanner{
		APIKey:  apiKey,
		BaseURL: deepseekDefaultBaseURL,
		Model:   deepseekDefaultModel,
		HTTP:    &http.Client{Timeout: deepseekTimeout},
	}
}

// Le forme della richiesta e della risposta, ridotte ai campi che servono.
type openAIChatRequest struct {
	Model      string           `json:"model"`
	MaxTokens  int              `json:"max_tokens"`
	Messages   []openAIMessage  `json:"messages"`
	Tools      []openAITool     `json:"tools"`
	ToolChoice openAIToolChoice `json:"tool_choice"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openAIToolChoice struct {
	Type     string            `json:"type"`
	Function map[string]string `json:"function"`
}

type openAIChatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			ToolCalls []struct {
				Function struct {
					Name string `json:"name"`
					// Arguments è una stringa che contiene JSON, non un
					// oggetto: è la differenza di forma più insidiosa rispetto
					// ad Anthropic, dove l'input del tool arriva già come
					// oggetto. Va deserializzato due volte.
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *DeepSeekPlanner) RequestPlan(ctx context.Context, input GeneratePlanInput) (aiPlanOutput, error) {
	var planData aiPlanOutput

	properties, required := planToolSchema()
	body, err := json.Marshal(openAIChatRequest{
		Model:     p.model(),
		MaxTokens: deepseekMaxTokens,
		Messages: []openAIMessage{
			{Role: "user", Content: buildPlanPrompt(input)},
		},
		Tools: []openAITool{{
			Type: "function",
			Function: openAIFunction{
				Name:        planToolName,
				Description: planToolDescription,
				// Lo schema è lo stesso oggetto che l'implementazione Anthropic
				// mette in `input_schema`: qui va sotto `parameters` senza una
				// riga di conversione.
				Parameters: map[string]any{
					"type":       "object",
					"properties": properties,
					"required":   required,
				},
			},
		}},
		// Chiamata forzata: senza, il modello può rispondere in prosa e il
		// piano non arriva mai.
		ToolChoice: openAIToolChoice{
			Type:     "function",
			Function: map[string]string{"name": planToolName},
		},
	})
	if err != nil {
		return planData, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return planData, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return planData, fmt.Errorf("deepseek API: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return planData, fmt.Errorf("deepseek API: %w", err)
	}

	var out openAIChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		// Un corpo che non è nemmeno JSON di solito è un errore di un proxy
		// davanti all'API: si riporta lo stato, che dice più del parse error.
		return planData, fmt.Errorf("deepseek API: risposta illeggibile (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != nil {
			return planData, fmt.Errorf("deepseek API: HTTP %d: %s", resp.StatusCode, out.Error.Message)
		}
		return planData, fmt.Errorf("deepseek API: HTTP %d", resp.StatusCode)
	}
	if len(out.Choices) == 0 {
		return planData, fmt.Errorf("deepseek did not return a training plan")
	}

	choice := out.Choices[0]
	switch choice.FinishReason {
	case "content_filter":
		// L'equivalente del rifiuto di Anthropic. Qui non c'è un fallback lato
		// server: chi chiama vede l'errore.
		return planData, ErrPlanRefused
	case "length":
		// Il piano è stato troncato a metà: `arguments` è JSON incompleto e
		// deserializzarlo darebbe un "unexpected end of JSON input" che non
		// dice a nessuno cosa è successo. Il tetto di output di DeepSeek è
		// basso e un programma con molti giorni e più blocchi ci arriva
		// davvero, quindi vale la pena dirlo chiaramente.
		return planData, fmt.Errorf(
			"deepseek: piano troncato al tetto di %d token di output — programma troppo grande per questo modello",
			deepseekMaxTokens)
	}

	if len(choice.Message.ToolCalls) == 0 {
		return planData, fmt.Errorf("deepseek did not return a training plan")
	}
	if err := json.Unmarshal([]byte(choice.Message.ToolCalls[0].Function.Arguments), &planData); err != nil {
		return planData, fmt.Errorf("deepseek: argomenti della chiamata illeggibili: %w", err)
	}
	if planData.Name == "" {
		return planData, fmt.Errorf("deepseek did not return a training plan")
	}
	return planData, nil
}

// I default stanno nei getter così che un DeepSeekPlanner costruito a mano —
// nei test, o puntato a un altro endpoint compatibile — funzioni anche con i
// campi lasciati vuoti.
func (p *DeepSeekPlanner) baseURL() string {
	if p.BaseURL == "" {
		return deepseekDefaultBaseURL
	}
	return p.BaseURL
}

func (p *DeepSeekPlanner) model() string {
	if p.Model == "" {
		return deepseekDefaultModel
	}
	return p.Model
}

func (p *DeepSeekPlanner) httpClient() *http.Client {
	if p.HTTP == nil {
		return &http.Client{Timeout: deepseekTimeout}
	}
	return p.HTTP
}
