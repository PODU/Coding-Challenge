package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gg/internal/object"
)

func TestPackRoundTrip(t *testing.T) {
	objs := []PackObject{
		{Type: object.TypeBlob, Payload: []byte("hello\n")},
		{Type: object.TypeBlob, Payload: bytes.Repeat([]byte("x"), 1000)}, // multi-byte varint size
		{Type: object.TypeTree, Payload: []byte{}},
		{Type: object.TypeCommit, Payload: []byte("tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n")},
	}
	pack, err := BuildPack(objs)
	if err != nil {
		t.Fatal(err)
	}
	if string(pack[:4]) != "PACK" {
		t.Fatalf("bad signature %q", pack[:4])
	}

	parsed, err := ParsePack(pack)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != len(objs) {
		t.Fatalf("parsed %d objects, want %d", len(parsed), len(objs))
	}
	for i := range objs {
		if parsed[i].Type != objs[i].Type {
			t.Fatalf("object %d type = %s, want %s", i, parsed[i].Type, objs[i].Type)
		}
		if !bytes.Equal(parsed[i].Payload, objs[i].Payload) {
			t.Fatalf("object %d payload mismatch", i)
		}
	}
}

func TestPackCorruptTrailerRejected(t *testing.T) {
	pack, err := BuildPack([]PackObject{{Type: object.TypeBlob, Payload: []byte("a")}})
	if err != nil {
		t.Fatal(err)
	}
	pack[len(pack)-1] ^= 0xFF
	if _, err := ParsePack(pack); err == nil {
		t.Fatal("expected checksum error")
	}
}

func pkt(s string) string { return fmt.Sprintf("%04x%s", len(s)+4, s) }

// stubServer speaks just enough smart-HTTP to validate a push.
func stubServer(t *testing.T, advertisedRefs []RemoteRef) (*httptest.Server, *[]PackObject) {
	t.Helper()
	var received []PackObject

	mux := http.NewServeMux()
	mux.HandleFunc("/info/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("service") != "git-receive-pack" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
		var b strings.Builder
		b.WriteString(pkt("# service=git-receive-pack\n"))
		b.WriteString("0000")
		if len(advertisedRefs) == 0 {
			b.WriteString(pkt(ZeroOID + " capabilities^{}\x00report-status\n"))
		} else {
			for i, ref := range advertisedRefs {
				line := ref.OID + " " + ref.Name
				if i == 0 {
					line += "\x00report-status"
				}
				line += "\n"
				b.WriteString(pkt(line))
			}
		}
		b.WriteString("0000")
		io.WriteString(w, b.String())
	})
	mux.HandleFunc("/git-receive-pack", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Find the packfile, which begins at the "PACK" signature.
		idx := bytes.Index(body, []byte("PACK"))
		if idx < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		objs, err := ParsePack(body[idx:])
		if err != nil {
			http.Error(w, "bad pack: "+err.Error(), http.StatusBadRequest)
			return
		}
		received = objs

		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
		var b strings.Builder
		b.WriteString(pkt("unpack ok\n"))
		b.WriteString(pkt("ok refs/heads/main\n"))
		b.WriteString("0000")
		io.WriteString(w, b.String())
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &received
}

func TestDiscoverEmptyRepo(t *testing.T) {
	srv, _ := stubServer(t, nil)
	refs, err := DiscoverReceivePack(srv.Client(), srv.URL, Auth{})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs for empty repo, got %v", refs)
	}
}

func TestDiscoverWithRefs(t *testing.T) {
	want := []RemoteRef{{OID: "1111111111111111111111111111111111111111", Name: "refs/heads/main"}}
	srv, _ := stubServer(t, want)
	refs, err := DiscoverReceivePack(srv.Client(), srv.URL, Auth{})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Name != "refs/heads/main" || refs[0].OID != want[0].OID {
		t.Fatalf("got %v", refs)
	}
}

func TestReceivePackUploadsPack(t *testing.T) {
	srv, received := stubServer(t, nil)

	pack, err := BuildPack([]PackObject{
		{Type: object.TypeBlob, Payload: []byte("hello\n")},
		{Type: object.TypeCommit, Payload: []byte("tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	updates := []RefUpdate{{Old: ZeroOID, New: "2222222222222222222222222222222222222222", Ref: "refs/heads/main"}}
	report, err := ReceivePack(srv.Client(), srv.URL, Auth{}, updates, pack)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "unpack ok") {
		t.Fatalf("report missing unpack ok: %q", report)
	}
	if len(*received) != 2 {
		t.Fatalf("server received %d objects, want 2", len(*received))
	}
}

func TestReceivePackParsesPktLineCommand(t *testing.T) {
	// Verify the request body begins with a valid pkt-line ref-update command.
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/git-receive-pack", func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
		io.WriteString(w, pkt("unpack ok\n")+pkt("ok refs/heads/main\n")+"0000")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	pack, _ := BuildPack([]PackObject{{Type: object.TypeBlob, Payload: []byte("z")}})
	_, err := ReceivePack(srv.Client(), srv.URL, Auth{},
		[]RefUpdate{{Old: ZeroOID, New: "3333333333333333333333333333333333333333", Ref: "refs/heads/main"}}, pack)
	if err != nil {
		t.Fatal(err)
	}

	br := bufio.NewReader(bytes.NewReader(gotBody))
	var lenBuf [4]byte
	if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
		t.Fatal(err)
	}
	var length int
	fmt.Sscanf(string(lenBuf[:]), "%04x", &length)
	payload := make([]byte, length-4)
	io.ReadFull(br, payload)
	cmd := string(payload)
	if !strings.HasPrefix(cmd, ZeroOID+" 3333333333333333333333333333333333333333 refs/heads/main") {
		t.Fatalf("unexpected command line: %q", cmd)
	}
	if !strings.Contains(cmd, "report-status") {
		t.Fatalf("command missing capability: %q", cmd)
	}
}
