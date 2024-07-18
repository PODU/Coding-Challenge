package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"blog/internal/blog"
)

// RenderFunc renders Markdown to HTML for Post.HTMLCached.
type RenderFunc func(markdown string) string

// JSONStore keeps each post in posts/{id}.json and its comments in
// comments/{postID}.json, with an in-memory index built on open.
type JSONStore struct {
	dir    string
	render RenderFunc

	mu            sync.RWMutex
	posts         map[int64]*blog.Post
	slugs         map[string]int64
	comments      map[int64][]*blog.Comment
	nextID        int64
	nextCommentID int64
}

// OpenJSON opens the store in dir, creating it if needed.
func OpenJSON(dir string, render RenderFunc) (*JSONStore, error) {
	if render == nil {
		render = func(s string) string { return s }
	}
	s := &JSONStore{
		dir:           dir,
		render:        render,
		posts:         make(map[int64]*blog.Post),
		slugs:         make(map[string]int64),
		comments:      make(map[int64][]*blog.Comment),
		nextID:        1,
		nextCommentID: 1,
	}
	for _, sub := range []string{"posts", "comments"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONStore) load() error {
	postsDir := filepath.Join(s.dir, "posts")
	entries, err := os.ReadDir(postsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(postsDir, e.Name()))
		if err != nil {
			return err
		}
		var p blog.Post
		if err := json.Unmarshal(data, &p); err != nil {
			return fmt.Errorf("decode %s: %w", e.Name(), err)
		}
		p.HTMLCached = s.render(p.MarkdownRaw)
		s.posts[p.ID] = &p
		if p.Slug != "" {
			s.slugs[p.Slug] = p.ID
		}
		if p.ID >= s.nextID {
			s.nextID = p.ID + 1
		}
		comments, err := s.loadComments(p.ID)
		if err != nil {
			return err
		}
		s.comments[p.ID] = comments
		for _, c := range comments {
			if c.ID >= s.nextCommentID {
				s.nextCommentID = c.ID + 1
			}
		}
	}
	return nil
}

func (s *JSONStore) loadComments(postID int64) ([]*blog.Comment, error) {
	path := filepath.Join(s.dir, "comments", fmt.Sprintf("%d.json", postID))
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cs []*blog.Comment
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("decode comments %d: %w", postID, err)
	}
	return cs, nil
}

// uniqueSlug returns the first of base, base-2, base-3, ... not used by a post
// other than ignoreID.
func (s *JSONStore) uniqueSlug(base string, ignoreID int64) string {
	if base == "" {
		return ""
	}
	candidate := base
	for i := 2; ; i++ {
		owner, taken := s.slugs[candidate]
		if !taken || owner == ignoreID {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

// Create assigns the post an id and a unique slug and saves it.
func (s *JSONStore) Create(ctx context.Context, p *blog.Post) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p.ID = s.nextID
	if p.Slug == "" {
		p.Slug = blog.Slugify(p.Title)
	}
	if p.Slug == "" {
		p.Slug = fmt.Sprintf("post-%d", p.ID)
	}
	p.Slug = s.uniqueSlug(p.Slug, p.ID)
	p.HTMLCached = s.render(p.MarkdownRaw)

	if err := s.writePost(p); err != nil {
		return 0, err
	}
	stored := clonePost(p)
	s.posts[p.ID] = stored
	s.slugs[p.Slug] = p.ID
	s.nextID++
	return p.ID, nil
}

// Update saves changes to an existing post.
func (s *JSONStore) Update(ctx context.Context, p *blog.Post) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.posts[p.ID]
	if !ok {
		return ErrNotFound
	}
	// Slugs never change, so links keep working.
	p.Slug = existing.Slug
	p.HTMLCached = s.render(p.MarkdownRaw)

	if err := s.writePost(p); err != nil {
		return err
	}
	s.posts[p.ID] = clonePost(p)
	return nil
}

func (s *JSONStore) Get(ctx context.Context, id int64) (*blog.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.posts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clonePost(p), nil
}

func (s *JSONStore) GetBySlug(ctx context.Context, slug string) (*blog.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.slugs[slug]
	if !ok {
		return nil, ErrNotFound
	}
	return clonePost(s.posts[id]), nil
}

func (s *JSONStore) List(ctx context.Context, opts ListOpts) ([]*blog.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*blog.Post
	for _, p := range s.posts {
		if matches(p, opts) {
			out = append(out, clonePost(p))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := effectiveDate(out[i]), effectiveDate(out[j])
		if di.Equal(dj) {
			// Same date: order by id so the result is stable.
			if opts.Order == OldestFirst {
				return out[i].ID < out[j].ID
			}
			return out[i].ID > out[j].ID
		}
		if opts.Order == OldestFirst {
			return di.Before(dj)
		}
		return di.After(dj)
	})
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

func (s *JSONStore) AddComment(ctx context.Context, c *blog.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.posts[c.PostID]; !ok {
		return ErrNotFound
	}
	c.ID = s.nextCommentID
	s.nextCommentID++
	s.comments[c.PostID] = append(s.comments[c.PostID], cloneComment(c))
	return s.writeComments(c.PostID)
}

// ListComments returns a post's comments newest-first.
func (s *JSONStore) ListComments(ctx context.Context, postID int64) ([]*blog.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	src := s.comments[postID]
	out := make([]*blog.Comment, len(src))
	for i, c := range src {
		out[len(src)-1-i] = cloneComment(c) // reverse: newest-first
	}
	return out, nil
}

func (s *JSONStore) Close() error { return nil }

func (s *JSONStore) writePost(p *blog.Post) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "posts", fmt.Sprintf("%d.json", p.ID))
	return atomicWrite(path, data)
}

func (s *JSONStore) writeComments(postID int64) error {
	data, err := json.MarshalIndent(s.comments[postID], "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "comments", fmt.Sprintf("%d.json", postID))
	return atomicWrite(path, data)
}

// atomicWrite writes to a temp file and renames it into place, so a crash
// can't leave a half-written file.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func clonePost(p *blog.Post) *blog.Post {
	cp := *p
	if p.PublishedAt != nil {
		t := *p.PublishedAt
		cp.PublishedAt = &t
	}
	return &cp
}

func cloneComment(c *blog.Comment) *blog.Comment {
	cc := *c
	return &cc
}
