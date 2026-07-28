package model

import "time"

type PlanTemplate struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Goal        string  `json:"goal"`
	Focus       string  `json:"focus"`
	Level       string  `json:"level"`
	DaysPerWeek int     `json:"daysPerWeek"`
	SessionMin  int     `json:"sessionMin"`
	TotalWeeks  int     `json:"totalWeeks"`
	Glyph       string  `json:"glyph"`
	Tag         *string `json:"tag,omitempty"`
}

type UserProgram struct {
	ID          string         `json:"id"`
	UserID      string         `json:"userId,omitempty"`
	TemplateID  *string        `json:"templateId,omitempty"`
	Name        string         `json:"name"`
	Goal        string         `json:"goal"`
	Level       string         `json:"level"`
	DaysPerWeek int            `json:"daysPerWeek"`
	TotalWeeks  int            `json:"totalWeeks"`
	CurrentWeek int            `json:"week"`
	IsActive    bool           `json:"isActive"`
	StartedAt   time.Time      `json:"startedAt"`
	Schedule    []ScheduleItem `json:"schedule,omitempty"`
}

type ScheduleItem struct {
	Day    string  `json:"day"`
	Name   string  `json:"name"`
	Focus  string  `json:"focus"`
	Status string  `json:"status"`
	Volume *string `json:"volume,omitempty"`
}

type ProgramWorkout struct {
	ID          string            `json:"id"`
	ProgramID   string            `json:"programId"`
	Name        string            `json:"name"`
	Focus       string            `json:"focus"`
	DayOfWeek   int               `json:"dayOfWeek"`
	WeekNumber  int               `json:"weekNumber"`
	OrderInWeek int               `json:"orderInWeek"`
	Exercises   []WorkoutExercise `json:"exercises,omitempty"`
}

type WorkoutExercise struct {
	ID          string  `json:"id"`
	ExerciseID  string  `json:"exerciseId"`
	Name        string  `json:"name"`
	Muscle      string  `json:"muscle"`
	Sets        int     `json:"sets"`
	TargetReps  int     `json:"targetReps"`
	RestSeconds int     `json:"rest"`
	OrderIndex  int     `json:"orderIndex"`
	Last        float64 `json:"last"`
	Suggested   float64 `json:"suggested"`
	PrToBeat    float64 `json:"prToBeat,omitempty"`
}

type TodayWorkout struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Focus     string            `json:"focus"`
	EstMin    int               `json:"estMin"`
	Exercises []WorkoutExercise `json:"exercises"`
}
