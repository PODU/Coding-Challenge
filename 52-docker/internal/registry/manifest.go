package registry

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Manifest and index media types.
const (
	MediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaTypeDockerManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeOCIIndex           = "application/vnd.oci.image.index.v1+json"
	MediaTypeOCIManifest        = "application/vnd.oci.image.manifest.v1+json"
)

// AcceptManifests is the Accept header for manifest requests.
var AcceptManifests = strings.Join([]string{
	MediaTypeOCIIndex,
	MediaTypeOCIManifest,
	MediaTypeDockerManifestList,
	MediaTypeDockerManifest,
}, ", ")

// Platform is the OS and architecture of an index entry.
type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

// Descriptor points at a blob or manifest by digest.
type Descriptor struct {
	MediaType string    `json:"mediaType"`
	Digest    string    `json:"digest"`
	Size      int64     `json:"size"`
	Platform  *Platform `json:"platform,omitempty"`
}

// Index is a multi-arch manifest list (OCI: image index).
type Index struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []Descriptor `json:"manifests"`
}

// Manifest is a single-platform image manifest.
type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        Descriptor   `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

// ImageConfig is the part of the image config needed to start a process.
type ImageConfig struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Config       struct {
		Env        []string `json:"Env"`
		Cmd        []string `json:"Cmd"`
		Entrypoint []string `json:"Entrypoint"`
		WorkingDir string   `json:"WorkingDir"`
	} `json:"config"`
}

// IsIndexMediaType reports whether mt is a multi-arch index type.
func IsIndexMediaType(mt string) bool {
	return mt == MediaTypeDockerManifestList || mt == MediaTypeOCIIndex
}

// IsManifestMediaType reports whether mt is a single-platform manifest type.
func IsManifestMediaType(mt string) bool {
	return mt == MediaTypeDockerManifest || mt == MediaTypeOCIManifest
}

// ParseIndex decodes a multi-arch index.
func ParseIndex(data []byte) (Index, error) {
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return Index{}, fmt.Errorf("parse manifest index: %w", err)
	}
	return idx, nil
}

// ParseManifest decodes an image manifest.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Config.Digest == "" && len(m.Layers) == 0 {
		return Manifest{}, fmt.Errorf("parse manifest: no config or layers (is this a manifest list?)")
	}
	return m, nil
}

// ParseImageConfig decodes an image config blob.
func ParseImageConfig(data []byte) (ImageConfig, error) {
	var c ImageConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return ImageConfig{}, fmt.Errorf("parse image config: %w", err)
	}
	return c, nil
}

// LooksLikeIndex guesses from the document itself whether it is a multi-arch
// index, for servers that don't send a useful Content-Type.
func LooksLikeIndex(data []byte) bool {
	var probe struct {
		MediaType string            `json:"mediaType"`
		Manifests []json.RawMessage `json:"manifests"`
		Layers    []json.RawMessage `json:"layers"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	if IsIndexMediaType(probe.MediaType) {
		return true
	}
	if IsManifestMediaType(probe.MediaType) {
		return false
	}
	return len(probe.Manifests) > 0 && len(probe.Layers) == 0
}

// SelectManifest returns the entry for os/arch.
func SelectManifest(idx Index, os, arch string) (Descriptor, error) {
	for _, m := range idx.Manifests {
		if m.Platform != nil && m.Platform.OS == os && m.Platform.Architecture == arch {
			return m, nil
		}
	}
	return Descriptor{}, fmt.Errorf("no manifest found for platform %s/%s", os, arch)
}
