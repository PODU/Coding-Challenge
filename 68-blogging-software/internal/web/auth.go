package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "blog_session"

// randomToken returns 32 random bytes, hex encoded.
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return hex.EncodeToString(b)
}

func (s *Server) getSession(r *http.Request) (id string, sess *session, ok bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", nil, false
	}
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	sess, ok = s.sessions[c.Value]
	if !ok {
		return "", nil, false
	}
	if s.now().After(sess.expiresAt) {
		delete(s.sessions, c.Value)
		return "", nil, false
	}
	return c.Value, sess, true
}

// ensureSession returns the request's session, creating an anonymous one if
// needed. The login form needs a session to hold its CSRF token before anyone
// has logged in.
func (s *Server) ensureSession(w http.ResponseWriter, r *http.Request) *session {
	if _, sess, ok := s.getSession(r); ok {
		return sess
	}
	id := randomToken()
	sess := &session{
		csrf:      randomToken(),
		expiresAt: s.now().Add(s.cfg.SessionTTL),
	}
	s.sessMu.Lock()
	s.sessions[id] = sess
	s.sessMu.Unlock()
	s.setSessionCookie(w, id)
	return sess
}

func (s *Server) setSessionCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// requireAuth redirects to the login page unless logged in.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.editorEnabled() {
			s.renderError(w, http.StatusServiceUnavailable, "The editor is disabled: no admin credentials configured.")
			return
		}
		_, sess, ok := s.getSession(r)
		if !ok || !sess.authed {
			http.Redirect(w, r, "/editor/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) checkCSRF(r *http.Request) bool {
	_, sess, ok := s.getSession(r)
	if !ok {
		return false
	}
	token := r.FormValue("csrf_token")
	if token == "" || sess.csrf == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(sess.csrf)) == 1
}

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if !s.editorEnabled() {
		s.renderError(w, http.StatusServiceUnavailable, "The editor is disabled: no admin credentials configured.")
		return
	}
	if _, sess, ok := s.getSession(r); ok && sess.authed {
		http.Redirect(w, r, "/editor", http.StatusFound)
		return
	}
	sess := s.ensureSession(w, r)
	s.render(w, http.StatusOK, "login", struct {
		Site      Site
		CSRFToken string
		Error     string
	}{s.cfg.Site, sess.csrf, ""})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.editorEnabled() {
		s.renderError(w, http.StatusServiceUnavailable, "The editor is disabled: no admin credentials configured.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "Bad request")
		return
	}
	sess := s.ensureSession(w, r)

	if !s.checkCSRF(r) {
		s.renderError(w, http.StatusForbidden, "Invalid CSRF token")
		return
	}

	user := r.FormValue("username")
	pass := r.FormValue("password")
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(s.cfg.AdminUser)) == 1
	passOK := bcrypt.CompareHashAndPassword([]byte(s.cfg.AdminPasswordHash), []byte(pass)) == nil

	if !userOK || !passOK {
		s.render(w, http.StatusUnauthorized, "login", struct {
			Site      Site
			CSRFToken string
			Error     string
		}{s.cfg.Site, sess.csrf, "Invalid username or password."})
		return
	}

	sess.user = s.cfg.AdminUser
	sess.authed = true
	sess.expiresAt = s.now().Add(s.cfg.SessionTTL)
	http.Redirect(w, r, "/editor", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !s.checkCSRF(r) {
		s.renderError(w, http.StatusForbidden, "Invalid CSRF token")
		return
	}
	if id, _, ok := s.getSession(r); ok {
		s.sessMu.Lock()
		delete(s.sessions, id)
		s.sessMu.Unlock()
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/editor/login", http.StatusFound)
}
