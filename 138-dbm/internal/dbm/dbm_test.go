package dbm

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ccdbm/internal/hashfn"
)

func tmpPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.db")
}

func create(t *testing.T, opts Options) *DB {
	t.Helper()
	opts.NoSync = true
	db, err := Create(tmpPath(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func reopen(t *testing.T, db *DB, opts Options) *DB {
	t.Helper()
	path := db.Path()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	opts.NoSync = true
	nd, err := Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nd.Close() })
	return nd
}

func key(i int) []byte { return []byte(fmt.Sprintf("key-%06d", i)) }
func val(i int) []byte { return []byte(fmt.Sprintf("value number %d", i)) }

func mustGet(t *testing.T, db *DB, k, want []byte) {
	t.Helper()
	got, err := db.Get(k)
	if err != nil {
		t.Fatalf("Get(%q): %v", k, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Get(%q) = %q, want %q", k, got, want)
	}
}

func mustMissing(t *testing.T, db *DB, k []byte) {
	t.Helper()
	if _, err := db.Get(k); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(%q) err = %v, want ErrNotFound", k, err)
	}
}

func TestCreateOpenPersist(t *testing.T) {
	db := create(t, Options{})
	if err := db.Put([]byte("greeting"), []byte("Hello, world!")); err != nil {
		t.Fatal(err)
	}
	if err := db.Put([]byte("language"), []byte("Go")); err != nil {
		t.Fatal(err)
	}
	mustGet(t, db, []byte("greeting"), []byte("Hello, world!"))
	if db.Len() != 2 {
		t.Fatalf("len = %d", db.Len())
	}

	db = reopen(t, db, Options{})
	mustGet(t, db, []byte("greeting"), []byte("Hello, world!"))
	mustGet(t, db, []byte("language"), []byte("Go"))
	if db.Len() != 2 || db.PageSize() != DefaultPageSize || db.HashID() != hashfn.FNV1a {
		t.Fatalf("metadata lost: len %d page %d hash %v", db.Len(), db.PageSize(), db.HashID())
	}
	if err := db.Delete([]byte("language")); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete([]byte("language")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	db = reopen(t, db, Options{})
	mustMissing(t, db, []byte("language"))
	mustGet(t, db, []byte("greeting"), []byte("Hello, world!"))
	if db.Len() != 1 {
		t.Fatalf("len after reopen = %d", db.Len())
	}
}

func TestCreateRefusesExisting(t *testing.T) {
	db := create(t, Options{})
	if _, err := Create(db.Path(), Options{}); err == nil {
		t.Fatal("Create over an existing file succeeded")
	}
}

func TestOverwriteAndBinary(t *testing.T) {
	db := create(t, Options{})
	k := []byte{0, 0xff, 0, 1}
	db.Put(k, []byte{1})
	db.Put(k, []byte{2, 3})
	db.Put([]byte{}, []byte("empty key"))
	db.Put([]byte("empty value"), []byte{})
	mustGet(t, db, k, []byte{2, 3})
	mustGet(t, db, nil, []byte("empty key"))
	mustGet(t, db, []byte("empty value"), []byte{})
	if db.Len() != 3 {
		t.Fatalf("len = %d", db.Len())
	}
	db = reopen(t, db, Options{})
	mustGet(t, db, k, []byte{2, 3})
	mustGet(t, db, []byte("empty value"), []byte{})
}

func TestManyKeysSplitAndDouble(t *testing.T) {
	db := create(t, Options{PageSize: 512})
	const n = 5000
	for i := 0; i < n; i++ {
		if err := db.Put(key(i), val(i)); err != nil {
			t.Fatalf("put %d: %v", i, err)
		}
	}
	if db.GlobalDepth() < 6 {
		t.Fatalf("global depth only %d after %d keys in 512-byte pages", db.GlobalDepth(), n)
	}
	st, err := db.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.DirectoryPages < 2 {
		t.Fatalf("directory should span several pages, got %d", st.DirectoryPages)
	}
	if st.Keys != n || st.BucketPages < 100 {
		t.Fatalf("stats: %+v", st)
	}
	for i := 0; i < n; i++ {
		mustGet(t, db, key(i), val(i))
	}
	db = reopen(t, db, Options{})
	for i := 0; i < n; i++ {
		mustGet(t, db, key(i), val(i))
	}
	mustMissing(t, db, []byte("nope"))
	keys, err := db.Keys()
	if err != nil || len(keys) != n {
		t.Fatalf("Keys: %d, %v", len(keys), err)
	}
}

func TestLargeValuesUseOverflow(t *testing.T) {
	db := create(t, Options{PageSize: 256})
	big := bytes.Repeat([]byte("0123456789abcdef"), 1024) // 16 KiB in 256-byte pages
	if err := db.Put([]byte("big"), big); err != nil {
		t.Fatal(err)
	}
	if err := db.Put([]byte("small"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	huge := make([]byte, 300_000)
	rand.New(rand.NewSource(1)).Read(huge)
	if err := db.Put([]byte("huge"), huge); err != nil {
		t.Fatal(err)
	}
	st, _ := db.Stats()
	if st.OverflowPages < 64 || st.LongestChain < 64 {
		t.Fatalf("expected overflow chains, got %+v", st)
	}
	db = reopen(t, db, Options{})
	mustGet(t, db, []byte("big"), big)
	mustGet(t, db, []byte("small"), []byte("x"))
	mustGet(t, db, []byte("huge"), huge)
	// shrinking a value releases its overflow pages to the free list
	if err := db.Put([]byte("huge"), []byte("tiny now")); err != nil {
		t.Fatal(err)
	}
	st2, _ := db.Stats()
	if st2.FreePages < 1000 || st2.Pages != st.Pages {
		t.Fatalf("overflow pages not freed: before %+v after %+v", st, st2)
	}
	mustGet(t, db, []byte("huge"), []byte("tiny now"))
}

func TestFreePagesAreReused(t *testing.T) {
	db := create(t, Options{PageSize: 256})
	v := bytes.Repeat([]byte("v"), 2000)
	for round := 0; round < 5; round++ {
		for i := 0; i < 20; i++ {
			if err := db.Put(key(i), v); err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < 20; i++ {
			if err := db.Delete(key(i)); err != nil {
				t.Fatal(err)
			}
		}
	}
	st, _ := db.Stats()
	first := st.Pages
	for i := 0; i < 20; i++ {
		db.Put(key(i), v)
	}
	st, _ = db.Stats()
	if st.Pages != first {
		t.Fatalf("file grew from %d to %d pages although free pages existed", first, st.Pages)
	}
	if db.Len() != 20 {
		t.Fatalf("len = %d", db.Len())
	}
}

func TestIteration(t *testing.T) {
	db := create(t, Options{PageSize: 256})
	const n = 300
	for i := 0; i < n; i++ {
		db.Put(key(i), val(i))
	}
	seen := map[string]int{}
	k, err := db.FirstKey()
	if err != nil {
		t.Fatal(err)
	}
	steps := 0
	for k != nil {
		seen[string(k)]++
		if k, err = db.NextKey(k); err != nil {
			t.Fatal(err)
		}
		if steps++; steps > n+1 {
			t.Fatal("iteration does not terminate")
		}
	}
	if len(seen) != n {
		t.Fatalf("first/next visited %d keys, want %d", len(seen), n)
	}
	for k, c := range seen {
		if c != 1 {
			t.Fatalf("%s visited %d times", k, c)
		}
	}
	// Each agrees with first/next
	var order [][]byte
	db.Each(func(k, v []byte, _ uint64) bool {
		order = append(order, append([]byte{}, k...))
		return true
	})
	if len(order) != n {
		t.Fatalf("Each visited %d", len(order))
	}
	// Each stops early when asked
	c := 0
	db.Each(func(_, _ []byte, _ uint64) bool { c++; return c < 5 })
	if c != 5 {
		t.Fatalf("Each did not stop: %d", c)
	}
}

func TestIterationAfterDeletes(t *testing.T) {
	db := create(t, Options{PageSize: 256})
	for i := 0; i < 100; i++ {
		db.Put(key(i), val(i))
	}
	for i := 0; i < 100; i += 2 {
		db.Delete(key(i))
	}
	seen := map[string]bool{}
	for k, _ := db.FirstKey(); k != nil; k, _ = db.NextKey(k) {
		if seen[string(k)] {
			t.Fatalf("%s visited twice", k)
		}
		seen[string(k)] = true
	}
	if len(seen) != 50 {
		t.Fatalf("saw %d keys after deletes, want 50", len(seen))
	}
	for i := 0; i < 100; i++ {
		if seen[string(key(i))] != (i%2 == 1) {
			t.Fatalf("key %d presence wrong", i)
		}
	}
	// delete-while-iterating: delete the current key, then ask for the next
	for k, _ := db.FirstKey(); k != nil; k, _ = db.NextKey(k) {
		if err := db.Delete(k); err != nil {
			t.Fatal(err)
		}
	}
	if db.Len() != 0 {
		t.Fatalf("%d keys survived delete-while-iterating", db.Len())
	}
	if k, _ := db.FirstKey(); k != nil {
		t.Fatalf("FirstKey on empty db = %q", k)
	}
}

func TestReorganiseShrinks(t *testing.T) {
	path := tmpPath(t)
	db, err := Create(path, Options{PageSize: 512, NoSync: true})
	if err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte("z"), 3000)
	for i := 0; i < 400; i++ {
		db.Put(key(i), big)
	}
	for i := 0; i < 400; i++ {
		if i%10 != 0 {
			db.Delete(key(i))
		}
	}
	before, _ := os.Stat(path)
	db.Close()

	res, err := Reorganise(path)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if after.Size() >= before.Size()/4 {
		t.Fatalf("reorganise did not shrink: %d -> %d", before.Size(), after.Size())
	}
	if res.Keys != 40 || res.BytesBefore != before.Size() || res.BytesAfter != after.Size() {
		t.Fatalf("result %+v", res)
	}
	if _, err := os.Stat(path + ".reorg"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
	db, err = Open(path, Options{NoSync: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.Len() != 40 || db.PageSize() != 512 {
		t.Fatalf("len %d page %d", db.Len(), db.PageSize())
	}
	for i := 0; i < 400; i++ {
		if i%10 == 0 {
			mustGet(t, db, key(i), big)
		} else {
			mustMissing(t, db, key(i))
		}
	}
	st, _ := db.Stats()
	if st.FreePages != 0 {
		t.Fatalf("reorganised file has %d free pages", st.FreePages)
	}
}

func TestReadOnly(t *testing.T) {
	db := create(t, Options{})
	db.Put([]byte("a"), []byte("1"))
	db = reopen(t, db, Options{ReadOnly: true})
	mustGet(t, db, []byte("a"), []byte("1"))
	if err := db.Put([]byte("b"), []byte("2")); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("put on read-only: %v", err)
	}
	if err := db.Delete([]byte("a")); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("delete on read-only: %v", err)
	}
	if _, err := db.Stats(); err != nil {
		t.Fatal(err)
	}
}

func TestClosed(t *testing.T) {
	db := create(t, Options{})
	db.Close()
	if _, err := db.Get([]byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatalf("get after close: %v", err)
	}
	if err := db.Put([]byte("x"), nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("put after close: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestOpenErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(filepath.Join(dir, "missing.db"), Options{}); err == nil {
		t.Fatal("opened a missing file")
	}
	junk := filepath.Join(dir, "junk.db")
	os.WriteFile(junk, []byte("this is not a database file at all, not even close......"), 0o644)
	if _, err := Open(junk, Options{}); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("junk file: %v", err)
	}
	short := filepath.Join(dir, "short.db")
	os.WriteFile(short, []byte("CC"), 0o644)
	if _, err := Open(short, Options{}); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("short file: %v", err)
	}
	// truncated: header claims more pages than exist
	db, _ := Create(filepath.Join(dir, "trunc.db"), Options{NoSync: true})
	db.Put([]byte("k"), []byte("v"))
	db.Close()
	os.Truncate(filepath.Join(dir, "trunc.db"), DefaultPageSize)
	if _, err := Open(filepath.Join(dir, "trunc.db"), Options{}); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("truncated file: %v", err)
	}
	if _, err := Create(filepath.Join(dir, "badpage.db"), Options{PageSize: 1000}); err == nil {
		t.Fatal("accepted non-power-of-two page size")
	}
	if _, err := Create(filepath.Join(dir, "badhash.db"), Options{Hash: 77}); err == nil {
		t.Fatal("accepted unknown hash")
	}
}

func TestHashOptionPersists(t *testing.T) {
	for _, name := range hashfn.Names() {
		id, _ := hashfn.Parse(name)
		db := create(t, Options{Hash: id, PageSize: 256})
		for i := 0; i < 500; i++ {
			db.Put(key(i), val(i))
		}
		db = reopen(t, db, Options{})
		if db.HashID() != id {
			t.Fatalf("%s: reopened with %v", name, db.HashID())
		}
		for i := 0; i < 500; i++ {
			mustGet(t, db, key(i), val(i))
		}
		st, _ := db.Stats()
		if st.Hash != name {
			t.Fatalf("stats hash = %s", st.Hash)
		}
	}
}

func TestTTL(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	opts := Options{Now: func() time.Time { return now }}
	db := create(t, opts)
	db.PutTTL([]byte("session"), []byte("abc"), 30*time.Second)
	db.Put([]byte("forever"), []byte("x"))
	mustGet(t, db, []byte("session"), []byte("abc"))
	_, exp, _ := db.GetExpires([]byte("session"))
	if exp != 1_000_030 {
		t.Fatalf("expires = %d", exp)
	}
	now = now.Add(29 * time.Second)
	mustGet(t, db, []byte("session"), []byte("abc"))
	now = now.Add(2 * time.Second)
	mustMissing(t, db, []byte("session"))
	keys, _ := db.Keys()
	if len(keys) != 1 || string(keys[0]) != "forever" {
		t.Fatalf("expired key still listed: %q", keys)
	}
	if k, _ := db.FirstKey(); string(k) != "forever" {
		t.Fatalf("FirstKey = %q", k)
	}
	// expired entries still occupy space until purged
	if db.Len() != 2 {
		t.Fatalf("len = %d", db.Len())
	}
	if err := db.Delete([]byte("session")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete expired: %v", err)
	}
	if db.Len() != 1 {
		t.Fatalf("len after purge = %d", db.Len())
	}
	// re-putting an expired key revives it
	db.PutTTL([]byte("session"), []byte("new"), time.Minute)
	mustGet(t, db, []byte("session"), []byte("new"))
	if db.Len() != 2 {
		t.Fatalf("len = %d", db.Len())
	}
	// ttl <= 0 means no expiry
	db.PutTTL([]byte("session"), []byte("kept"), 0)
	_, exp, _ = db.GetExpires([]byte("session"))
	if exp != 0 {
		t.Fatalf("ttl 0 set expiry %d", exp)
	}
}

func TestReorganiseDropsExpired(t *testing.T) {
	path := tmpPath(t)
	db, _ := Create(path, Options{NoSync: true, Now: func() time.Time { return time.Unix(100, 0) }})
	db.PutTTL([]byte("old"), []byte("1"), time.Second) // expires at 101
	db.Put([]byte("keep"), []byte("2"))
	db.Close()
	res, err := Reorganise(path) // real clock: long past 101
	if err != nil {
		t.Fatal(err)
	}
	if res.Keys != 1 || res.Expired != 1 {
		t.Fatalf("result %+v", res)
	}
	db, _ = Open(path, Options{NoSync: true})
	defer db.Close()
	if db.Len() != 1 {
		t.Fatalf("len = %d", db.Len())
	}
	mustGet(t, db, []byte("keep"), []byte("2"))
}

func TestStatsShape(t *testing.T) {
	db := create(t, Options{PageSize: 256})
	st, _ := db.Stats()
	if st.Pages != 3 || st.DirectoryPages != 1 || st.BucketPages != 1 || st.Keys != 0 || st.GlobalDepth != 0 {
		t.Fatalf("empty db stats: %+v", st)
	}
	for i := 0; i < 200; i++ {
		db.Put(key(i), val(i))
	}
	st, _ = db.Stats()
	if st.Keys != 200 || st.BucketPages < 20 || st.Fill() <= 0 || st.Fill() > 1 || st.DirectorySlots != 1<<uint(st.GlobalDepth) {
		t.Fatalf("stats: %+v", st)
	}
	if st.MinLocalDepth > st.MaxLocalDepth || st.MaxLocalDepth > st.GlobalDepth {
		t.Fatalf("depth range wrong: %+v", st)
	}
	if st.Pages != 1+st.DirectoryPages+st.BucketPages+st.OverflowPages+st.FreePages {
		t.Fatalf("page accounting: %+v", st)
	}
	var buf bytes.Buffer
	st.Write(&buf)
	if !bytes.Contains(buf.Bytes(), []byte("keys:")) {
		t.Fatalf("Write output: %s", buf.String())
	}
}

func TestPathologicalHashStaysBounded(t *testing.T) {
	// A hash that only ever varies in its top bits never lets a split
	// separate anything; the store must fall back to overflow chains
	// rather than doubling the directory to the limit.
	db := create(t, Options{PageSize: 256})
	db.hash = func(b []byte) uint64 { return hashfn.FNV1a64(b) << 40 }
	for i := 0; i < 200; i++ {
		if err := db.Put(key(i), val(i)); err != nil {
			t.Fatal(err)
		}
	}
	if db.GlobalDepth() != 0 {
		t.Fatalf("directory grew to depth %d", db.GlobalDepth())
	}
	st, _ := db.Stats()
	if st.OverflowPages == 0 {
		t.Fatalf("expected an overflow chain: %+v", st)
	}
	for i := 0; i < 200; i++ {
		mustGet(t, db, key(i), val(i))
	}
}

// TestRandomOpsAgainstModel drives the store with random puts, deletes
// and overwrites of random-size values (some spanning pages) and checks
// it against a map after every reopen.
func TestRandomOpsAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	db := create(t, Options{PageSize: 256})
	model := map[string][]byte{}
	randKey := func() []byte { return []byte(fmt.Sprintf("k%d", rng.Intn(400))) }
	randVal := func() []byte {
		n := rng.Intn(20)
		if rng.Intn(10) == 0 {
			n = rng.Intn(3000)
		}
		v := make([]byte, n)
		rng.Read(v)
		return v
	}
	check := func() {
		t.Helper()
		if db.Len() != uint64(len(model)) {
			t.Fatalf("len %d, model %d", db.Len(), len(model))
		}
		for k, v := range model {
			mustGet(t, db, []byte(k), v)
		}
		keys, _ := db.Keys()
		if len(keys) != len(model) {
			t.Fatalf("Keys %d, model %d", len(keys), len(model))
		}
		for _, k := range keys {
			if _, ok := model[string(k)]; !ok {
				t.Fatalf("phantom key %q", k)
			}
		}
	}
	for step := 0; step < 4000; step++ {
		k := randKey()
		switch rng.Intn(3) {
		case 0, 1:
			v := randVal()
			if err := db.Put(k, v); err != nil {
				t.Fatalf("step %d put: %v", step, err)
			}
			model[string(k)] = v
		case 2:
			err := db.Delete(k)
			_, had := model[string(k)]
			if had && err != nil {
				t.Fatalf("step %d delete: %v", step, err)
			}
			if !had && !errors.Is(err, ErrNotFound) {
				t.Fatalf("step %d delete missing: %v", step, err)
			}
			delete(model, string(k))
		}
		if step%1000 == 999 {
			check()
			db = reopen(t, db, Options{})
			check()
		}
	}
	check()
	path := db.Path()
	db.Close()
	if _, err := Reorganise(path); err != nil {
		t.Fatal(err)
	}
	nd, err := Open(path, Options{NoSync: true})
	if err != nil {
		t.Fatal(err)
	}
	defer nd.Close()
	db = nd
	check()
}

func TestSyncMode(t *testing.T) {
	// the default (syncing) path must work too; just fewer operations
	db, err := Create(tmpPath(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 5; i++ {
		if err := db.Put(key(i), val(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Sync(); err != nil {
		t.Fatal(err)
	}
	mustGet(t, db, key(3), val(3))
}

func BenchmarkPut(b *testing.B) {
	db, _ := Create(filepath.Join(b.TempDir(), "bench.db"), Options{NoSync: true})
	defer db.Close()
	v := bytes.Repeat([]byte("v"), 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Put(key(i), v)
	}
}

func BenchmarkGet(b *testing.B) {
	db, _ := Create(filepath.Join(b.TempDir(), "bench.db"), Options{NoSync: true})
	defer db.Close()
	v := bytes.Repeat([]byte("v"), 100)
	const n = 10000
	for i := 0; i < n; i++ {
		db.Put(key(i), v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Get(key(i % n))
	}
}
