package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"blog/internal/blog"
	"blog/internal/render"
	"blog/internal/store"

	"golang.org/x/crypto/bcrypt"
)

const testPassword = "s3cret-pass"

var fixedNow = time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

// newTestServer builds a Server backed by an in-memory SQLite store with a
// fixed clock and known admin credentials.
func newTestServer(t *testing.T) (*Server, store.Store) {
	t.Helper()
	r := render.New()
	st, err := store.OpenSQLite(":memory:", r.Render)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	hash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	srv, err := New(Config{
		Site:              Site{Title: "Test Blog", Description: "testing"},
		AdminUser:         "admin",
		AdminPasswordHash: string(hash),
		BaseURL:           "http://example.test",
		Now:               blog.FixedClock(fixedNow),
	}, st, r)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, st
}

// seedPost inserts a post directly via the store for handler tests.
func seedPost(t *testing.T, st store.Store, title string, status blog.Status, publishedAt *time.Time) int64 {
	t.Helper()
	p := &blog.Post{
		Title:       title,
		Summary:     "summary of " + title,
		MarkdownRaw: "# " + title + "\n\nBody.",
		Status:      status,
		PublishedAt: publishedAt,
		CreatedAt:   fixedNow,
		UpdatedAt:   fixedNow,
	}
	id, err := st.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return id
}

// agent is a minimal cookie-tracking client over an http.Handler, avoiding any
// bound network port.
type agent struct {
	h       http.Handler
	cookies map[string]string
}

func newAgent(h http.Handler) *agent {
	return &agent{h: h, cookies: map[string]string{}}
}

func (a *agent) do(method, target string, form url.Values) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range a.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(a.cookies, c.Name)
		} else {
			a.cookies[c.Name] = c.Value
		}
	}
	return rec
}

var csrfRe = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func extractCSRF(t *testing.T, body string) string {
	t.Helper()
	m := csrfRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no csrf token in body:\n%s", body)
	}
	return m[1]
}

// login performs the full login handshake and returns an authenticated agent.
func login(t *testing.T, srv *Server) *agent {
	t.Helper()
	a := newAgent(srv)
	rec := a.do("GET", "/editor/login", nil)
	csrf := extractCSRF(t, rec.Body.String())
	form := url.Values{"username": {"admin"}, "password": {testPassword}, "csrf_token": {csrf}}
	rec = a.do("POST", "/editor/login", form)
	if rec.Code != http.StatusFound {
		t.Fatalf("login: code = %d, want 302\n%s", rec.Code, rec.Body.String())
	}
	return a
}

func TestHomeListsPublishedNewestFirst(t *testing.T) {
	srv, st := newTestServer(t)
	past := fixedNow.Add(-48 * time.Hour)
	older := fixedNow.Add(-72 * time.Hour)
	future := fixedNow.Add(48 * time.Hour)
	seedPost(t, st, "Older", blog.Published, &older)
	seedPost(t, st, "Newer", blog.Published, &past)
	seedPost(t, st, "Draft", blog.Draft, nil)
	seedPost(t, st, "Scheduled", blog.Published, &future)

	rec := newAgent(srv).do("GET", "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("home code = %d", rec.Code)
	}
	body := rec.Body.String()
	iNewer := strings.Index(body, "Newer")
	iOlder := strings.Index(body, "Older")
	if iNewer < 0 || iOlder < 0 {
		t.Fatalf("published posts missing from home:\n%s", body)
	}
	if iNewer > iOlder {
		t.Errorf("home not newest-first: Newer at %d, Older at %d", iNewer, iOlder)
	}
	if strings.Contains(body, "Draft") {
		t.Error("draft visible on home")
	}
	if strings.Contains(body, "Scheduled") {
		t.Error("future-scheduled post visible on home")
	}
}

func TestPermalinkPublishedAndDraft(t *testing.T) {
	srv, st := newTestServer(t)
	past := fixedNow.Add(-time.Hour)
	pubID := seedPost(t, st, "Visible", blog.Published, &past)
	draftID := seedPost(t, st, "Hidden", blog.Draft, nil)

	rec := newAgent(srv).do("GET", "/posts/"+itoa(pubID), nil)
	if rec.Code != http.StatusOK {
		t.Errorf("published permalink code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Visible") {
		t.Error("published post body missing title")
	}

	rec = newAgent(srv).do("GET", "/posts/"+itoa(draftID), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("draft permalink code = %d, want 404", rec.Code)
	}

	rec = newAgent(srv).do("GET", "/posts/9999", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing permalink code = %d, want 404", rec.Code)
	}
}

func TestEditorAuthGate(t *testing.T) {
	srv, _ := newTestServer(t)

	// Unauthenticated -> redirect to login.
	rec := newAgent(srv).do("GET", "/editor", nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/editor/login" {
		t.Errorf("unauth /editor: code=%d loc=%q, want 302 -> /editor/login", rec.Code, rec.Header().Get("Location"))
	}

	// Bad credentials -> 401.
	a := newAgent(srv)
	rec = a.do("GET", "/editor/login", nil)
	csrf := extractCSRF(t, rec.Body.String())
	rec = a.do("POST", "/editor/login", url.Values{"username": {"admin"}, "password": {"wrong"}, "csrf_token": {csrf}})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad login code = %d, want 401", rec.Code)
	}

	// Good credentials -> can reach editor list.
	authed := login(t, srv)
	rec = authed.do("GET", "/editor", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("authed /editor code = %d, want 200", rec.Code)
	}
}

func TestEditorLoginCSRFRequired(t *testing.T) {
	srv, _ := newTestServer(t)
	a := newAgent(srv)
	a.do("GET", "/editor/login", nil) // establishes session cookie
	rec := a.do("POST", "/editor/login", url.Values{"username": {"admin"}, "password": {testPassword}})
	if rec.Code != http.StatusForbidden {
		t.Errorf("login without csrf: code = %d, want 403", rec.Code)
	}
}

func TestCreatePostFlow(t *testing.T) {
	srv, st := newTestServer(t)
	a := login(t, srv)

	// Fetch the new-post form to obtain a CSRF token.
	rec := a.do("GET", "/editor/post", nil)
	csrf := extractCSRF(t, rec.Body.String())

	form := url.Values{
		"title":      {"My First Post"},
		"summary":    {"A summary"},
		"content":    {"# Heading\n\nHello **world**."},
		"status":     {"published"},
		"csrf_token": {csrf},
	}
	rec = a.do("POST", "/editor/post", form)
	if rec.Code != http.StatusFound {
		t.Fatalf("create code = %d, want 302\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/posts/") {
		t.Fatalf("redirect = %q, want /posts/{id}", loc)
	}

	// Post is publicly visible at its permalink with rendered markdown.
	rec = newAgent(srv).do("GET", loc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("permalink code = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<strong>world</strong>") {
		t.Errorf("markdown not rendered on permalink:\n%s", body)
	}

	// And on the homepage.
	rec = newAgent(srv).do("GET", "/", nil)
	if !strings.Contains(rec.Body.String(), "My First Post") {
		t.Error("new post not on homepage")
	}

	// Published date was stamped to the injected clock.
	ctx := context.Background()
	all, _ := st.List(ctx, store.ListOpts{})
	if len(all) != 1 || all[0].PublishedAt == nil || !all[0].PublishedAt.Equal(fixedNow) {
		t.Errorf("published_at not stamped: %+v", all[0])
	}
}

func TestCreatePostValidationError(t *testing.T) {
	srv, _ := newTestServer(t)
	a := login(t, srv)
	rec := a.do("GET", "/editor/post", nil)
	csrf := extractCSRF(t, rec.Body.String())

	// Missing title -> validation error, re-render with 422.
	rec = a.do("POST", "/editor/post", url.Values{
		"title": {""}, "content": {"x"}, "status": {"draft"}, "csrf_token": {csrf},
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty title create: code = %d, want 422", rec.Code)
	}
}

func TestCreatePostCSRFRequired(t *testing.T) {
	srv, _ := newTestServer(t)
	a := login(t, srv)
	rec := a.do("POST", "/editor/post", url.Values{
		"title": {"No CSRF"}, "content": {"x"}, "status": {"published"},
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("create without csrf: code = %d, want 403", rec.Code)
	}
}

func TestEditPostDraftToPublishStampsDate(t *testing.T) {
	srv, st := newTestServer(t)
	id := seedPost(t, st, "Draft Post", blog.Draft, nil)
	a := login(t, srv)

	rec := a.do("GET", "/editor/post/"+itoa(id), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit form code = %d", rec.Code)
	}
	csrf := extractCSRF(t, rec.Body.String())

	rec = a.do("POST", "/editor/post/"+itoa(id), url.Values{
		"title": {"Draft Post Published"}, "summary": {"s"}, "content": {"body"},
		"status": {"published"}, "csrf_token": {csrf},
	})
	if rec.Code != http.StatusFound {
		t.Fatalf("update code = %d, want 302\n%s", rec.Code, rec.Body.String())
	}

	got, _ := st.Get(context.Background(), id)
	if got.Status != blog.Published {
		t.Errorf("status = %q, want published", got.Status)
	}
	if got.PublishedAt == nil || !got.PublishedAt.Equal(fixedNow) {
		t.Errorf("published_at = %v, want %v", got.PublishedAt, fixedNow)
	}
	if got.Title != "Draft Post Published" {
		t.Errorf("title not updated: %q", got.Title)
	}

	// Now publicly visible.
	rec = newAgent(srv).do("GET", "/posts/"+itoa(id), nil)
	if rec.Code != http.StatusOK {
		t.Errorf("published permalink code = %d", rec.Code)
	}
}

func TestAddCommentFlow(t *testing.T) {
	srv, st := newTestServer(t)
	past := fixedNow.Add(-time.Hour)
	id := seedPost(t, st, "Commentable", blog.Published, &past)

	a := newAgent(srv)
	rec := a.do("POST", "/posts/"+itoa(id)+"/comments", url.Values{
		"name": {"Alice"}, "email": {"alice@example.com"}, "body": {"Great post!"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("comment code = %d, want 303\n%s", rec.Code, rec.Body.String())
	}

	rec = a.do("POST", "/posts/"+itoa(id)+"/comments", url.Values{
		"name": {"Bob"}, "email": {"bob@example.com"}, "body": {"<script>alert('xss')</script>"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("second comment code = %d", rec.Code)
	}

	// View post: comments newest-first, XSS escaped.
	rec = newAgent(srv).do("GET", "/posts/"+itoa(id), nil)
	body := rec.Body.String()
	iBob := strings.Index(body, "Bob")
	iAlice := strings.Index(body, "Alice")
	if iBob < 0 || iAlice < 0 {
		t.Fatalf("comments missing:\n%s", body)
	}
	if iBob > iAlice {
		t.Error("comments not newest-first")
	}
	if strings.Contains(body, "<script>alert('xss')</script>") {
		t.Error("comment XSS not escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("expected escaped comment body")
	}
	// Email must never be rendered.
	if strings.Contains(body, "alice@example.com") {
		t.Error("commenter email leaked into page")
	}
}

func TestAddCommentValidation(t *testing.T) {
	srv, st := newTestServer(t)
	past := fixedNow.Add(-time.Hour)
	id := seedPost(t, st, "P", blog.Published, &past)

	rec := newAgent(srv).do("POST", "/posts/"+itoa(id)+"/comments", url.Values{
		"name": {""}, "email": {"x@y.com"}, "body": {"hi"},
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid comment code = %d, want 422", rec.Code)
	}
}

func TestCommentOnDraftRejected(t *testing.T) {
	srv, st := newTestServer(t)
	id := seedPost(t, st, "Draft", blog.Draft, nil)
	rec := newAgent(srv).do("POST", "/posts/"+itoa(id)+"/comments", url.Values{
		"name": {"A"}, "email": {"a@b.com"}, "body": {"hi"},
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("comment on draft code = %d, want 404", rec.Code)
	}
}

func TestFeedEndpoint(t *testing.T) {
	srv, st := newTestServer(t)
	past := fixedNow.Add(-time.Hour)
	seedPost(t, st, "Published One", blog.Published, &past)
	seedPost(t, st, "A Draft", blog.Draft, nil)

	rec := newAgent(srv).do("GET", "/feed.xml", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed code = %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/rss+xml") {
		t.Errorf("content-type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Published One") {
		t.Error("published post missing from feed")
	}
	if strings.Contains(body, "A Draft") {
		t.Error("draft present in feed")
	}
	if !strings.Contains(body, "http://example.test/posts/") {
		t.Error("feed missing canonical permalink")
	}
}

func TestStaticAssets(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := newAgent(srv).do("GET", "/static/style.css", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("static code = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "site-header") {
		t.Error("css content missing")
	}
}

func TestEditorDisabledWithoutCreds(t *testing.T) {
	r := render.New()
	st, _ := store.OpenSQLite(":memory:", r.Render)
	t.Cleanup(func() { st.Close() })
	srv, err := New(Config{Site: Site{Title: "No Editor"}, Now: blog.FixedClock(fixedNow)}, st, r)
	if err != nil {
		t.Fatal(err)
	}
	rec := newAgent(srv).do("GET", "/editor", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("editor disabled code = %d, want 503", rec.Code)
	}
	// Public blog still serves.
	rec = newAgent(srv).do("GET", "/", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("home code = %d with editor disabled", rec.Code)
	}
}

// itoa is a tiny int64 -> string helper for test URLs.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
