package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// recorder registra le chiamate di una PruneTask.
type recorder struct {
	mu      sync.Mutex
	cutoffs []time.Time
	err     error
}

func (r *recorder) task(name string) PruneTask {
	return PruneTask{Name: name, Delete: func(_ context.Context, cutoff time.Time) (int64, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.cutoffs = append(r.cutoffs, cutoff)
		return 1, r.err
	}}
}

func (r *recorder) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.cutoffs)
}

func TestOnceRunsEveryTask(t *testing.T) {
	a, b := &recorder{}, &recorder{}
	NewPruner(time.Hour, time.Hour, a.task("a"), b.task("b")).Once(context.Background())

	if a.calls() != 1 || b.calls() != 1 {
		t.Errorf("calls: a=%d b=%d, want 1 each", a.calls(), b.calls())
	}
}

// Il cutoff deve essere `retention` nel passato: se fosse time.Now() la
// potatura cancellerebbe anche le righe appena scadute, buttando via la
// finestra di debug che è tutto il punto della retention.
func TestCutoffIsRetentionInThePast(t *testing.T) {
	const retention = 24 * time.Hour
	r := &recorder{}
	before := time.Now()
	NewPruner(time.Hour, retention, r.task("t")).Once(context.Background())
	after := time.Now()

	// Once ha letto l'ora fra `before` e `after`, quindi il cutoff sta in
	// [before-retention, after-retention].
	got := r.cutoffs[0]
	if got.Before(before.Add(-retention)) || got.After(after.Add(-retention)) {
		t.Errorf("cutoff = %v, want within [%v, %v]",
			got, before.Add(-retention), after.Add(-retention))
	}
}

// Una tabella che fallisce non deve impedire alle altre di essere ripulite.
func TestOneFailingTaskDoesNotStopTheOthers(t *testing.T) {
	failing := &recorder{err: errors.New("connection refused")}
	healthy := &recorder{}

	NewPruner(time.Hour, time.Hour, failing.task("broken"), healthy.task("fine")).
		Once(context.Background())

	if healthy.calls() != 1 {
		t.Error("the healthy task was skipped after the previous one failed")
	}
}

func TestRunPrunesImmediatelyThenOnTheInterval(t *testing.T) {
	r := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go NewPruner(30*time.Millisecond, time.Hour, r.task("t")).Run(ctx)

	// Il primo giro è immediato: non si aspetta un intervallo intero prima di
	// pulire, altrimenti un processo riavviato spesso non poterebbe mai.
	waitFor(t, func() bool { return r.calls() >= 1 }, "the immediate first prune")
	waitFor(t, func() bool { return r.calls() >= 3 }, "the ticker to fire twice more")
}

func TestRunStopsOnContextCancel(t *testing.T) {
	r := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		NewPruner(10*time.Millisecond, time.Hour, r.task("t")).Run(ctx)
		close(done)
	}()

	waitFor(t, func() bool { return r.calls() >= 1 }, "the first prune")
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
