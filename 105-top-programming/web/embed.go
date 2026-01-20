// Package web embeds the static frontend assets served by cmd/api.
package web

import "embed"

// FS holds the static frontend files (index.html, app.js).
//
//go:embed index.html app.js
var FS embed.FS
