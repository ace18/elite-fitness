package model

import "time"

// Exercise — una voce della libreria esercizi.
//
// I due contatori arrivano dall'elenco del pannello e servono a una cosa sola:
// sapere se si può cancellare. Sia template_workout_exercises che set_logs
// referenziano exercises senza ON DELETE, quindi una riga in uso non si cancella
// — e vale la pena dirlo prima che qualcuno prema il pulsante, invece di
// mostrare un errore del database dopo.
type Exercise struct {
	ID          string
	Name        string
	MuscleGroup string
	Category    string
	// InTemplates — quante righe di template lo usano.
	InTemplates int
	// InHistory — quante serie registrate lo nominano. Questo non si sistema
	// togliendolo dalle template: è storia degli atleti, e non si tocca.
	InHistory int
}

// Deletable — nessuno lo usa, quindi si può togliere dalla libreria.
func (e *Exercise) Deletable() bool { return e.InTemplates == 0 && e.InHistory == 0 }

// IsIsolation dice se il carico si muove a scatti piccoli (vedi
// service.loadIncrement): è l'unica conseguenza pratica della categoria.
func (e *Exercise) IsIsolation() bool { return e.Category == "isolation" }

// TemplateDay — un giorno di una template, dentro un blocco di periodizzazione.
//
// "Giorno" e "blocco" sono la stessa riga vista da due lati: week_number dice in
// quale blocco vive, day_of_week in quale giorno della settimana. Un giorno
// definito alla settimana 5 resta in vigore fino al blocco successivo (vedi
// GetWorkoutsForWeek), quindi non serve ripetere le settimane intermedie.
type TemplateDay struct {
	ID          string
	TemplateID  string
	Name        string
	Focus       string
	DayOfWeek   int
	WeekNumber  int
	OrderInWeek int
	Exercises   []TemplateExercise
}

// TemplateExercise — una riga di esercizio dentro un giorno di template.
type TemplateExercise struct {
	ID           string
	ExerciseID   string
	ExerciseName string
	MuscleGroup  string
	Sets         int
	TargetReps   int
	RestSeconds  int
	OrderIndex   int
	// LoadPct/LoadOffsetKg — il carico prescritto, come frazione del massimale
	// più una quota fissa in chili. Nil su quasi tutte le righe: solo i cicli a
	// carico imposto li usano, e per tutte le altre il peso lo propone lo
	// storico dell'atleta.
	LoadPct      *float64
	LoadOffsetKg *float64
}

// Prescribed dice se questa riga impone il carico invece di lasciarlo decidere
// all'autoregolazione.
func (t *TemplateExercise) Prescribed() bool { return t.LoadPct != nil }

// TemplateBlock raggruppa i giorni di una stessa settimana, per disegnarli.
type TemplateBlock struct {
	WeekNumber int
	Days       []TemplateDay
}

// TemplateSummary — una riga dell'elenco template del pannello.
type TemplateSummary struct {
	PlanTemplate
	ArchivedAt *time.Time
	// Days/Blocks — quanto è definita: una template senza giorni si assegna
	// senza errori e lascia l'atleta con un programma vuoto.
	Days   int
	Blocks int
	// InUse — quanti programmi sono nati da questa template. È ciò che decide
	// se si può cancellare o solo archiviare.
	InUse int
}

func (t *TemplateSummary) Archived() bool { return t.ArchivedAt != nil }

// Deletable — nessun programma è mai nato da questa template, quindi la chiave
// esterna di user_programs non ha niente da proteggere.
func (t *TemplateSummary) Deletable() bool { return t.InUse == 0 }

// Complete dice se la template ha almeno un giorno definito. Assegnarne una
// vuota è legale per il database e inutile per l'atleta.
func (t *TemplateSummary) Complete() bool { return t.Days > 0 }
