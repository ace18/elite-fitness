package service

import (
	"context"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
)

// ProgramCompleter archivia i programmi arrivati in fondo.
//
// Il controllo gira dopo aver salvato una sessione, che è l'unico momento in
// cui un programma può diventare completo: è l'ultima sessione dell'ultima
// settimana a chiuderlo.
type ProgramCompleter struct {
	programs *repository.ProgramRepo
	sessions *repository.SessionRepo
}

func NewProgramCompleter(programs *repository.ProgramRepo, sessions *repository.SessionRepo) *ProgramCompleter {
	return &ProgramCompleter{programs: programs, sessions: sessions}
}

// ActiveProgram restituisce il programma attivo dell'utente, o nil se non ce
// n'è. Serve anche a registrare la sessione sul programma giusto senza fidarsi
// del programId mandato dal client.
func (c *ProgramCompleter) ActiveProgram(ctx context.Context, userID string) *model.UserProgram {
	program, err := c.programs.GetActiveProgram(ctx, userID)
	if err != nil {
		return nil
	}
	return program
}

// CompleteIfFinished archivia il programma se la sessione appena registrata
// era l'ultima dell'ultima settimana. Dice se ha archiviato.
func (c *ProgramCompleter) CompleteIfFinished(ctx context.Context, program *model.UserProgram) (bool, error) {
	if program == nil {
		return false, nil
	}
	done, err := c.sessions.CountSessionsForProgramSince(ctx, program.ID, program.FinalWeekStart())
	if err != nil {
		return false, err
	}
	if !program.IsComplete(time.Now(), done) {
		return false, nil
	}
	if err := c.programs.Archive(ctx, program.ID); err != nil {
		return false, err
	}
	return true, nil
}
