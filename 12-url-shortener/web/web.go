// Package web embeds the GUI page so the server is a single binary.
package web

import _ "embed"

// IndexHTML is the embedded GUI served at GET /.
//
//go:embed index.html
var IndexHTML []byte
