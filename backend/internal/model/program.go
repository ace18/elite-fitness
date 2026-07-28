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

// WeekAt calcola in che settimana del programma si è a una certa data.
//
// È derivata da started_at invece che tenuta in una colonna: un contatore
// memorizzato va aggiornato da qualcuno, e finché nessuno lo faceva il
// programma restava per sempre alla settimana 1. Derivandola non c'è niente da
// mantenere e non può sfasarsi.
//
// Il conteggio è di calendario: un programma di 12 settimane dura 12 settimane
// anche se ci si allena poco. Chi salta due settimane le vede comunque passare.
func (p *UserProgram) WeekAt(now time.Time) int {
	total := p.TotalWeeks
	if total < 1 {
		total = 1
	}
	week := int(now.Sub(p.StartedAt)/(7*24*time.Hour)) + 1
	if week < 1 {
		// started_at nel futuro: non è un caso reale, ma "settimana 0" o
		// negativa manderebbe in tilt le barre di avanzamento.
		return 1
	}
	if week > total {
		// Programma finito. Si resta sull'ultima settimana: chiudere davvero un
		// programma (archiviarlo, proporne un altro) è un'altra funzione.
		return total
	}
	return week
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
