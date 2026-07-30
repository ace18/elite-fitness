package model

import (
	"testing"
	"time"
)

func TestClampCompletedAt(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   time.Time
		want time.Time
		why  string
	}{
		{
			name: "zero value falls back to now",
			in:   time.Time{},
			want: now,
			why:  "un client che non manda completedAt deve comportarsi come prima",
		},
		{
			name: "backdated session is preserved",
			in:   now.Add(-48 * time.Hour),
			want: now.Add(-48 * time.Hour),
			why:  "è il caso d'uso: allenamento di lunedì spedito mercoledì",
		},
		{
			name: "small clock skew is tolerated",
			in:   now.Add(2 * time.Minute),
			want: now.Add(2 * time.Minute),
			why:  "un telefono leggermente avanti non va corretto",
		},
		{
			name: "future beyond tolerance is clamped to now",
			in:   now.Add(72 * time.Hour),
			want: now,
			why:  "una data futura gonfierebbe il conteggio dell'ultima settimana",
		},
		{
			name: "absurd past is clamped to the backlog floor",
			in:   now.Add(-5 * 365 * 24 * time.Hour),
			want: now.Add(-completedAtMaxBacklog),
			why:  "client con la data sbagliata di anni",
		},
		{
			name: "exactly at now is preserved",
			in:   now,
			want: now,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClampCompletedAt(tc.in, now)
			if !got.Equal(tc.want) {
				t.Errorf("ClampCompletedAt(%v, %v) = %v, want %v — %s", tc.in, now, got, tc.want, tc.why)
			}
		})
	}
}

// Il punto della clamp è proteggere l'auto-archiviazione: una sessione con
// completed_at nel futuro non deve poter contare verso l'ultima settimana di un
// programma che non è ancora arrivato in fondo.
func TestClampCompletedAtProtectsFinalWeekCount(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	// Programma di 12 settimane iniziato una settimana fa: l'ultima settimana
	// comincia fra 11 settimane.
	p := &UserProgram{StartedAt: now.Add(-7 * 24 * time.Hour), TotalWeeks: 12, DaysPerWeek: 3}

	finalWeekStart := p.FinalWeekStart()
	if !finalWeekStart.After(now) {
		t.Fatalf("setup errato: l'ultima settimana dovrebbe essere nel futuro, è %v", finalWeekStart)
	}

	// Un client con l'orologio sbagliato dichiara di essersi allenato durante
	// l'ultima settimana, che non è ancora cominciata.
	cheeky := finalWeekStart.Add(24 * time.Hour)
	clamped := ClampCompletedAt(cheeky, now)

	if !clamped.Before(finalWeekStart) {
		t.Errorf("completed_at %v non è stato riportato prima dell'ultima settimana (%v): conterebbe verso l'archiviazione",
			clamped, finalWeekStart)
	}
}
