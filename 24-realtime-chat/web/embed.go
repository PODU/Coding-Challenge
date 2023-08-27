// Package web embeds the browser client so the server is a single binary.
package web

import "embed"

//go:embed index.html app.js style.css
var Files embed.FS
