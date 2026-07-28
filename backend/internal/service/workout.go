package service

import (
	"context"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
)

type WorkoutService struct {
	programs *repository.ProgramRepo
}

func NewWorkoutService(programs *repository.ProgramRepo) *WorkoutService {
	return &WorkoutService{programs: programs}
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

	// find today's workout by day of week (0=Mon)
	todayDow := int(time.Now().Weekday()+6) % 7
	var target *model.ProgramWorkout
	for i := range workouts {
		if workouts[i].DayOfWeek == todayDow {
			target = &workouts[i]
			break
		}
	}
	// fallback to next in order
	if target == nil {
		target = &workouts[0]
	}

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
