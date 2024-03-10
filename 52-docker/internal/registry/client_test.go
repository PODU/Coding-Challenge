package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// hostOf strips the scheme from an httptest server URL to use as Ref.Registry.
func hostOf(serverURL string) string {
	return strings.TrimPrefix(serverURL, "http://")
}

func TestClientPull(t *testing.T) {
	configBytes := []byte(`{"architecture":"amd64","os":"linux","config":{"Cmd":["/bin/sh"],"WorkingDir":"/data"}}`)
	configDigest := digestOf(configBytes)

	layerBytes := []byte("layer-blob-contents")
	layerDigest := digestOf(layerBytes)

	childBytes := []byte(fmt.Sprintf(
		`{"schemaVersion":2,"mediaType":%q,"config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":%q,"size":%d},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":%q,"size":%d}]}`,
		MediaTypeDockerManifest, configDigest, len(configBytes), layerDigest, len(layerBytes)))
	childDigest := digestOf(childBytes)

	indexBytes := []byte(fmt.Sprintf(
		`{"schemaVersion":2,"mediaType":%q,"manifests":[{"mediaType":%q,"digest":%q,"size":%d,"platform":{"architecture":"amd64","os":"linux"}},{"mediaType":%q,"digest":"sha256:other","size":1,"platform":{"architecture":"arm64","os":"linux"}}]}`,
		MediaTypeDockerManifestList, MediaTypeDockerManifest, childDigest, len(childBytes), MediaTypeDockerManifest))

	const token = "secret-token"
	var tokenRequests, challenges int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authed := r.Header.Get("Authorization") == "Bearer "+token

		switch {
		case r.URL.Path == "/token":
			tokenRequests++
			fmt.Fprintf(w, `{"token":%q}`, token)
			return

		case strings.Contains(r.URL.Path, "/manifests/"):
			if !authed {
				challenges++
				w.Header().Set("WWW-Authenticate",
					fmt.Sprintf(`Bearer realm="http://%s/token",service="reg",scope="repository:library/test:pull"`, r.Host))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			ref := r.URL.Path[strings.LastIndex(r.URL.Path, "/manifests/")+len("/manifests/"):]
			if ref == "latest" {
				w.Header().Set("Content-Type", MediaTypeDockerManifestList)
				w.Write(indexBytes)
				return
			}
			if ref == childDigest {
				w.Header().Set("Content-Type", MediaTypeDockerManifest)
				w.Write(childBytes)
				return
			}
			http.Error(w, "no such manifest", http.StatusNotFound)
			return

		case strings.Contains(r.URL.Path, "/blobs/"):
			if !authed {
				w.Header().Set("WWW-Authenticate",
					fmt.Sprintf(`Bearer realm="http://%s/token",service="reg",scope="repository:library/test:pull"`, r.Host))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			digest := r.URL.Path[strings.LastIndex(r.URL.Path, "/blobs/")+len("/blobs/"):]
			switch digest {
			case configDigest:
				w.Write(configBytes)
			case layerDigest:
				w.Write(layerBytes)
			default:
				http.Error(w, "no such blob", http.StatusNotFound)
			}
			return
		}
		http.Error(w, "unexpected path "+r.URL.Path, http.StatusBadRequest)
	}))
	defer srv.Close()

	client := &Client{Scheme: "http"}
	ref := Ref{Registry: hostOf(srv.URL), Repository: "library/test", Tag: "latest"}

	img, err := client.Pull(context.Background(), ref)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if img.Token != token {
		t.Errorf("token = %q; want %q", img.Token, token)
	}
	if img.Config.Config.WorkingDir != "/data" {
		t.Errorf("config workdir = %q; want /data", img.Config.Config.WorkingDir)
	}
	if len(img.Manifest.Layers) != 1 || img.Manifest.Layers[0].Digest != layerDigest {
		t.Errorf("manifest layers = %+v", img.Manifest.Layers)
	}
	if challenges == 0 || tokenRequests == 0 {
		t.Errorf("expected an auth challenge + token exchange; challenges=%d tokens=%d", challenges, tokenRequests)
	}

	// Fetch the layer blob and confirm the verifying reader accepts it.
	rc, err := client.FetchBlob(context.Background(), img.Ref, layerDigest, img.Token)
	if err != nil {
		t.Fatalf("FetchBlob: %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("read layer: %v", err)
	}
	if string(got) != string(layerBytes) {
		t.Errorf("layer bytes mismatch")
	}
}

func TestFetchBlobDigestMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("corrupted-bytes"))
	}))
	defer srv.Close()

	client := &Client{Scheme: "http"}
	ref := Ref{Registry: hostOf(srv.URL), Repository: "library/test", Tag: "latest"}

	// Claim a digest that the body will not match.
	wrongDigest := digestOf([]byte("the-expected-bytes"))
	rc, err := client.FetchBlob(context.Background(), ref, wrongDigest, "tok")
	if err != nil {
		t.Fatalf("FetchBlob setup: %v", err)
	}
	defer rc.Close()
	if _, err := io.ReadAll(rc); err == nil {
		t.Fatalf("expected digest mismatch error, got nil")
	}
}
