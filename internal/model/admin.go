package model

import "time"

// Admin — chi può entrare nel pannello di gestione.
//
// Niente tag JSON: queste strutture non escono mai da un endpoint, le legge
// solo un template. È il vantaggio di un pannello reso dal server — non c'è un
// contratto pubblico da mantenere, e cambiare un campo non rompe nessun client.
type Admin struct {
	ID        string
	Email     string
	Name      string
	CreatedBy *string
	// CreatedByEmail è comodo da mostrare nell'elenco senza una seconda query.
	// Nil per il primo amministratore, che non è stato creato da nessuno, e per
	// quelli il cui creatore è stato cancellato (created_by va a NULL).
	CreatedByEmail *string
	CreatedAt      time.Time
	LastLoginAt    *time.Time
	DisabledAt     *time.Time
}

func (a *Admin) Active() bool { return a.DisabledAt == nil }

// Display è il nome da mostrare: l'email finché non c'è un nome vero. Gli
// amministratori si creano da un indirizzo soltanto, quindi all'inizio il nome
// è sempre vuoto.
func (a *Admin) Display() string {
	if a.Name != "" {
		return a.Name
	}
	return a.Email
}

// AthleteRow — una riga dell'elenco atleti.
//
// Tutto quello che serve alla tabella arriva da una query sola (vedi
// AdminRepo.ListAthletes): con una query per atleta la pagina rallenterebbe in
// proporzione agli iscritti, che è esattamente la direzione sbagliata.
type AthleteRow struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time

	// Program è nil per chi non ha ancora scelto un programma.
	Program *AthleteProgram

	LastSessionAt *time.Time
	TotalSessions int
	// Sessions7d — allenamenti negli ultimi sette giorni. È il numero che dice
	// se qualcuno si sta allenando adesso, mentre TotalSessions dice solo che a
	// un certo punto l'ha fatto.
	Sessions7d int
}

func (a *AthleteRow) Display() string {
	if a.Name != "" {
		return a.Name
	}
	return a.Email
}

// Stale dice se l'atleta non si allena da più di `days` giorni. Chi non ha mai
// registrato niente non è "in ritardo": è un altro stato, e la tabella lo
// mostra come tale.
func (a *AthleteRow) Stale(days int) bool {
	if a.LastSessionAt == nil {
		return false
	}
	return time.Since(*a.LastSessionAt) > time.Duration(days)*24*time.Hour
}

type AthleteProgram struct {
	ID          string
	Name        string
	Week        int
	TotalWeeks  int
	DaysPerWeek int
	StartedAt   time.Time
}
