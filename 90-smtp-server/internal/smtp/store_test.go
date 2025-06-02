package smtp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMaildirStoreSave(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMaildirStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	env := Envelope{
		From:     "s@ex.com",
		To:       []string{"a@ex.com", "b@ex.com"},
		Data:     []byte("Subject: hi\r\n\r\nhello\r\n"),
		Received: time.Now(),
	}
	id, err := store.Save(env)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, id+".eml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "X-CC-From: <s@ex.com>") {
		t.Errorf("missing From trace header:\n%s", content)
	}
	if !strings.Contains(content, "X-CC-To: <a@ex.com>, <b@ex.com>") {
		t.Errorf("missing To trace header:\n%s", content)
	}
	if !strings.Contains(content, "hello") {
		t.Errorf("missing body:\n%s", content)
	}
}

func TestMaildirStoreConcurrent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMaildirStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Save(Envelope{
				From:     "s@ex.com",
				To:       []string{"r@ex.com"},
				Data:     []byte("body\r\n"),
				Received: time.Now(),
			})
			if err != nil {
				t.Errorf("save: %v", err)
			}
		}()
	}
	wg.Wait()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".eml") {
			count++
		}
	}
	if count != n {
		t.Errorf("got %d files, want %d (collision or loss)", count, n)
	}
}

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	if store.Count() != 0 {
		t.Fatal("new store not empty")
	}
	_, err := store.Save(Envelope{From: "a@b", To: []string{"c@d"}, Received: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if store.Count() != 1 {
		t.Fatalf("count = %d, want 1", store.Count())
	}
}
