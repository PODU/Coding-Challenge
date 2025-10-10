package document

import "testing"

// DidOpen stores text+version; didChange replaces+bumps; didClose removes.
func TestStoreLifecycle(t *testing.T) {
	s := NewStore()
	s.Open("file:///a.greet", "hello", 1)

	d, ok := s.Get("file:///a.greet")
	if !ok || d.Text != "hello" || d.Version != 1 {
		t.Fatalf("after open: %+v ok=%v", d, ok)
	}

	s.Update("file:///a.greet", "world", 2)
	d, _ = s.Get("file:///a.greet")
	if d.Text != "world" || d.Version != 2 {
		t.Fatalf("after update: %+v", d)
	}

	s.Close("file:///a.greet")
	if _, ok := s.Get("file:///a.greet"); ok {
		t.Fatal("document should be gone after close")
	}
	if s.Len() != 0 {
		t.Fatalf("len = %d, want 0", s.Len())
	}
}

// DidChange on an unknown URI creates the document defensively.
func TestUpdateCreatesDefensively(t *testing.T) {
	s := NewStore()
	s.Update("file:///never-opened.greet", "text", 5)

	d, ok := s.Get("file:///never-opened.greet")
	if !ok || d.Text != "text" || d.Version != 5 {
		t.Fatalf("defensive create failed: %+v ok=%v", d, ok)
	}
}
