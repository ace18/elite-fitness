package model

import (
	"testing"
	"time"
)

func TestWeekAt(t *testing.T) {
	start := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC) // lunedì
	day := 24 * time.Hour

	tests := []struct {
		name  string
		after time.Duration
		want  int
	}{
		{"start of the program", 0, 1},
		{"same day", 6 * time.Hour, 1},
		{"day 6 is still week 1", 6 * day, 1},
		{"day 7 starts week 2", 7 * day, 2},
		{"day 13 is still week 2", 13 * day, 2},
		{"day 14 starts week 3", 14 * day, 3},
		{"last week of a 12-week program", 77 * day, 12},
		// Oltre la fine si resta sull'ultima settimana: "settimana 15 di 12"
		// romperebbe le barre di avanzamento e non vuol dire niente.
		{"past the end clamps to the last week", 200 * day, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &UserProgram{StartedAt: start, TotalWeeks: 12}
			if got := p.WeekAt(start.Add(tt.after)); got != tt.want {
				t.Errorf("WeekAt(+%v) = %d, want %d", tt.after, got, tt.want)
			}
		})
	}
}

// started_at nel futuro non è un caso reale, ma una settimana 0 o negativa
// manderebbe in tilt "settimana X di Y" e le barre di avanzamento.
func TestWeekAtNeverGoesBelowOne(t *testing.T) {
	start := time.Now().Add(48 * time.Hour)
	p := &UserProgram{StartedAt: start, TotalWeeks: 8}
	if got := p.WeekAt(time.Now()); got != 1 {
		t.Errorf("WeekAt with a future start = %d, want 1", got)
	}
}

func TestWeekAtHandlesMissingTotalWeeks(t *testing.T) {
	start := time.Now().Add(-100 * 24 * time.Hour)
	for _, total := range []int{0, -3} {
		p := &UserProgram{StartedAt: start, TotalWeeks: total}
		if got := p.WeekAt(time.Now()); got != 1 {
			t.Errorf("TotalWeeks=%d: WeekAt = %d, want 1", total, got)
		}
	}
}

func TestIsComplete(t *testing.T) {
	start := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	// 8 settimane, 3 allenamenti a settimana.
	prog := func() *UserProgram {
		return &UserProgram{StartedAt: start, TotalWeeks: 8, DaysPerWeek: 3}
	}

	tests := []struct {
		name     string
		after    time.Duration
		sessions int
		want     bool
	}{
		{"first week, no sessions", 0, 0, false},
		// Il grosso del rischio: chiudere il programma perché si è allenato
		// tanto, ma a settimana 2 su 8.
		{"lots of sessions but not the final week", 7 * day, 99, false},
		{"final week, one session short", 49 * day, 2, false},
		{"final week, last session done", 49 * day, 3, true},
		{"final week, extra sessions", 49 * day, 5, true},
		// Chi finisce in ritardo ha comunque finito.
		{"long past the end, quota met", 300 * day, 3, true},
		{"long past the end, quota not met", 300 * day, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prog().IsComplete(start.Add(tt.after), tt.sessions); got != tt.want {
				t.Errorf("IsComplete(+%v, %d sessions) = %v, want %v",
					tt.after, tt.sessions, got, tt.want)
			}
		})
	}
}

func TestFinalWeekStart(t *testing.T) {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	p := &UserProgram{StartedAt: start, TotalWeeks: 8}

	want := start.Add(7 * 7 * 24 * time.Hour) // 8ª settimana = dopo 7 settimane
	if got := p.FinalWeekStart(); !got.Equal(want) {
		t.Errorf("FinalWeekStart = %v, want %v", got, want)
	}
	// La settimana calcolata a quell'istante dev'essere davvero l'ultima.
	if w := p.WeekAt(p.FinalWeekStart()); w != 8 {
		t.Errorf("WeekAt(FinalWeekStart) = %d, want 8", w)
	}

	single := &UserProgram{StartedAt: start, TotalWeeks: 1}
	if got := single.FinalWeekStart(); !got.Equal(start) {
		t.Errorf("one-week program: FinalWeekStart = %v, want the start date", got)
	}
}

// DaysPerWeek a zero renderebbe ogni programma completo alla prima sessione.
func TestIsCompleteGuardsAgainstZeroDaysPerWeek(t *testing.T) {
	p := &UserProgram{StartedAt: time.Now().Add(-100 * 24 * time.Hour), TotalWeeks: 4, DaysPerWeek: 0}
	if p.IsComplete(time.Now(), 0) {
		t.Error("complete with zero sessions logged")
	}
	if !p.IsComplete(time.Now(), 1) {
		t.Error("should be complete after one session when DaysPerWeek is unset")
	}
}

// La settimana deve avanzare da sola col passare del tempo: è esattamente ciò
// che non succedeva quando era una colonna che nessuno aggiornava.
func TestWeekAdvancesOverTime(t *testing.T) {
	start := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	p := &UserProgram{StartedAt: start, TotalWeeks: 12}

	prev := 0
	for w := 0; w < 12; w++ {
		got := p.WeekAt(start.Add(time.Duration(w) * 7 * 24 * time.Hour))
		if got != w+1 {
			t.Fatalf("after %d weeks: got %d, want %d", w, got, w+1)
		}
		if got <= prev {
			t.Fatalf("week did not advance: %d then %d", prev, got)
		}
		prev = got
	}
}
