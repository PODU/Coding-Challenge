package registry

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		in       string
		registry string
		repo     string
		tag      string
		digest   string
		wantErr  bool
	}{
		{in: "ubuntu", registry: DefaultRegistry, repo: "library/ubuntu", tag: "latest"},
		{in: "ubuntu:22.04", registry: DefaultRegistry, repo: "library/ubuntu", tag: "22.04"},
		{in: "library/alpine", registry: DefaultRegistry, repo: "library/alpine", tag: "latest"},
		{in: "bitnami/nginx:1.25", registry: DefaultRegistry, repo: "bitnami/nginx", tag: "1.25"},
		{in: "ghcr.io/owner/name:tag", registry: "ghcr.io", repo: "owner/name", tag: "tag"},
		{in: "localhost:5000/my/img", registry: "localhost:5000", repo: "my/img", tag: "latest"},
		{
			in:       "ubuntu@sha256:abc123",
			registry: DefaultRegistry,
			repo:     "library/ubuntu",
			digest:   "sha256:abc123",
		},
		{in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			ref, err := ParseRef(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRef(%q) = %+v, want error", tt.in, ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRef(%q) error: %v", tt.in, err)
			}
			if ref.Registry != tt.registry || ref.Repository != tt.repo || ref.Tag != tt.tag || ref.Digest != tt.digest {
				t.Errorf("ParseRef(%q) = %+v; want reg=%s repo=%s tag=%s digest=%s",
					tt.in, ref, tt.registry, tt.repo, tt.tag, tt.digest)
			}
		})
	}
}

func TestRefURLs(t *testing.T) {
	ref, err := ParseRef("ubuntu:22.04")
	if err != nil {
		t.Fatal(err)
	}
	wantManifest := "https://registry-1.docker.io/v2/library/ubuntu/manifests/22.04"
	if got := ref.ManifestURL("https"); got != wantManifest {
		t.Errorf("ManifestURL = %q; want %q", got, wantManifest)
	}
	wantBlob := "https://registry-1.docker.io/v2/library/ubuntu/blobs/sha256:deadbeef"
	if got := ref.BlobURL("https", "sha256:deadbeef"); got != wantBlob {
		t.Errorf("BlobURL = %q; want %q", got, wantBlob)
	}

	dref, _ := ParseRef("ubuntu@sha256:abc")
	wantDigestURL := "https://registry-1.docker.io/v2/library/ubuntu/manifests/sha256:abc"
	if got := dref.ManifestURL("https"); got != wantDigestURL {
		t.Errorf("digest ManifestURL = %q; want %q", got, wantDigestURL)
	}
}
