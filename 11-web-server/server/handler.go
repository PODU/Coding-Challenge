package server

import (
	"errors"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// errorBody renders a tiny HTML page for an error status.
func errorBody(status int) []byte {
	return []byte("<!DOCTYPE html>\n<html><head><title>" +
		statusText(status) + "</title></head><body><h1>" +
		statusText(status) + "</h1></body></html>\n")
}

// statusText is "<code> <reason>", e.g. "404 Not Found".
func statusText(status int) string {
	return strconv.Itoa(status) + " " + reason(status)
}

// Handle serves req from the document root. Only GET and HEAD are allowed.
func Handle(req *Request, root string) *Response {
	if req.Method != "GET" && req.Method != "HEAD" {
		resp := errorResponse(405, false)
		resp.Headers["Allow"] = "GET, HEAD"
		return resp
	}
	head := req.Method == "HEAD"

	abs, err := Resolve(root, req.Target)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			return errorResponse(403, head)
		}
		return errorResponse(400, head)
	}

	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return errorResponse(403, head)
		}
		return errorResponse(404, head)
	}

	// Directories serve their index.html; there is no directory listing.
	if info.IsDir() {
		indexPath := filepath.Join(abs, "index.html")
		idxInfo, err := os.Stat(indexPath)
		if err != nil || idxInfo.IsDir() {
			return errorResponse(404, head)
		}
		abs = indexPath
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return errorResponse(403, head)
		}
		return errorResponse(500, head)
	}

	resp := newResponse(200, contentType(abs), data)
	resp.HeadOnly = head
	return resp
}

func errorResponse(status int, head bool) *Response {
	resp := newResponse(status, "text/html; charset=utf-8", errorBody(status))
	resp.HeadOnly = head
	return resp
}

func contentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ct, ok := builtinTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// builtinTypes pins the common types so behaviour is identical across hosts
// regardless of the OS mime database.
var builtinTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".htm":  "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "application/javascript",
	".json": "application/json",
	".txt":  "text/plain; charset=utf-8",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
}
