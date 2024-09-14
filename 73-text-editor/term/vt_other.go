//go:build !windows

package term

import "os"

// enableVirtualTerminal has nothing to do outside Windows.
func enableVirtualTerminal(*os.File) func() {
	return func() {}
}
