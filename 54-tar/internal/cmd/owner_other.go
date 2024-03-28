//go:build !unix

package cmd

import (
	"os"

	"mytar/internal/ustar"
)

// fillOwner is a no-op on hosts without POSIX owner info (e.g. Windows); UID
// and GID default to 0 and uname/gname to "root".
func fillOwner(h *ustar.Header, info os.FileInfo) {}
