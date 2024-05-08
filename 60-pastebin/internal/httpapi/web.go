package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"pastebin/internal/highlight"
	"pastebin/internal/paste"
)

const deleteKeyCookie = "pb_dk"

const timeFmt = "2006-01-02 15:04 MST"

type indexData struct {
	Languages []highlight.Language
}

type viewData struct {
	Paste     paste.Paste
	DeleteKey string
	CreatedAt string
	ExpiresAt string
	CSSClass  string
}

type errorData struct {
	Code    int
	Title   string
	Message string
}

// render renders into a buffer first, so a template error gives a plain 500
// rather than half a page.
func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	t, ok := s.templates[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		s.log.Error("template render failed", "template", name, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) renderError(w http.ResponseWriter, status int, title, message string) {
	s.render(w, status, "error", errorData{Code: status, Title: title, Message: message})
}

// webIndex handles GET /.
func (s *Server) webIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "index", indexData{Languages: languageList()})
}

// webCreate handles POST / and redirects to the new paste.
func (s *Server) webCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "Bad Request", "Could not parse the form.")
		return
	}
	res, err := s.svc.Create(r.Context(), paste.CreateParams{
		Content:   r.FormValue("content"),
		Language:  r.FormValue("language"),
		ExpiresIn: r.FormValue("expires_in"),
	})
	if err != nil {
		s.renderWebServiceError(w, err)
		return
	}

	// Pass the delete key to the next page in a short-lived cookie, so the
	// creator sees it once and it never appears in a URL.
	http.SetCookie(w, &http.Cookie{
		Name:     deleteKeyCookie,
		Value:    res.Paste.ID + ":" + res.DeleteKey,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/"+res.Paste.ID, http.StatusSeeOther)
}

// webView handles GET /{id}.
func (s *Server) webView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.svc.Get(r.Context(), id)
	if err != nil {
		s.renderWebServiceError(w, err)
		return
	}

	data := viewData{
		Paste:     p,
		CreatedAt: p.CreatedAt.Format(timeFmt),
		CSSClass:  highlight.CSSClass(p.Language),
	}
	if p.ExpiresAt != nil {
		data.ExpiresAt = p.ExpiresAt.Format(timeFmt)
	}
	if c, err := r.Cookie(deleteKeyCookie); err == nil {
		if cid, key, ok := strings.Cut(c.Value, ":"); ok && cid == id {
			data.DeleteKey = key
			http.SetCookie(w, &http.Cookie{Name: deleteKeyCookie, Path: "/", MaxAge: -1})
		}
	}
	s.render(w, http.StatusOK, "view", data)
}

// webDelete handles POST /{id}/delete, with the key in the delete_key field.
func (s *Server) webDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "Bad Request", "Could not parse the form.")
		return
	}
	key := r.FormValue("delete_key")
	err := s.svc.Delete(r.Context(), id, key)
	switch {
	case err == nil:
		http.SetCookie(w, &http.Cookie{Name: deleteKeyCookie, Path: "/", MaxAge: -1})
		s.render(w, http.StatusOK, "deleted", nil)
	case errors.Is(err, paste.ErrNotFound):
		// Already gone, which is what the user wanted anyway.
		s.render(w, http.StatusOK, "deleted", nil)
	default:
		s.renderWebServiceError(w, err)
	}
}

// handleRaw serves GET /{id}/raw and GET /api/pastes/{id}/raw as plain text.
func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.svc.Get(r.Context(), id)
	if err != nil {
		status, msg := rawErrorStatus(err)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(msg + "\n"))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(p.Content))
}

func rawErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, paste.ErrExpired):
		return http.StatusGone, "410 paste expired"
	case errors.Is(err, paste.ErrNotFound):
		return http.StatusNotFound, "404 paste not found"
	default:
		return http.StatusInternalServerError, "500 internal server error"
	}
}

func (s *Server) renderWebServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paste.ErrInvalid):
		s.renderError(w, http.StatusBadRequest, "Bad Request", "The paste is empty or the options are invalid.")
	case errors.Is(err, paste.ErrTooLarge):
		s.renderError(w, http.StatusRequestEntityTooLarge, "Too Large", "The paste exceeds the maximum allowed size.")
	case errors.Is(err, paste.ErrForbidden):
		s.renderError(w, http.StatusForbidden, "Forbidden", "The delete key is missing or incorrect.")
	case errors.Is(err, paste.ErrNotFound):
		s.renderError(w, http.StatusNotFound, "Not Found", "No such paste. It may have been deleted.")
	case errors.Is(err, paste.ErrExpired):
		s.renderError(w, http.StatusGone, "Expired", "This paste has expired and is no longer available.")
	default:
		s.log.Error("internal error", "error", err)
		s.renderError(w, http.StatusInternalServerError, "Server Error", "Something went wrong.")
	}
}
