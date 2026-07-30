package model

import "time"

type SessionLog struct {
	ID     string `json:"id"`
	UserID string `json:"userId,omitempty"`
	// ClientSessionID — UUID generato dal client alla fine dell'allenamento e
	// riusato a ogni tentativo di invio. È ciò che rende il salvataggio
	// idempotente (migrazione 008). Nil per i client che non lo mandano.
	ClientSessionID *string `json:"clientSessionId,omitempty"`
	WorkoutID       *string `json:"workoutId,omitempty"`
	ProgramID       *string `json:"programId,omitempty"`
	Name        string    `json:"name"`
	DurationMin *int      `json:"durationMin,omitempty"`
	TotalVolume *float64  `json:"totalVolume,omitempty"`
	TotalSets   *int      `json:"totalSets,omitempty"`
	AvgRPE      *float64  `json:"avgRpe,omitempty"`
	SessionRPE  *int      `json:"sessionRpe,omitempty"`
	CompletedAt time.Time `json:"completedAt"`
	Sets        []SetLog  `json:"sets,omitempty"`
}

// Tolleranza sull'orologio del client: un telefono un po' avanti non deve
// vedersi riscrivere il timestamp di un allenamento appena finito.
const completedAtSkewTolerance = 5 * time.Minute

// Quanto indietro si accetta un allenamento. La bozza locale scade dopo 24h e
// una coda offline aggiunge al più qualche giorno, quindi è largo: serve solo a
// limitare i danni di un client con la data sbagliata di anni.
const completedAtMaxBacklog = 30 * 24 * time.Hour

// ClampCompletedAt normalizza il momento di fine allenamento dichiarato dal
// client.
//
// Serve perché con la sincronizzazione differita `completed_at` non può più
// essere NOW(): un allenamento fatto lunedì e spedito mercoledì andava
// registrato mercoledì, sfasando la settimana del programma
// (UserProgram.WeekAt) e il conteggio dell'ultima settimana.
//
// Il futuro è la direzione pericolosa: CountSessionsForProgramSince conta le
// sessioni con completed_at >= FinalWeekStart, quindi una data avanti nel tempo
// gonfierebbe quel conteggio e archivierebbe il programma prima del dovuto.
// Il passato invece è legittimo — è esattamente il caso d'uso — e si tocca solo
// se assurdo.
func ClampCompletedAt(t, now time.Time) time.Time {
	if t.IsZero() {
		return now
	}
	if t.After(now.Add(completedAtSkewTolerance)) {
		return now
	}
	if t.Before(now.Add(-completedAtMaxBacklog)) {
		return now.Add(-completedAtMaxBacklog)
	}
	return t
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
