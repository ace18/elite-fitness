package model

import "time"

type SessionLog struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId,omitempty"`
	WorkoutID   *string   `json:"workoutId,omitempty"`
	ProgramID   *string   `json:"programId,omitempty"`
	Name        string    `json:"name"`
	DurationMin *int      `json:"durationMin,omitempty"`
	TotalVolume *float64  `json:"totalVolume,omitempty"`
	TotalSets   *int      `json:"totalSets,omitempty"`
	AvgRPE      *float64  `json:"avgRpe,omitempty"`
	SessionRPE  *int      `json:"sessionRpe,omitempty"`
	CompletedAt time.Time `json:"completedAt"`
	Sets        []SetLog  `json:"sets,omitempty"`
}

type SetLog struct {
	ID           string   `json:"id,omitempty"`
	SessionID    string   `json:"sessionId,omitempty"`
	ExerciseID   string   `json:"exerciseId"`
	ExerciseName string   `json:"exerciseName"`
	SetNumber    int      `json:"setNumber"`
	Weight       float64  `json:"weight"`
	Reps         int      `json:"reps"`
	RPE          *float64 `json:"rpe,omitempty"`
	IsPR         bool     `json:"isPr"`
}

type PR struct {
	Lift  string `json:"lift"`
	Value string `json:"value"`
	When  string `json:"when"`
	Fresh bool   `json:"fresh"`
}

type BodyWeightEntry struct {
	Weight float64 `json:"value"`
	Unit   string  `json:"unit"`
}

type ProgressMetrics struct {
	Streak       int             `json:"streak"`
	WeekSessions int             `json:"weekSessions"`
	WeekGoal     int             `json:"weekGoal"`
	BodyWeight   BodyWeightStats `json:"bodyWeight"`
	Est1RM       Est1RMStats     `json:"est1RM"`
	Volume       VolumeStats     `json:"volume"`
	PRs          []PR            `json:"prs"`
}

type BodyWeightStats struct {
	Value  float64   `json:"value"`
	Unit   string    `json:"unit"`
	Delta  float64   `json:"delta"`
	Series []float64 `json:"series"`
}

type Est1RMStats struct {
	Lift   string    `json:"lift"`
	Value  float64   `json:"value"`
	Unit   string    `json:"unit"`
	Delta  float64   `json:"delta"`
	Series []float64 `json:"series"`
}

type VolumeStats struct {
	Value  float64   `json:"value"`
	Unit   string    `json:"unit"`
	Delta  float64   `json:"delta"`
	Series []float64 `json:"series"`
}
