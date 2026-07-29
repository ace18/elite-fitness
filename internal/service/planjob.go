package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/elitecoach/backend/internal/model"
)

// La generazione di un piano con Claude può durare minuti. Tenerla dentro la
// richiesta HTTP non funziona dietro Cloudflare: il tunnel chiude la
// connessione all'origine dopo ~100s (errore 524) e l'utente vede un errore
// mentre il backend continua a girare e gli scrive comunque il programma —
// che gli comparirebbe addosso al giro dopo senza spiegazione.
//
// Quindi: POST avvia il lavoro e risponde subito con un id, il client fa
// polling. Il lavoro sopravvive alla disconnessione del client, e il polling
// resta ben sotto qualsiasi timeout del tunnel.
//
// Lo stato vive nel processo, come il rate limiter: si perde al riavvio e non
// è condiviso fra istanze. Va bene finché il backend gira su una replica sola
// (le migrazioni all'avvio impongono già quel vincolo); se un domani si
// replica, questo va spostato su Postgres.

type PlanJobStatus string

const (
	PlanJobPending PlanJobStatus = "pending"
	PlanJobDone    PlanJobStatus = "done"
	PlanJobFailed  PlanJobStatus = "failed"
)

const (
	// Quanto un job resta leggibile dopo essere finito. Deve bastare al client
	// per accorgersi dell'esito con il suo polling.
	planJobRetention = 30 * time.Minute
	// Tetto massimo alla generazione. Senza, una chiamata piantata terrebbe
	// occupata una goroutine per sempre.
	planJobTimeout = 10 * time.Minute
)

type PlanJob struct {
	ID       string
	UserID   string
	Status   PlanJobStatus
	Program  *model.UserProgram
	ErrCode  string
	Finished time.Time
}

type PlanJobStore struct {
	mu   sync.Mutex
	jobs map[string]*PlanJob
}

func NewPlanJobStore() *PlanJobStore {
	return &PlanJobStore{jobs: make(map[string]*PlanJob)}
}

// Start registra un job e lancia `run` in background. Il contesto passato a
// `run` NON è quello della richiesta HTTP: quello viene annullato appena
// l'handler risponde, che è esattamente ciò che vogliamo evitare.
func (s *PlanJobStore) Start(userID string, run func(ctx context.Context) (*model.UserProgram, error), onError func(error) string) *PlanJob {
	job := &PlanJob{ID: newJobID(), UserID: userID, Status: PlanJobPending}

	s.mu.Lock()
	s.sweepLocked(time.Now())
	s.jobs[job.ID] = job
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), planJobTimeout)
		defer cancel()

		program, err := run(ctx)

		s.mu.Lock()
		defer s.mu.Unlock()
		job.Finished = time.Now()
		if err != nil {
			job.Status = PlanJobFailed
			job.ErrCode = onError(err)
			return
		}
		job.Status = PlanJobDone
		job.Program = program
	}()

	return job
}

// Get restituisce il job solo al suo proprietario: l'id da solo non deve
// bastare a leggere il programma di un altro utente.
func (s *PlanJobStore) Get(id, userID string) (PlanJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok || job.UserID != userID {
		return PlanJob{}, false
	}
	// Copia: il chiamante legge fuori dal lock mentre la goroutine può ancora
	// scrivere sull'originale.
	return *job, true
}

// sweepLocked butta via i job finiti da un pezzo. Senza, la mappa cresce di una
// riga per ogni generazione e non la libera mai. Va chiamata col lock preso.
func (s *PlanJobStore) sweepLocked(now time.Time) {
	for id, job := range s.jobs {
		if job.Status == PlanJobPending {
			continue
		}
		if now.Sub(job.Finished) > planJobRetention {
			delete(s.jobs, id)
		}
	}
}

func newJobID() string {
	b := make([]byte, 16)
	// crypto/rand.Read non fallisce su nessuna piattaforma supportata; se
	// fallisse, un id prevedibile sarebbe comunque inutile senza il userID.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
