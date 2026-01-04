// Command mcp-server is an MCP server for the Coding Challenges Shared
// Solutions repository. It speaks line-delimited JSON-RPC 2.0 over stdin and
// stdout, so all logging goes to stderr.
package main

import (
	"log"
	"os"

	"ccshared-mcp/internal/mcp"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[ccshared-mcp] ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	srv := mcp.NewServer(mcp.NewGitHubFetcher())
	srv.SetLogger(log.Printf)

	if err := srv.Serve(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
