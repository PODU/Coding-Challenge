package web

import "net/http"

func (s *Server) routes() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /posts/{id}", s.handleShowPost)
	mux.HandleFunc("POST /posts/{id}/comments", s.handleAddComment)
	mux.HandleFunc("GET /feed.xml", s.handleFeed)
	mux.Handle("GET /static/", s.staticHandler())

	// Login and logout
	mux.HandleFunc("GET /editor/login", s.handleLoginForm)
	mux.HandleFunc("POST /editor/login", s.handleLogin)
	mux.HandleFunc("POST /editor/logout", s.handleLogout)

	// Editor (login required)
	mux.Handle("GET /editor", s.requireAuth(http.HandlerFunc(s.handleEditorList)))
	mux.Handle("GET /editor/{$}", s.requireAuth(http.HandlerFunc(s.handleEditorList)))
	mux.Handle("GET /editor/post", s.requireAuth(http.HandlerFunc(s.handleNewPostForm)))
	mux.Handle("POST /editor/post", s.requireAuth(http.HandlerFunc(s.handleCreatePost)))
	mux.Handle("GET /editor/post/{id}", s.requireAuth(http.HandlerFunc(s.handleEditPostForm)))
	mux.Handle("POST /editor/post/{id}", s.requireAuth(http.HandlerFunc(s.handleUpdatePost)))

	s.mux = mux
}
