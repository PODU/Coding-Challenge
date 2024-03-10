// Command ccrun is a toy `docker run`. It pulls an image from Docker Hub,
// unpacks it into a directory and runs a process in it inside Linux
// namespaces, with optional cgroup limits.
//
// Pulling and unpacking work anywhere; actually running a container needs
// Linux.
package main

import (
	"context"
	"fmt"
	"os"

	"ccrun/internal/cli"
	"ccrun/internal/container"
	"ccrun/internal/imagefs"
	"ccrun/internal/registry"
)

func main() {
	// We are the re-exec'd copy running inside the new namespaces.
	if len(os.Args) > 1 && os.Args[1] == container.ReexecArg {
		container.Child()
		return
	}

	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	spec, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccrun: %v\n", err)
		return 2
	}

	rootfs := spec.Rootfs
	var cfg registry.ImageConfig

	if rootfs == "" {
		dir, c, err := pullImage(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ccrun: pull: %v\n", err)
			return 1
		}
		rootfs, cfg = dir, c
	}

	code, err := container.Run(spec, rootfs, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccrun: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// pullImage downloads the image and unpacks its layers into a new temp
// directory, which it returns along with the image config.
func pullImage(spec container.Spec) (string, registry.ImageConfig, error) {
	ref, err := registry.ParseRef(spec.Image)
	if err != nil {
		return "", registry.ImageConfig{}, err
	}
	if spec.Tag != "" {
		ref.Tag = spec.Tag
		ref.Digest = ""
	}

	ctx := context.Background()
	client := &registry.Client{}

	fmt.Fprintf(os.Stderr, "ccrun: pulling %s/%s:%s\n", ref.Registry, ref.Repository, ref.Tag)
	img, err := client.Pull(ctx, ref)
	if err != nil {
		return "", registry.ImageConfig{}, err
	}

	rootfs, err := os.MkdirTemp("", "ccrun-rootfs-")
	if err != nil {
		return "", registry.ImageConfig{}, fmt.Errorf("create rootfs dir: %w", err)
	}

	for i, layer := range img.Manifest.Layers {
		fmt.Fprintf(os.Stderr, "ccrun: extracting layer %d/%d (%s)\n", i+1, len(img.Manifest.Layers), layer.Digest)
		rc, err := client.FetchBlob(ctx, img.Ref, layer.Digest, img.Token)
		if err != nil {
			return "", registry.ImageConfig{}, err
		}
		err = imagefs.Extract(rc, rootfs)
		rc.Close()
		if err != nil {
			return "", registry.ImageConfig{}, fmt.Errorf("extract layer %s: %w", layer.Digest, err)
		}
	}

	return rootfs, img.Config, nil
}
