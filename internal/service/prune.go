package service

import (
	"context"
	"log"
	"time"
)

// Ogni quanto gira la potatura. I token vivono 15 minuti e gli state OAuth 10:
// un giro all'ora è più che sufficiente a tenere le tabelle piccole.
const DefaultPruneInterval = time.Hour

// Quanto si tengono le righe DOPO la scadenza. Non servono a funzionare — sono
// già inutilizzabili — ma sono l'unica traccia di un login andato storto, e
// gettarle subito significa non poter più rispondere a "il link è mai partito?".
const DefaultPruneRetention = 24 * time.Hour

// PruneTask è una singola tabella da ripulire. La firma è una funzione e non
// un repository così il Pruner si testa senza database.
type PruneTask struct {
	Name   string
	Delete func(ctx context.Context, cutoff time.Time) (int64, error)
}

// Pruner cancella periodicamente le righe scadute. Senza di lui
// magic_link_tokens e oauth_states crescono per sempre: ogni richiesta di
// login lascia una riga, e nessuno la toglie.
type Pruner struct {
	interval  time.Duration
	retention time.Duration
	tasks     []PruneTask
}

func NewPruner(interval, retention time.Duration, tasks ...PruneTask) *Pruner {
	return &Pruner{interval: interval, retention: retention, tasks: tasks}
}

// Run pota subito e poi a ogni intervallo, finché il contesto non viene
// annullato. Va lanciata in una goroutine.
func (p *Pruner) Run(ctx context.Context) {
	p.Once(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Once(ctx)
		}
	}
}

// Once esegue un giro solo. Un errore su una tabella non ferma le altre: la
// potatura è manutenzione, non deve mai diventare il motivo per cui il resto
// smette di funzionare.
func (p *Pruner) Once(ctx context.Context) {
	cutoff := time.Now().Add(-p.retention)
	for _, task := range p.tasks {
		removed, err := task.Delete(ctx, cutoff)
		if err != nil {
			log.Printf("[prune] %s: %v", task.Name, err)
			continue
		}
		if removed > 0 {
			log.Printf("[prune] %s: removed %d expired rows", task.Name, removed)
		}
	}
}
