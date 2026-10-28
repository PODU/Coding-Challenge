//go:build !windows

package platform

import (
	"fmt"
	"runtime"
)

// NativeOptions selects what the native backend sets up.
type NativeOptions struct {
	Capture bool
}

// Native is only implemented for Windows. Elsewhere use the log backend,
// which speaks the full protocol but prints input instead of injecting it.
func Native(NativeOptions) (Screen, error) {
	return nil, fmt.Errorf("platform: no native input backend for %s yet; use --backend log", runtime.GOOS)
}
