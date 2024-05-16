// Package wire holds the JSON DTOs shared by the client and the server.
package wire

// FileInfo describes one file in a remote (or local) index.
type FileInfo struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Size    int64  `json:"size"`
	Version int64  `json:"version"`
}

// ListResponse is the body of GET /v1/files.
type ListResponse struct {
	Files []FileInfo `json:"files"`
}

// LoginRequest is the body of POST /v1/auth/login.
type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// LoginResponse is the body returned by a successful login.
type LoginResponse struct {
	Token string `json:"token"`
}

// ErrorResponse is the body of any non-2xx API response.
type ErrorResponse struct {
	Error string `json:"error"`
}
