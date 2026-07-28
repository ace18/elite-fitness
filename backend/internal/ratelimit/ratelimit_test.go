package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestAllowsUpToMaxThenBlocks(t *testing.T) {
	l := New(3, time.Minute)
	for i := 1; i <= 3; i++ {
		if !l.Allow("a@b.c") {
			t.Fatalf("attempt %d blocked, want allowed", i)
		}
	}
	if l.Allow("a@b.c") {
		t.Error("4th attempt allowed, want blocked")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, time.Minute)
	if !l.Allow("first") {
		t.Fatal("first key blocked")
	}
	if !l.Allow("second") {
		t.Error("second key blocked by the first key's usage")
	}
}

func TestWindowExpires(t *testing.T) {
	l := New(1, 40*time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("first attempt blocked")
	}
	if l.Allow("k") {
		t.Fatal("second attempt allowed inside the window")
	}
	time.Sleep(60 * time.Millisecond)
	if !l.Allow("k") {
		t.Error("still blocked after the window passed")
	}
}

// Un tentativo rifiutato non deve contare: altrimenti chi martella l'endpoint
// si allungherebbe l'attesa da solo, all'infinito.
func TestBlockedAttemptsDoNotExtendTheWindow(t *testing.T) {
	l := New(1, 60*time.Millisecond)
	l.Allow("k")
	for i := 0; i < 20; i++ {
		l.Allow("k")
	}
	time.Sleep(80 * time.Millisecond)
	if !l.Allow("k") {
		t.Error("key still blocked — rejected attempts extended the window")
	}
}

func TestRetryAfter(t *testing.T) {
	l := New(1, time.Minute)
	if d := l.RetryAfter("k"); d != 0 {
		t.Errorf("unused key: RetryAfter = %v, want 0", d)
	}
	l.Allow("k")
	d := l.RetryAfter("k")
	if d <= 0 || d > time.Minute {
		t.Errorf("RetryAfter = %v, want (0, 1m]", d)
	}
}

// La mappa non deve crescere senza limite quando l'attaccante cambia chiave a
// ogni richiesta: è il modo più stupido di far esplodere la memoria.
func TestExpiredKeysAreSwept(t *testing.T) {
	l := New(1, 20*time.Millisecond)
	for i := 0; i < 500; i++ {
		l.Allow(string(rune(i)) + "-key")
	}
	time.Sleep(40 * time.Millisecond)

	// La prima Allow dopo la finestra fa scattare lo sweep.
	l.Allow("trigger")

	l.mu.Lock()
	size := len(l.hits)
	l.mu.Unlock()
	if size > 1 {
		t.Errorf("map holds %d keys after the sweep, want 1 (only the trigger)", size)
	}
}

func TestConcurrentAllowIsRaceFree(t *testing.T) {
	l := New(100, time.Minute)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("shared") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 100 {
		t.Errorf("allowed %d of 200 concurrent attempts, want exactly 100", allowed)
	}
}
