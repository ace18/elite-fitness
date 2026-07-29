package service

import (
	"context"
	"encoding/json"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIService struct {
	client   anthropic.Client
	programs *repository.ProgramRepo
	db       *pgxpool.Pool
}

func NewAIService(apiKey string, programs *repository.ProgramRepo, db *pgxpool.Pool) *AIService {
	// senza opts il client legge ANTHROPIC_API_KEY dall'ambiente
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return &AIService{client: anthropic.NewClient(opts...), programs: programs, db: db}
}

type GeneratePlanInput struct {
	Goal   string `json:"goal"`
	Level  string `json:"level"`
	Days   int    `json:"days"`
	Length int    `json:"length"`
	Notes  string `json:"notes"`
}

type aiPlanOutput struct {
	Name        string      `json:"name"`
	Goal        string      `json:"goal"`
	Level       string      `json:"level"`
	TotalWeeks  int         `json:"totalWeeks"`
	DaysPerWeek int         `json:"daysPerWeek"`
	Workouts    []aiWorkout `json:"workouts"`
}

type aiWorkout struct {
	Name      string       `json:"name"`
	Focus     string       `json:"focus"`
	DayOfWeek int          `json:"dayOfWeek"`
	Exercises []aiExercise `json:"exercises"`
}

type aiExercise struct {
	Name        string `json:"name"`
	MuscleGroup string `json:"muscleGroup"`
	Sets        int    `json:"sets"`
	TargetReps  int    `json:"targetReps"`
	RestSeconds int    `json:"restSeconds"`
}

var planToolSchema = anthropic.ToolParam{
	Name:        "create_training_plan",
	Description: anthropic.String("Create a complete structured weekly training program"),
	InputSchema: anthropic.ToolInputSchemaParam{
		Type: "object",
		Properties: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string"},
			"goal":        map[string]interface{}{"type": "string"},
			"level":       map[string]interface{}{"type": "string"},
			"totalWeeks":  map[string]interface{}{"type": "integer"},
			"daysPerWeek": map[string]interface{}{"type": "integer"},
			"workouts": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":     "object",
					"required": []string{"name", "focus", "dayOfWeek", "exercises"},
					"properties": map[string]interface{}{
						"name":      map[string]interface{}{"type": "string"},
						"focus":     map[string]interface{}{"type": "string"},
						"dayOfWeek": map[string]interface{}{"type": "integer", "description": "0=Monday, 6=Sunday"},
						"exercises": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type":     "object",
								"required": []string{"name", "muscleGroup", "sets", "targetReps", "restSeconds"},
								"properties": map[string]interface{}{
									"name":        map[string]interface{}{"type": "string"},
									"muscleGroup": map[string]interface{}{"type": "string"},
									"sets":        map[string]interface{}{"type": "integer"},
									"targetReps":  map[string]interface{}{"type": "integer"},
									"restSeconds": map[string]interface{}{"type": "integer"},
								},
							},
						},
					},
				},
			},
		},
		Required: []string{"name", "goal", "level", "totalWeeks", "daysPerWeek", "workouts"},
	},
}

func (s *AIService) GeneratePlan(ctx context.Context, userID string, input GeneratePlanInput) (*model.UserProgram, error) {
	planData, err := s.requestPlan(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.savePlan(ctx, userID, planData)
}

// requestPlan è la sola parte che parla con Claude: separata da savePlan così
// si può testare senza database (vedi ai_test.go).
func (s *AIService) requestPlan(ctx context.Context, input GeneratePlanInput) (aiPlanOutput, error) {
	prompt := fmt.Sprintf(
		`You are an expert strength and conditioning coach.
Create a complete, evidence-based training program for:
- Goal: %s | Level: %s | Days/week: %d | Session length: %d min
%s
Generate a realistic, progressive program with appropriate exercise selection, volume, and rest periods.
Use standard barbell/dumbbell/cable/bodyweight exercises. DayOfWeek: 0=Monday, 1=Tuesday, ... 6=Sunday.
Pick %d consecutive weekdays starting Monday.`,
		input.Goal, input.Level, input.Days, input.Length,
		notesLine(input.Notes),
		input.Days,
	)

	// In streaming: su Opus 5 il thinking è attivo di default e condivide
	// MaxTokens con la risposta, quindi il budget è alto — e sopra ~16k token
	// una richiesta non-streaming rischia di sbattere nei timeout HTTP dell'SDK
	// prima ancora di avere una risposta.
	stream := s.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:      "claude-opus-5",
		MaxTokens:  32000,
		Tools:      []anthropic.ToolUnionParam{{OfTool: &planToolSchema}},
		ToolChoice: anthropic.ToolChoiceParamOfTool("create_training_plan"),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})

	// Accumulate ricompone i delta in un Message completo, come se fosse
	// arrivato in un colpo solo: l'input del tool_use arriva a pezzi
	// (input_json_delta) ed è JSON valido solo a blocco chiuso.
	var resp anthropic.Message
	var planData aiPlanOutput
	for stream.Next() {
		if err := resp.Accumulate(stream.Current()); err != nil {
			return planData, fmt.Errorf("claude API: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return planData, fmt.Errorf("claude API: %w", err)
	}

	// extract tool_use block
	for _, block := range resp.Content {
		if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
			if err := json.Unmarshal(tu.Input, &planData); err != nil {
				return planData, err
			}
			break
		}
	}
	if planData.Name == "" {
		return planData, fmt.Errorf("claude did not return a training plan")
	}
	return planData, nil
}

func (s *AIService) savePlan(ctx context.Context, userID string, plan aiPlanOutput) (*model.UserProgram, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// deactivate existing programs
	if _, err := tx.Exec(ctx,
		`UPDATE user_programs SET is_active = FALSE WHERE user_id = $1`, userID); err != nil {
		return nil, err
	}

	var programID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO user_programs (user_id, name, goal, level, days_per_week, total_weeks)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		userID, plan.Name, plan.Goal, plan.Level, plan.DaysPerWeek, plan.TotalWeeks,
	).Scan(&programID); err != nil {
		return nil, err
	}

	for i, w := range plan.Workouts {
		var workoutID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO program_workouts (program_id, name, focus, day_of_week, order_in_week)
			 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			programID, w.Name, w.Focus, w.DayOfWeek, i,
		).Scan(&workoutID); err != nil {
			return nil, err
		}

		for j, ex := range w.Exercises {
			exID, err := s.programs.FindOrCreateExercise(ctx, tx, ex.Name, ex.MuscleGroup)
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO program_workout_exercises
				 (workout_id, exercise_id, sets, target_reps, rest_seconds, order_index)
				 VALUES ($1,$2,$3,$4,$5,$6)`,
				workoutID, exID, ex.Sets, ex.TargetReps, ex.RestSeconds, j,
			); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.programs.GetActiveProgram(ctx, userID)
}

func notesLine(notes string) string {
	if notes == "" {
		return ""
	}
	return "Additional context: " + notes
}
