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
	// MinOneRM/MaxOneRM delimitano i massimali per cui la template ha senso.
	// Nil su quasi tutte: solo i cicli a carico prescritto, dove gli
	// incrementi sono assoluti e quindi non si adattano fuori finestra.
	MinOneRM *float64 `json:"minOneRm,omitempty"`
	MaxOneRM *float64 `json:"maxOneRm,omitempty"`

	// Prescribes dice se almeno un esercizio della template calcola il carico
	// dal massimale.
	//
	// Non è la stessa cosa di avere una finestra di massimali, ed è la
	// distinzione che conta al momento di assegnare: la finestra dice "questa
	// template vale solo fra 145 e 170 kg", Prescribes dice "senza un massimale
	// questa template non sa che pesi mettere". Un ciclo di squat ha entrambe,
	// ma una template scritta a mano nel pannello può benissimo prescrivere una
	// percentuale senza dichiarare nessuna finestra — e in quel caso, chiedendo
	// il massimale solo in base alla finestra, l'atleta si ritroverebbe la
	// scheda senza carichi (CreateFromTemplate lascia NULL quando il massimale
	// è zero) senza che nessuno abbia visto un errore.
	Prescribes bool `json:"prescribesLoads,omitempty"`
}

type UserProgram struct {
	ID          string  `json:"id"`
	UserID      string  `json:"userId,omitempty"`
	TemplateID  *string `json:"templateId,omitempty"`
	Name        string  `json:"name"`
	Goal        string  `json:"goal"`
	Level       string  `json:"level"`
	DaysPerWeek int     `json:"daysPerWeek"`
	TotalWeeks  int     `json:"totalWeeks"`
	CurrentWeek int     `json:"week"`
	IsActive    bool    `json:"isActive"`
	// OneRMKg è il massimale su cui sono stati calcolati i carichi, per i
	// programmi che li prescrivono. Zero per tutti gli altri.
	OneRMKg float64 `json:"oneRmKg,omitempty"`

	// AssignedBy/AssignedByEmail — l'amministratore che ha assegnato il
	// programma dal pannello. Entrambi nil se l'atleta se l'è scelto da sé.
	//
	// Sono due perché servono a due cose: AssignedBy è il collegamento vivo alla
	// riga in `admins` e va a NULL se quella riga sparisce; AssignedByEmail è
	// l'indirizzo registrato al momento dell'assegnazione e resta comunque
	// (vedi migrazione 013).
	//
	// Fuori dal JSON: questa struct è anche la risposta di /api/program, e
	// l'indirizzo di chi amministra l'installazione non ha motivo di arrivare
	// sul telefono di ogni iscritto. Il pannello legge i campi direttamente.
	AssignedBy      *string `json:"-"`
	AssignedByEmail *string `json:"-"`

	StartedAt time.Time      `json:"startedAt"`
	Schedule  []ScheduleItem `json:"schedule,omitempty"`
}

// AssignedByCoach dice se il programma è stato messo lì da un amministratore
// invece che scelto dall'atleta.
//
// Guarda l'email e non l'id: l'id si azzera se l'amministratore viene cancellato
// dal database, e siccome NULL è anche il valore che significa "scelto
// dall'atleta", basarsi su quello farebbe cambiare risposta a una domanda sul
// passato — il programma risulterebbe scelto da chi non l'ha scelto.
func (p *UserProgram) AssignedByCoach() bool { return p.AssignedByEmail != nil }

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

// WeekStart è l'istante da cui parte la settimana `week` del programma
// (1 = la prima). Le settimane sono blocchi di sette giorni dall'inizio, non
// settimane di calendario: chi comincia di giovedì ha la sua settimana 1 che
// finisce il giovedì dopo.
func (p *UserProgram) WeekStart(week int) time.Time {
	if week < 1 {
		week = 1
	}
	return p.StartedAt.Add(time.Duration(week-1) * 7 * 24 * time.Hour)
}

// FinalWeekStart è l'istante da cui parte l'ultima settimana del programma.
func (p *UserProgram) FinalWeekStart() time.Time {
	total := p.TotalWeeks
	if total < 1 {
		total = 1
	}
	return p.WeekStart(total)
}

// IsComplete dice se il programma è arrivato in fondo: si è nell'ultima
// settimana e le sessioni previste per quella settimana sono state fatte tutte.
//
// `sessionsInFinalWeek` va contato da FinalWeekStart in poi, non dentro una
// finestra chiusa di sette giorni: chi finisce il programma con settimane di
// ritardo ha comunque completato l'ultima settimana, e un conteggio a finestra
// lo lascerebbe in un programma che non finisce mai.
func (p *UserProgram) IsComplete(now time.Time, sessionsInFinalWeek int) bool {
	if p.WeekAt(now) < p.TotalWeeks {
		return false
	}
	perWeek := p.DaysPerWeek
	if perWeek < 1 {
		// Un programma senza giorni previsti si considererebbe finito subito.
		perWeek = 1
	}
	return sessionsInFinalWeek >= perWeek
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
	ID         string `json:"id"`
	ExerciseID string `json:"exerciseId"`
	Name       string `json:"name"`
	Muscle     string `json:"muscle"`
	// Category è 'compound' o 'isolation'. Serve a sapere di quanto si può
	// alzare il carico: su un complementare il disco più piccolo pesa molto di
	// più in percentuale (vedi loadIncrement).
	Category    string `json:"category,omitempty"`
	Sets        int    `json:"sets"`
	TargetReps  int    `json:"targetReps"`
	RestSeconds int    `json:"rest"`
	OrderIndex  int    `json:"orderIndex"`
	// LoadKg è il carico prescritto dal programma. Zero quando non c'è: in quel
	// caso il peso lo propone suggestNext dallo storico, come sempre.
	LoadKg    float64 `json:"loadKg,omitempty"`
	Last      float64 `json:"last"`
	Suggested float64 `json:"suggested"`
	PrToBeat  float64 `json:"prToBeat,omitempty"`
}

type TodayWorkout struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Focus     string            `json:"focus"`
	EstMin    int               `json:"estMin"`
	Exercises []WorkoutExercise `json:"exercises"`
}
