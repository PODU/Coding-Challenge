package main

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/podu2997/cckvm/internal/client"
	"github.com/podu2997/cckvm/internal/layout"
	"github.com/podu2997/cckvm/internal/platform"
	"github.com/podu2997/cckvm/internal/platform/platformtest"
	"github.com/podu2997/cckvm/internal/server"
)

func TestParseSize(t *testing.T) {
	if w, h, err := parseSize("2560X1440"); err != nil || w != 2560 || h != 1440 {
		t.Fatalf("%d %d %v", w, h, err)
	}
	for _, bad := range []string{"", "1920", "x", "0x0", "-5x10", "99999x10"} {
		if _, _, err := parseSize(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestFingerprintFormat(t *testing.T) {
	fp := fingerprintOf([]byte("x"))
	if !strings.HasPrefix(fp, "SHA256:") || len(fp) != len("SHA256:")+32*3-1 {
		t.Fatalf("%s", fp)
	}
	if normalizeFingerprint(strings.ToLower(fp)) != normalizeFingerprint(strings.ReplaceAll(fp[7:], ":", "")) {
		t.Fatal("normalisation")
	}
}

// TestTLSEndToEnd runs server and client over TLS with a pinned,
// freshly generated certificate, and checks that a wrong pin is refused.
func TestTLSEndToEnd(t *testing.T) {
	certPath := filepath.Join(t.TempDir(), "server.pem")
	srvTLS, fp, err := serverTLS(certPath)
	if err != nil {
		t.Fatal(err)
	}
	// Loading again reuses the file, so the fingerprint is stable.
	if _, fp2, err := serverTLS(certPath); err != nil || fp2 != fp {
		t.Fatalf("fingerprint changed: %s %s %v", fp, fp2, err)
	}

	lay, _ := layout.Parse(strings.NewReader("section: screens\na:\nb:\nend\nsection: links\na:\nright = b\nb:\nleft = a\nend"))
	desk := platformtest.NewFake(platform.Rect{W: 100, H: 100})
	srv, err := server.New(server.Config{Layout: lay, Name: "a", Screen: desk})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx, tlsListener(raw, srvTLS))

	if _, err := clientTLS(""); err == nil {
		t.Fatal("TLS without a pin accepted")
	}
	wrong, _ := clientTLS(fingerprintOf([]byte("someone else")))
	err = client.Run(ctx, client.Config{Name: "b", Addr: raw.Addr().String(), Screen: platformtest.NewFake(platform.Rect{W: 50, H: 50}), Dial: wrong})
	if err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("wrong pin: %v", err)
	}

	dial, _ := clientTLS(fp)
	lap := platformtest.NewFake(platform.Rect{W: 50, H: 50})
	go client.Run(ctx, client.Config{Name: "b", Addr: raw.Addr().String(), Screen: lap, Dial: dial})
	deadline := time.Now().Add(5 * time.Second)
	for !desk.Captured() {
		if time.Now().After(deadline) {
			t.Fatal("never switched over TLS")
		}
		desk.Emit(platform.Event{Kind: platform.MouseMotion, X: 99, Y: 50})
		time.Sleep(20 * time.Millisecond)
	}
	lap.Expect(t, "enter 1 24")
}
