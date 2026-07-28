// Package ratelimit implementa una finestra scorrevole in memoria, pensata per
// gli endpoint che costano soldi o mandano email a indirizzi altrui.
//
// Lo stato vive nel processo: riparte da zero a ogni riavvio e non è condiviso
// fra più istanze. Va benissimo per fermare un abuso banale; se un giorno il
// backend gira replicato servirà spostarlo su Postgres o Redis.
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	hits      map[string][]time.Time
	lastSweep time.Time
}

// New — al massimo `max` tentativi per chiave in `window`.
func New(max int, window time.Duration) *Limiter {
	return &Limiter{
		max:       max,
		window:    window,
		hits:      make(map[string][]time.Time),
		lastSweep: time.Now(),
	}
}

// Allow registra un tentativo per `key` e dice se sta dentro il limite. Un
// tentativo rifiutato NON viene registrato: martellare l'endpoint non allunga
// da solo il tempo di attesa.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.sweepLocked(now)

	fresh := l.freshLocked(key, now)
	if len(fresh) >= l.max {
		l.hits[key] = fresh
		return false
	}
	l.hits[key] = append(fresh, now)
	return true
}

// RetryAfter dice quanto manca prima che si liberi uno slot per `key`.
// Zero se la chiave può già riprovare.
func (l *Limiter) RetryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	fresh := l.freshLocked(key, now)
	if len(fresh) < l.max {
		return 0
	}
	// Il tentativo più vecchio è il primo a uscire dalla finestra.
	wait := l.window - now.Sub(fresh[0])
	if wait < 0 {
		return 0
	}
	return wait
}

// freshLocked restituisce i tentativi ancora dentro la finestra, riusando
// l'array esistente. Va chiamata col lock preso.
func (l *Limiter) freshLocked(key string, now time.Time) []time.Time {
	times := l.hits[key]
	kept := times[:0]
	for _, t := range times {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	return kept
}

// sweepLocked elimina le chiavi scadute. Senza questo un attaccante che usa
// indirizzi sempre diversi farebbe crescere la mappa all'infinito — cioè il
// rate limiter diventerebbe lui stesso il problema. Passa al massimo una volta
// per finestra, così il costo resta ammortizzato.
func (l *Limiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key, times := range l.hits {
		alive := false
		for _, t := range times {
			if now.Sub(t) < l.window {
				alive = true
				break
			}
		}
		if !alive {
			delete(l.hits, key)
		}
	}
}
