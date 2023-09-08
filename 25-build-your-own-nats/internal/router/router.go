// Package router tracks subscriptions by subject and delivers published
// messages to them. Delivery goes through the Subscriber interface, so this
// package doesn't depend on the server.
//
// Subjects match exactly; the `*` and `>` wildcards are not supported.
package router

import (
	"sync"
	"sync/atomic"
)

// Subscriber receives messages for a subscription; in practice a client
// connection.
type Subscriber interface {
	// Deliver sends one message. It is called from many publishers at once.
	Deliver(subject, sid, reply string, payload []byte)
}

// Subscription is one SUB. Sid is unique only within its connection.
type Subscription struct {
	Subject string
	Queue   string
	Sid     string
	Sub     Subscriber

	// max is the limit from `UNSUB sid max_msgs`, 0 for none. Publishers
	// update these concurrently, hence atomics.
	max       int64
	delivered atomic.Int64
}

// Registry is the subscription index. It is safe for concurrent use.
type Registry struct {
	mu   sync.RWMutex
	subs map[string]map[*Subscription]struct{} // keyed by subject
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{subs: make(map[string]map[*Subscription]struct{})}
}

// Subscribe adds sub.
func (r *Registry) Subscribe(sub *Subscription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := r.subs[sub.Subject]
	if set == nil {
		set = make(map[*Subscription]struct{})
		r.subs[sub.Subject] = set
	}
	set[sub] = struct{}{}
}

// Unsubscribe removes sub. Removing it twice is fine.
func (r *Registry) Unsubscribe(sub *Subscription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remove(sub)
}

// SetAutoUnsub handles `UNSUB sid max_msgs`: sub is removed once it has
// delivered max messages, which may already be the case.
func (r *Registry) SetAutoUnsub(sub *Subscription, max int) {
	atomic.StoreInt64(&sub.max, int64(max))
	if int64(max) <= sub.delivered.Load() {
		r.Unsubscribe(sub)
	}
}

// remove deletes sub. The caller holds the write lock.
func (r *Registry) remove(sub *Subscription) {
	set := r.subs[sub.Subject]
	if set == nil {
		return
	}
	delete(set, sub)
	if len(set) == 0 {
		delete(r.subs, sub.Subject)
	}
}

// Publish delivers payload to every subscription on subject. Delivery happens
// after releasing the lock so a slow consumer can't hold up SUB/UNSUB on other
// connections.
func (r *Registry) Publish(subject, reply string, payload []byte) {
	r.mu.RLock()
	set := r.subs[subject]
	matched := make([]*Subscription, 0, len(set))
	for s := range set {
		matched = append(matched, s)
	}
	r.mu.RUnlock()

	for _, s := range matched {
		s.Sub.Deliver(subject, s.Sid, reply, payload)
		n := s.delivered.Add(1)
		if max := atomic.LoadInt64(&s.max); max > 0 && n >= max {
			r.Unsubscribe(s)
		}
	}
}

// Count returns the number of subscriptions on subject.
func (r *Registry) Count(subject string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.subs[subject])
}
