// Package notify is an in-process pub/sub used by the API to wake log-stream
// subscribers when the worker writes a new log line. It exists so SSE handlers
// can block on a channel instead of polling SQLite every few hundred ms.
//
// Cross-process wakes are delivered by the worker posting to an HTTP endpoint
// on the API, which then calls Notify here. We go through HTTP (not the
// queue backend) because redka only exposes list operations — no pub/sub,
// no blocking ops — so a dedicated in-memory broker is the simplest thing
// that gives us "subscribe once, wake many times".
package notify

import "sync"

type Broker struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func New() *Broker {
	return &Broker{subs: make(map[string]map[chan struct{}]struct{})}
}

// Subscribe returns a buffered (depth 1) wake channel plus an unsubscribe
// func. The channel only signals "you should refresh" — it never carries
// payload, so a coalesced signal is no worse than a fresh one.
func (b *Broker) Subscribe(id string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	if _, ok := b.subs[id]; !ok {
		b.subs[id] = make(map[chan struct{}]struct{})
	}
	b.subs[id][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs[id], ch)
		if len(b.subs[id]) == 0 {
			delete(b.subs, id)
		}
		b.mu.Unlock()
	}
}

// Notify wakes every current subscriber for the given id. A subscriber whose
// buffer is already full is skipped — the pending wake will drain the DB
// anyway, so coalescing two writes into one wake is fine.
func (b *Broker) Notify(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[id] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
