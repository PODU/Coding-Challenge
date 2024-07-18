package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"blog/internal/blog"
	"blog/internal/store"
)

type editorFormData struct {
	Site           Site
	Post           *blog.Post
	Action         string
	CSRFToken      string
	Errors         []string
	PublishedAtStr string
}

func (s *Server) sessionCSRF(r *http.Request) string {
	if _, sess, ok := s.getSession(r); ok {
		return sess.csrf
	}
	return ""
}

func (s *Server) handleEditorList(w http.ResponseWriter, r *http.Request) {
	posts, err := s.store.List(r.Context(), store.ListOpts{Order: store.NewestFirst})
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load posts.")
		return
	}
	s.render(w, http.StatusOK, "editor_list", struct {
		Site      Site
		Posts     []*blog.Post
		CSRFToken string
	}{s.cfg.Site, posts, s.sessionCSRF(r)})
}

func (s *Server) handleNewPostForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "editor_form", editorFormData{
		Site:      s.cfg.Site,
		Post:      &blog.Post{Status: blog.Draft},
		Action:    "/editor/post",
		CSRFToken: s.sessionCSRF(r),
	})
}

func (s *Server) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "Bad request.")
		return
	}
	if !s.checkCSRF(r) {
		s.renderError(w, http.StatusForbidden, "Invalid CSRF token.")
		return
	}

	post, publishedStr, errs := s.postFromForm(r, &blog.Post{})
	if len(errs) > 0 {
		s.renderForm(w, "/editor/post", post, publishedStr, s.sessionCSRF(r), errs)
		return
	}

	now := s.now()
	post.CreatedAt = now
	post.UpdatedAt = now
	// Publishing without a date means publish now.
	if post.Status == blog.Published && post.PublishedAt == nil {
		post.PublishedAt = &now
	}

	id, err := s.store.Create(r.Context(), post)
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not save post.")
		return
	}
	http.Redirect(w, r, "/posts/"+strconv.FormatInt(id, 10), http.StatusFound)
}

func (s *Server) handleEditPostForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	post, err := s.store.Get(r.Context(), id)
	if err == store.ErrNotFound {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load post.")
		return
	}
	publishedStr := ""
	if post.PublishedAt != nil {
		publishedStr = post.PublishedAt.UTC().Format(time.RFC3339)
	}
	s.renderForm(w, "/editor/post/"+strconv.FormatInt(id, 10), post, publishedStr, s.sessionCSRF(r), nil)
}

func (s *Server) handleUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "Bad request.")
		return
	}
	if !s.checkCSRF(r) {
		s.renderError(w, http.StatusForbidden, "Invalid CSRF token.")
		return
	}

	existing, err := s.store.Get(r.Context(), id)
	if err == store.ErrNotFound {
		s.renderError(w, http.StatusNotFound, "Post not found.")
		return
	}
	if err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not load post.")
		return
	}

	post, publishedStr, errs := s.postFromForm(r, existing)
	if len(errs) > 0 {
		s.renderForm(w, "/editor/post/"+strconv.FormatInt(id, 10), post, publishedStr, s.sessionCSRF(r), errs)
		return
	}

	now := s.now()
	post.UpdatedAt = now
	// First time published and no date given: publish now.
	if post.Status == blog.Published && post.PublishedAt == nil {
		post.PublishedAt = &now
	}

	if err := s.store.Update(r.Context(), post); err != nil {
		s.renderError(w, http.StatusInternalServerError, "Could not save post.")
		return
	}
	http.Redirect(w, r, "/posts/"+strconv.FormatInt(id, 10), http.StatusFound)
}

// postFromForm applies the form to a copy of base. It also returns the raw
// published_at text, so the form can be shown again as typed if it's invalid.
func (s *Server) postFromForm(r *http.Request, base *blog.Post) (*blog.Post, string, []string) {
	p := *base // copy id, slug, created_at
	p.Title = strings.TrimSpace(r.FormValue("title"))
	p.Summary = strings.TrimSpace(r.FormValue("summary"))
	p.MarkdownRaw = r.FormValue("content")

	switch r.FormValue("status") {
	case "published", "publish":
		p.Status = blog.Published
	default:
		p.Status = blog.Draft
	}

	var errs []string
	publishedStr := r.FormValue("published_at")
	if pa, ok := parsePublishedAt(publishedStr); ok {
		p.PublishedAt = pa
	} else {
		errs = append(errs, "Published at must be a valid RFC3339 timestamp.")
	}

	if err := p.Validate(); err != nil {
		errs = append(errs, err.Error())
	}
	return &p, publishedStr, errs
}

func (s *Server) renderForm(w http.ResponseWriter, action string, post *blog.Post, publishedStr, csrf string, errs []string) {
	status := http.StatusOK
	if len(errs) > 0 {
		status = http.StatusUnprocessableEntity
	}
	s.render(w, status, "editor_form", editorFormData{
		Site:           s.cfg.Site,
		Post:           post,
		Action:         action,
		CSRFToken:      csrf,
		Errors:         errs,
		PublishedAtStr: publishedStr,
	})
}
