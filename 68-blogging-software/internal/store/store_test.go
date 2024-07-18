package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blog/internal/blog"
)

// upper is a trivial RenderFunc used to confirm HTMLCached is populated.
func upper(s string) string { return "<p>" + s + "</p>" }

// storeFactory builds a fresh store for a subtest.
type storeFactory struct {
	name string
	open func(t *testing.T) Store
}

func factories() []storeFactory {
	return []storeFactory{
		{"json", func(t *testing.T) Store {
			s, err := OpenJSON(t.TempDir(), upper)
			if err != nil {
				t.Fatalf("OpenJSON: %v", err)
			}
			return s
		}},
		{"sqlite", func(t *testing.T) Store {
			s, err := OpenSQLite(filepath.Join(t.TempDir(), "t.db"), upper)
			if err != nil {
				t.Fatalf("OpenSQLite: %v", err)
			}
			return s
		}},
	}
}

func tp(s string) *time.Time { t, _ := time.Parse(time.RFC3339, s); u := t.UTC(); return &u }

func TestStoreCreateGetUpdate(t *testing.T) {
	for _, f := range factories() {
		t.Run(f.name, func(t *testing.T) {
			s := f.open(t)
			defer s.Close()
			ctx := context.Background()

			p := &blog.Post{
				Title:       "Hello World",
				Summary:     "First",
				MarkdownRaw: "# Hi",
				Status:      blog.Published,
				PublishedAt: tp("2026-06-27T10:00:00Z"),
				CreatedAt:   *tp("2026-06-27T09:00:00Z"),
				UpdatedAt:   *tp("2026-06-27T10:00:00Z"),
			}
			id, err := s.Create(ctx, p)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if id == 0 {
				t.Fatal("expected non-zero id")
			}
			if p.Slug != "hello-world" {
				t.Errorf("slug = %q, want hello-world", p.Slug)
			}
			if p.HTMLCached != "<p># Hi</p>" {
				t.Errorf("HTMLCached = %q, want rendered", p.HTMLCached)
			}

			got, err := s.Get(ctx, id)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Title != "Hello World" || got.HTMLCached != "<p># Hi</p>" {
				t.Errorf("round-trip mismatch: %+v", got)
			}
			if got.PublishedAt == nil || !got.PublishedAt.Equal(*p.PublishedAt) {
				t.Errorf("PublishedAt mismatch: %v vs %v", got.PublishedAt, p.PublishedAt)
			}

			// Permalink retrieval by slug.
			bySlug, err := s.GetBySlug(ctx, "hello-world")
			if err != nil || bySlug.ID != id {
				t.Errorf("GetBySlug = %+v, %v", bySlug, err)
			}

			// Update keeps slug stable.
			got.Title = "Hello World Edited"
			got.MarkdownRaw = "# Edited"
			if err := s.Update(ctx, got); err != nil {
				t.Fatalf("Update: %v", err)
			}
			reloaded, _ := s.Get(ctx, id)
			if reloaded.Title != "Hello World Edited" {
				t.Errorf("title not updated: %q", reloaded.Title)
			}
			if reloaded.Slug != "hello-world" {
				t.Errorf("slug changed after update: %q", reloaded.Slug)
			}
			if reloaded.HTMLCached != "<p># Edited</p>" {
				t.Errorf("HTMLCached not regenerated: %q", reloaded.HTMLCached)
			}
		})
	}
}

func TestStoreGetNotFound(t *testing.T) {
	for _, f := range factories() {
		t.Run(f.name, func(t *testing.T) {
			s := f.open(t)
			defer s.Close()
			if _, err := s.Get(context.Background(), 999); err != ErrNotFound {
				t.Errorf("Get missing: err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestStoreUniqueSlug(t *testing.T) {
	for _, f := range factories() {
		t.Run(f.name, func(t *testing.T) {
			s := f.open(t)
			defer s.Close()
			ctx := context.Background()
			mk := func() *blog.Post {
				return &blog.Post{Title: "Same Title", Status: blog.Draft,
					CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			}
			p1 := mk()
			s.Create(ctx, p1)
			p2 := mk()
			s.Create(ctx, p2)
			p3 := mk()
			s.Create(ctx, p3)
			if p1.Slug != "same-title" || p2.Slug != "same-title-2" || p3.Slug != "same-title-3" {
				t.Errorf("slugs = %q, %q, %q", p1.Slug, p2.Slug, p3.Slug)
			}
		})
	}
}

func TestStoreListOrderingAndFilter(t *testing.T) {
	for _, f := range factories() {
		t.Run(f.name, func(t *testing.T) {
			s := f.open(t)
			defer s.Close()
			ctx := context.Background()

			seed := []struct {
				title string
				pub   string
				stat  blog.Status
			}{
				{"Oldest", "2026-01-01T00:00:00Z", blog.Published},
				{"Middle", "2026-03-01T00:00:00Z", blog.Published},
				{"Newest", "2026-06-01T00:00:00Z", blog.Published},
				{"Draft", "2026-05-01T00:00:00Z", blog.Draft},
				{"Future", "2026-12-01T00:00:00Z", blog.Published},
			}
			for _, sd := range seed {
				p := &blog.Post{Title: sd.title, Status: sd.stat, PublishedAt: tp(sd.pub),
					CreatedAt: *tp(sd.pub), UpdatedAt: *tp(sd.pub)}
				if _, err := s.Create(ctx, p); err != nil {
					t.Fatal(err)
				}
			}

			now := tp("2026-06-27T00:00:00Z")
			pub, err := s.List(ctx, ListOpts{Status: blog.Published, PublishedBefore: now, Order: NewestFirst})
			if err != nil {
				t.Fatal(err)
			}
			var titles []string
			for _, p := range pub {
				titles = append(titles, p.Title)
			}
			want := []string{"Newest", "Middle", "Oldest"}
			if len(titles) != len(want) {
				t.Fatalf("public list = %v, want %v", titles, want)
			}
			for i := range want {
				if titles[i] != want[i] {
					t.Errorf("order[%d] = %q, want %q (full %v)", i, titles[i], want[i], titles)
				}
			}

			// All posts (editor view) includes draft + future.
			all, _ := s.List(ctx, ListOpts{Order: NewestFirst})
			if len(all) != 5 {
				t.Errorf("all list len = %d, want 5", len(all))
			}

			// Limit.
			limited, _ := s.List(ctx, ListOpts{Status: blog.Published, PublishedBefore: now, Order: NewestFirst, Limit: 2})
			if len(limited) != 2 || limited[0].Title != "Newest" {
				t.Errorf("limited = %v", limited)
			}
		})
	}
}

func TestStoreComments(t *testing.T) {
	for _, f := range factories() {
		t.Run(f.name, func(t *testing.T) {
			s := f.open(t)
			defer s.Close()
			ctx := context.Background()

			p := &blog.Post{Title: "P", Status: blog.Published, PublishedAt: tp("2026-06-01T00:00:00Z"),
				CreatedAt: *tp("2026-06-01T00:00:00Z"), UpdatedAt: *tp("2026-06-01T00:00:00Z")}
			id, _ := s.Create(ctx, p)

			c1 := &blog.Comment{PostID: id, Name: "A", Email: "a@x.com", Body: "first",
				CreatedAt: *tp("2026-06-02T00:00:00Z")}
			c2 := &blog.Comment{PostID: id, Name: "B", Email: "b@x.com", Body: "second",
				CreatedAt: *tp("2026-06-03T00:00:00Z")}
			if err := s.AddComment(ctx, c1); err != nil {
				t.Fatal(err)
			}
			if err := s.AddComment(ctx, c2); err != nil {
				t.Fatal(err)
			}

			cs, err := s.ListComments(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if len(cs) != 2 {
				t.Fatalf("want 2 comments, got %d", len(cs))
			}
			// Newest-first.
			if cs[0].Body != "second" || cs[1].Body != "first" {
				t.Errorf("comment order = %q, %q; want second, first", cs[0].Body, cs[1].Body)
			}

			// Comment on missing post fails.
			if err := s.AddComment(ctx, &blog.Comment{PostID: 9999, Name: "x", Email: "x@x.com", Body: "y",
				CreatedAt: time.Now()}); err != ErrNotFound {
				t.Errorf("AddComment missing post: err = %v, want ErrNotFound", err)
			}
		})
	}
}

// TestJSONAtomicWrite verifies no partial/temp files linger after writes.
func TestJSONAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenJSON(dir, upper)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := &blog.Post{Title: "Atomic", Status: blog.Draft, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := s.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "posts"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			t.Errorf("unexpected non-json file left behind: %s", e.Name())
		}
	}
}

// TestJSONReload confirms the in-memory index rebuilds from disk.
func TestJSONReload(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s1, _ := OpenJSON(dir, upper)
	p := &blog.Post{Title: "Persisted", Status: blog.Published, PublishedAt: tp("2026-06-01T00:00:00Z"),
		CreatedAt: *tp("2026-06-01T00:00:00Z"), UpdatedAt: *tp("2026-06-01T00:00:00Z")}
	id, _ := s1.Create(ctx, p)
	s1.AddComment(ctx, &blog.Comment{PostID: id, Name: "A", Email: "a@x.com", Body: "hi", CreatedAt: time.Now().UTC()})
	s1.Close()

	s2, err := OpenJSON(dir, upper)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, err := s2.Get(ctx, id)
	if err != nil || got.Title != "Persisted" {
		t.Fatalf("reload Get = %+v, %v", got, err)
	}
	cs, _ := s2.ListComments(ctx, id)
	if len(cs) != 1 {
		t.Errorf("reload comments = %d, want 1", len(cs))
	}
}

func TestMigrateJSONToSQLite(t *testing.T) {
	ctx := context.Background()
	src, _ := OpenJSON(t.TempDir(), upper)
	defer src.Close()

	// Seed posts + comments.
	for i := 0; i < 3; i++ {
		p := &blog.Post{
			Title:       []string{"One", "Two", "Three"}[i],
			Status:      blog.Published,
			PublishedAt: tp([]string{"2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", "2026-03-01T00:00:00Z"}[i]),
			CreatedAt:   *tp("2026-01-01T00:00:00Z"),
			UpdatedAt:   *tp("2026-01-01T00:00:00Z"),
			MarkdownRaw: "body",
		}
		id, _ := src.Create(ctx, p)
		src.AddComment(ctx, &blog.Comment{PostID: id, Name: "N", Email: "n@x.com", Body: "c",
			CreatedAt: *tp("2026-04-01T00:00:00Z")})
	}

	dst, _ := OpenSQLite(filepath.Join(t.TempDir(), "m.db"), upper)
	defer dst.Close()

	posts, comments, err := Migrate(ctx, dst, src)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if posts != 3 || comments != 3 {
		t.Errorf("migrated %d posts, %d comments; want 3, 3", posts, comments)
	}

	// Identical ids/fields/order.
	srcAll, _ := src.List(ctx, ListOpts{Order: NewestFirst})
	dstAll, _ := dst.List(ctx, ListOpts{Order: NewestFirst})
	if len(srcAll) != len(dstAll) {
		t.Fatalf("len mismatch %d vs %d", len(srcAll), len(dstAll))
	}
	for i := range srcAll {
		a, b := srcAll[i], dstAll[i]
		if a.ID != b.ID || a.Title != b.Title || a.Slug != b.Slug {
			t.Errorf("post %d mismatch: %+v vs %+v", i, a, b)
		}
		if !a.PublishedAt.Equal(*b.PublishedAt) {
			t.Errorf("post %d published_at mismatch: %v vs %v", i, a.PublishedAt, b.PublishedAt)
		}
		ac, _ := src.ListComments(ctx, a.ID)
		bc, _ := dst.ListComments(ctx, b.ID)
		if len(ac) != len(bc) {
			t.Errorf("post %d comment count mismatch %d vs %d", a.ID, len(ac), len(bc))
			continue
		}
		for j := range ac {
			if ac[j].ID != bc[j].ID || ac[j].Body != bc[j].Body {
				t.Errorf("comment mismatch: %+v vs %+v", ac[j], bc[j])
			}
		}
	}
}
