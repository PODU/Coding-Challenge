//go:build linux

package main

import (
	"os"

	"cctmux/internal/client"
	"cctmux/internal/protocol"
	"cctmux/internal/server"
)

func platformRun(cfg Config, req request) error {
	if req.server {
		return server.Run(cfg.Socket, os.Getenv("SHELL"), cfg.Prefix, 80, 24)
	}
	verb := req.verb
	if verb == 0 {
		verb = protocol.NewSession
	}
	return client.Run(client.Options{
		SockPath: cfg.Socket,
		Verb:     verb,
		Target:   req.target,
	})
}
