package router

import (
	"sync"
	"testing"
)

// recordingSub collects deliveries; safe for concurrent use.
type recordingSub struct {
	mu  sync.Mutex
	got [][]byte
}

func (r *recordingSub) Deliver(subject, sid, reply string, payload []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]byte, len(payload))
	copy(cp, payload)
	r.got = append(r.got, cp)
}

func (r *recordingSub) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.got)
}

func TestPublishDeliversToMatchingSubject(t *testing.T) {
	reg := New()
	a := &recordingSub{}
	b := &recordingSub{}
	other := &recordingSub{}

	reg.Subscribe(&Subscription{Subject: "foo", Sid: "1", Sub: a})
	reg.Subscribe(&Subscription{Subject: "foo", Sid: "2", Sub: b})
	reg.Subscribe(&Subscription{Subject: "bar", Sid: "3", Sub: other})

	reg.Publish("foo", "", []byte("hello"))

	if a.count() != 1 || b.count() != 1 {
		t.Fatalf("both foo subscribers should get 1 msg, got a=%d b=%d", a.count(), b.count())
	}
	if other.count() != 0 {
		t.Fatalf("bar subscriber should get 0, got %d", other.count())
	}
}

func TestPublishNoSubscribersIsNoop(t *testing.T) {
	reg := New()
	reg.Publish("nobody", "", []byte("x")) // must not panic
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	reg := New()
	s := &recordingSub{}
	sub := &Subscription{Subject: "foo", Sid: "1", Sub: s}
	reg.Subscribe(sub)

	reg.Publish("foo", "", []byte("1"))
	reg.Unsubscribe(sub)
	reg.Publish("foo", "", []byte("2"))

	if s.count() != 1 {
		t.Fatalf("want 1 delivery before unsub, got %d", s.count())
	}
}

func TestAutoUnsubAfterMax(t *testing.T) {
	reg := New()
	s := &recordingSub{}
	sub := &Subscription{Subject: "foo", Sid: "1", Sub: s}
	reg.Subscribe(sub)
	reg.SetAutoUnsub(sub, 2)

	for i := 0; i < 5; i++ {
		reg.Publish("foo", "", []byte("m"))
	}
	if s.count() != 2 {
		t.Fatalf("auto-unsub after 2: want 2 deliveries, got %d", s.count())
	}
	if reg.Count("foo") != 0 {
		t.Fatalf("subscription should be removed, count=%d", reg.Count("foo"))
	}
}

func TestSetAutoUnsubZeroRemovesImmediately(t *testing.T) {
	reg := New()
	s := &recordingSub{}
	sub := &Subscription{Subject: "foo", Sid: "1", Sub: s}
	reg.Subscribe(sub)
	reg.SetAutoUnsub(sub, 0)
	reg.Publish("foo", "", []byte("m"))
	if s.count() != 0 {
		t.Fatalf("max 0 should remove immediately, got %d deliveries", s.count())
	}
}

func TestConcurrentPublish(t *testing.T) {
	reg := New()
	const subscribers = 8
	const messages = 200

	subs := make([]*recordingSub, subscribers)
	for i := range subs {
		subs[i] = &recordingSub{}
		reg.Subscribe(&Subscription{Subject: "fan", Sid: string(rune('a' + i)), Sub: subs[i]})
	}

	var wg sync.WaitGroup
	for p := 0; p < 4; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < messages/4; i++ {
				reg.Publish("fan", "", []byte("x"))
			}
		}()
	}
	wg.Wait()

	for i, s := range subs {
		if s.count() != messages {
			t.Fatalf("subscriber %d got %d, want %d", i, s.count(), messages)
		}
	}
}
