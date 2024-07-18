package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"blog/internal/blog"
	"blog/internal/feed"
	"blog/internal/store"
)

// commentForm holds what was typed, to refill the form after an error.
type commentForm struct {
	Name  string
	Email string
	Body  string
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	posts, err := s.store.List(r.Context(), store.ListOpts{
		Status:          blog.Published,
		PublishedBefore: &now,
		Order:           store.NewestFirst,
	})
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load posts.")
		return
	}
	s.render(w, http.StatusOK, "home", struct {
		Site  Site
		Posts []*blog.Post
	}{s.cfg.Site, posts})
}

func (s *Server) handleShowPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	s.showPost(w, r, id, http.StatusOK, nil, commentForm{})
}

// showPost is also used to redisplay the page after a rejected comment.
func (s *Server) showPost(w http.ResponseWriter, r *http.Request, id int64, status int, commentErrors []string, form commentForm) {
	post, err := s.store.Get(r.Context(), id)
	if err == store.ErrNotFound {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load post.")
		return
	}
	if !post.IsPublic(s.now()) {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	comments, err := s.store.ListComments(r.Context(), id)
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load comments.")
		return
	}
	s.render(w, status, "post", struct {
		Site          Site
		Post          *blog.Post
		Comments      []*blog.Comment
		CommentErrors []string
		Form          commentForm
	}{s.cfg.Site, post, comments, commentErrors, form})
}

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "Bad request.")
		return
	}

	form := commentForm{
		Name:  r.FormValue("name"),
		Email: r.FormValue("email"),
		Body:  r.FormValue("body"),
	}

	// Rate limit per IP to slow down spam.
	if !s.allowComment(clientIP(r)) {
		s.showPost(w, r, id, http.StatusTooManyRequests,
			[]string{"You are commenting too quickly. Please wait a moment."}, form)
		return
	}

	c := &blog.Comment{
		PostID:    id,
		Name:      form.Name,
		Email:     form.Email,
		Body:      form.Body,
		CreatedAt: s.now(),
	}
	if err := c.Validate(); err != nil {
		s.showPost(w, r, id, http.StatusUnprocessableEntity, []string{err.Error()}, form)
		return
	}

	// No comments on drafts or scheduled posts.
	post, err := s.store.Get(r.Context(), id)
	if err == store.ErrNotFound || (err == nil && !post.IsPublic(s.now())) {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load post.")
		return
	}

	if err := s.store.AddComment(r.Context(), c); err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not save comment.")
		return
	}
	http.Redirect(w, r, "/posts/"+strconv.FormatInt(id, 10)+"#comments", http.StatusSeeOther)
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	posts, err := s.store.List(r.Context(), store.ListOpts{
		Status:          blog.Published,
		PublishedBefore: &now,
		Order:           store.NewestFirst,
		Limit:           s.cfg.FeedLimit,
	})
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load feed.")
		return
	}
	body, err := feed.Render(posts, feed.Options{
		Title:       s.cfg.Site.Title,
		Description: s.cfg.Site.Description,
		BaseURL:     s.cfg.BaseURL,
		BuildTime:   now,
	})
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not render feed.")
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// allowComment allows rlLimit comments per IP in any rlWin window.
func (s *Server) allowComment(ip string) bool {
	now := s.now()
	s.rlMu.Lock()
	defer s.rlMu.Unlock()
	cutoff := now.Add(-s.rlWin)
	hits := s.rlHits[ip][:0:0]
	for _, t := range s.rlHits[ip] {
		if t.After(cutoff) {
			hits = append(hits, t)
		}
	}
	if len(hits) >= s.rlLimit {
		s.rlHits[ip] = hits
		return false
	}
	hits = append(hits, now)
	s.rlHits[ip] = hits
	return true
}

// clientIP is a best guess at the client's address, for rate limiting only.
// X-Forwarded-For can be spoofed, so don't use it for anything that matters.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return first
	}
	if i := strings.LastIndexByte(r.RemoteAddr, ':'); i >= 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

// parsePublishedAt parses an optional RFC3339 time. ok is false if it's invalid.
func parsePublishedAt(s string) (*time.Time, bool) {
	if s == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, false
	}
	u := t.UTC()
	return &u, true
}
