// Package registry is a small client for the Docker Registry HTTP API V2,
// enough to pull public images.
package registry

import (
	"fmt"
	"strings"
)

// DefaultRegistry is Docker Hub.
const DefaultRegistry = "registry-1.docker.io"

// DefaultTag is used when a reference has no tag or digest.
const DefaultTag = "latest"

// Ref is a parsed image reference.
type Ref struct {
	Registry   string // e.g. "registry-1.docker.io"
	Repository string // e.g. "library/ubuntu"
	Tag        string // e.g. "22.04"; empty when Digest is set
	Digest     string // e.g. "sha256:abc..."; empty when a Tag is used
}

// ParseRef parses references like "ubuntu", "ubuntu:22.04", "library/alpine",
// "ghcr.io/owner/name:tag" or "ubuntu@sha256:...", filling in Docker's
// defaults.
func ParseRef(s string) (Ref, error) {
	if strings.TrimSpace(s) == "" {
		return Ref{}, fmt.Errorf("empty image reference")
	}

	registry := DefaultRegistry
	name := s

	// Docker's rule: the first segment is a registry host only if it has a "."
	// or ":" in it, or is "localhost".
	if i := strings.IndexByte(s, '/'); i >= 0 {
		first := s[:i]
		if first == "localhost" || strings.ContainsAny(first, ".:") {
			registry = first
			name = s[i+1:]
		}
	}

	var digest, tag string
	if i := strings.IndexByte(name, '@'); i >= 0 {
		digest = name[i+1:]
		name = name[:i]
	}
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		tag = name[i+1:]
		name = name[:i]
	}

	if name == "" {
		return Ref{}, fmt.Errorf("invalid image reference %q: empty repository", s)
	}

	// Official images like "ubuntu" live under library/ on Docker Hub.
	if registry == DefaultRegistry && !strings.Contains(name, "/") {
		name = "library/" + name
	}

	if tag == "" && digest == "" {
		tag = DefaultTag
	}

	return Ref{Registry: registry, Repository: name, Tag: tag, Digest: digest}, nil
}

// reference is the digest if set, otherwise the tag.
func (r Ref) reference() string {
	if r.Digest != "" {
		return r.Digest
	}
	return r.Tag
}

// ManifestURL returns the URL of r's manifest.
func (r Ref) ManifestURL(scheme string) string {
	return fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, r.Registry, r.Repository, r.reference())
}

// BlobURL returns the URL of a blob in r's repository.
func (r Ref) BlobURL(scheme, digest string) string {
	return fmt.Sprintf("%s://%s/v2/%s/blobs/%s", scheme, r.Registry, r.Repository, digest)
}
