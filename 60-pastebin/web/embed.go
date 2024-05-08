// Package web embeds the templates and static files into the binary.
package web

import "embed"

//go:embed templates/*.html
var Templates embed.FS

// Static is served under /static/.
//
//go:embed static/*
var Static embed.FS
