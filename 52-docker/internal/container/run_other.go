//go:build !linux

package container

import (
	"fmt"
	"os"

	"ccrun/internal/registry"
)

// Run always fails here: containers need Linux namespaces and cgroups.
func Run(spec Spec, rootfs string, cfg registry.ImageConfig) (int, error) {
	return 1, ErrUnsupported
}

// Child is never reached outside Linux; it exists so the package builds.
func Child() {
	fmt.Fprintln(os.Stderr, "ccrun: "+ErrUnsupported.Error())
	os.Exit(1)
}
