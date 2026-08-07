package service

import (
	"context"
	"math"

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

	// Gli allenamenti della settimana in corso. Dalla 010 le template sono
	// periodizzate a blocchi, quindi qui la settimana conta davvero:
	// GetWorkoutsForWeek risolve al blocco in vigore.
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
		h, err := s.programs.GetExerciseHistory(ctx, userID, ex.ExerciseID, ex.TargetReps)
		if err != nil {
			return nil, err
		}
		ex.Last = h.Last
		// Un carico prescritto dal programma non si autoregola: il ciclo di
		// squat decide lui quanto caricare a ogni seduta, e sovrascriverlo con
		// una proposta dallo storico ne romperebbe la progressione.
		if ex.LoadKg > 0 {
			ex.Suggested = ex.LoadKg
		} else {
			ex.Suggested = suggestNext(h, ex.TargetReps, ex.Category)
		}
		if h.PR > 0 {
			ex.PrToBeat = h.PR
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

const (
	// progressionRate è di quanto si sposta il carico quando l'RPE dice che si
	// può: una frazione del peso attuale, non un numero fisso.
	//
	// Prima era 2.5 kg per tutto e per tutti. Su uno stacco da 200 kg è
	// l'1.25%, cioè quasi niente; su un'alzata laterale da 20 kg è il 12.5%,
	// cioè un salto che quasi nessuno regge a parità di ripetizioni. Lo stesso
	// numero era troppo piccolo di là e troppo grande di qua.
	progressionRate = 0.025

	// maxStep tiene a freno i pesi assurdi: da un carico battuto a macchina
	// male (uno zero di troppo) uscirebbe un incremento a due cifre proposto
	// come se fosse normale.
	maxStep = 10.0
)

// loadIncrement è il salto più piccolo che si riesce davvero a fare su un
// esercizio, e quindi il taglio a cui va arrotondato tutto quello che si
// propone: un 87.3 sul bilanciere non si carica.
//
// Sui fondamentali è la coppia di dischi da 1.25 per lato. Sui complementari —
// manubri, cavi — si scende, ed è quello che permette a un'alzata laterale di
// muoversi di poco invece che a scatti del 12%.
func loadIncrement(category string) float64 {
	if category == "isolation" {
		return 1.25
	}
	return 2.5
}

// progressionStep è di quanto muovere il carico: una percentuale del peso
// attuale, portata al taglio caricabile più vicino e mai sotto un taglio solo —
// se no su pesi leggeri il passo si arrotonderebbe a zero e la proposta
// resterebbe ferma per sempre.
func progressionStep(weight, increment float64) float64 {
	stepped := math.Round(weight*progressionRate/increment) * increment
	if stepped < increment {
		return increment
	}
	if stepped > maxStep {
		return maxStep
	}
	return stepped
}

// suggestNext propone il carico per l'esercizio di oggi.
//
// Due strade, a seconda di cosa dice lo storico:
//
//   - Si è già lavorato a queste ripetizioni: si autoregola sull'ultima volta,
//     seguendo l'RPE. È il caso normale, dentro un blocco.
//   - Non ci si è mai lavorato: si stima il peso dal massimale recente. È la
//     prima seduta di un blocco nuovo, quando lo schema è appena cambiato e
//     l'ultimo peso registrato viene da ripetizioni diverse.
//
// Prima esisteva solo la prima strada, e prendeva l'ultimo peso qualunque fosse
// il numero di ripetizioni: passando da 5×5 a 5×3 proponeva il peso dei cinque
// per fare dei tripli, cioè parecchio meno di quanto l'atleta poteva.
func suggestNext(h repository.ExerciseHistory, targetReps int, category string) float64 {
	increment := loadIncrement(category)
	if h.Last > 0 {
		return autoregulate(h.Last, h.LastRPE, increment)
	}
	if h.Recent1RM > 0 {
		return roundDownTo(weightForReps(h.Recent1RM, targetReps), increment)
	}
	// Esercizio mai fatto: non si inventa un carico, lo sceglie l'atleta.
	return 0
}

// autoregulate aggiusta il carico in base a quanto è costata l'ultima volta.
// Senza RPE non si tocca niente: l'unica cosa peggiore di non proporre nulla è
// proporre un aumento che nessuno ha detto di reggere.
func autoregulate(weight float64, rpe *float64, increment float64) float64 {
	if rpe == nil {
		return weight
	}
	step := progressionStep(weight, increment)
	switch r := *rpe; {
	case r <= 7:
		return weight + step
	case r >= 9.5:
		// Mai sotto zero: con un peso più piccolo del passo si resta dov'è e
		// sarà l'atleta a scegliere, invece di proporgli un carico negativo.
		if weight-step <= 0 {
			return weight
		}
		return weight - step
	default:
		return weight
	}
}

// weightForReps inverte la formula di Epley (1RM = peso × (1 + rip/30)) per
// ricavare il peso che dovrebbe corrispondere a un certo numero di ripetizioni.
//
// È una stima, e a ripetizioni alte perde precisione, ma serve solo a dare un
// punto di partenza il primo giorno di un blocco: da lì in poi riprende
// l'autoregolazione sull'RPE, che si corregge da sola.
func weightForReps(oneRM float64, reps int) float64 {
	if reps < 1 {
		reps = 1
	}
	return oneRM / (1 + float64(reps)/30)
}

// roundDownTo arrotonda per difetto a un multiplo di `increment`. Per difetto e
// non al più vicino: è un peso stimato, e su una stima è meglio partire
// leggeri e aggiustare al rialzo la volta dopo.
func roundDownTo(weight, increment float64) float64 {
	if weight <= 0 {
		return 0
	}
	return math.Floor(weight/increment) * increment
}
