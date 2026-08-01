package service

import (
	"context"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
)

type WorkoutService struct {
	programs *repository.ProgramRepo
	sessions *repository.SessionRepo
}

func NewWorkoutService(programs *repository.ProgramRepo, sessions *repository.SessionRepo) *WorkoutService {
	return &WorkoutService{programs: programs, sessions: sessions}
}

func (s *WorkoutService) BuildTodayWorkout(ctx context.Context, userID string) (*model.TodayWorkout, error) {
	program, err := s.programs.GetActiveProgram(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Gli allenamenti della settimana in corso: se un giorno le template
	// definiranno settimane diverse, l'allenamento di oggi seguirà quella
	// giusta senza altre modifiche qui.
	workouts, err := s.programs.GetWorkoutsForWeek(ctx, program.ID, program.CurrentWeek)
	if err != nil {
		return nil, err
	}
	if len(workouts) == 0 {
		return nil, nil
	}

	// L'allenamento da proporre è il prossimo NON ancora fatto in questa
	// settimana di programma, in ordine di scheda.
	//
	// Prima si sceglieva per giorno della settimana, con `workouts[0]` come
	// ripiego quando nessun giorno combaciava. Due conseguenze: di sabato — che
	// nel 5×5 è riposo — proponeva comunque il primo allenamento, e soprattutto
	// registrarne uno non cambiava niente, perché la scelta non guardava lo
	// storico. Chi aveva appena finito Strength A se lo ritrovava proposto di
	// nuovo.
	//
	// Contare le sessioni invece del giorno regge anche i ritardi: saltato il
	// lunedì, martedì si riparte comunque da dove si era rimasti, invece di
	// perdere l'allenamento perché il calendario è andato avanti.
	done, err := s.sessions.CountSessionsForProgramSince(
		ctx, program.ID, program.WeekStart(program.CurrentWeek))
	if err != nil {
		return nil, err
	}
	if done >= len(workouts) {
		// Quota della settimana esaurita: da qui in poi è riposo.
		return nil, nil
	}
	target := &workouts[done]

	exercises, err := s.programs.GetExercisesForWorkout(ctx, target.ID)
	if err != nil {
		return nil, err
	}

	for i := range exercises {
		ex := &exercises[i]
		last, rpe, pr, _ := s.programs.GetLastSetForExercise(ctx, userID, ex.ExerciseID)
		ex.Last = last
		ex.Suggested = suggestNext(last, rpe)
		if pr > 0 {
			ex.PrToBeat = pr
		}
	}

	estMin := 0
	for _, ex := range exercises {
		estMin += ex.Sets * (60 + ex.RestSeconds) / 60
	}

	return &model.TodayWorkout{
		ID:        target.ID,
		Name:      target.Name,
		Focus:     target.Focus,
		EstMin:    estMin,
		Exercises: exercises,
	}, nil
}

func suggestNext(weight float64, rpe *float64) float64 {
	if rpe == nil || weight == 0 {
		return weight
	}
	r := *rpe
	const step = 2.5
	if r <= 7 {
		return weight + step
	}
	if r >= 9.5 {
		return weight - step
	}
	return weight
}
