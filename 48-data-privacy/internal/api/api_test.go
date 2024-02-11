package api_test

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"datavault/internal/api"
	"datavault/internal/crypto"
	"datavault/internal/store"
	"datavault/internal/vault"
)

const (
	tokenizeKey   = "key-tokenize-only"
	detokenizeKey = "key-detokenize-only"
	bothKey       = "key-both"
)

func newTestServer(t *testing.T) *api.Server {
	t.Helper()
	master := make([]byte, crypto.KeySize)
	rand.Read(master)
	kr, err := crypto.NewKeyringSingle(master)
	if err != nil {
		t.Fatal(err)
	}
	dedup := make([]byte, 32)
	rand.Read(dedup)
	v := vault.New(store.NewMemStore(), kr, dedup)

	keysJSON := `[
		{"name":"tok","key":"` + tokenizeKey + `","scopes":["tokenize"]},
		{"name":"detok","key":"` + detokenizeKey + `","scopes":["detokenize"]},
		{"name":"both","key":"` + bothKey + `","scopes":["tokenize","detokenize"]}
	]`
	ks, err := api.LoadKeyStore(keysJSON, "")
	if err != nil {
		t.Fatal(err)
	}
	return api.NewServer(v, ks, nil)
}

func do(t *testing.T, srv *httptest.Server, method, path, key, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, data
}

func TestTokenizeDetokenizeHTTP(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t).Handler())
	defer ts.Close()

	reqBody := `{"id":"customer-42","data":{"ssn":"123-45-6789","email":"alice@example.com"}}`
	resp, data := do(t, ts, http.MethodPost, "/tokenize", bothKey, reqBody)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("tokenize status = %d, want 201; body=%s", resp.StatusCode, data)
	}
	if bytes.Contains(data, []byte("123-45-6789")) || bytes.Contains(data, []byte("alice@example.com")) {
		t.Fatalf("tokenize response leaked plaintext: %s", data)
	}

	var tokResp struct {
		ID   string            `json:"id"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(data, &tokResp); err != nil {
		t.Fatal(err)
	}
	if tokResp.ID != "customer-42" || len(tokResp.Data) != 2 {
		t.Fatalf("unexpected tokenize response: %+v", tokResp)
	}

	// Detokenize with a known token plus an unknown one.
	detReq := `{"tokens":["` + tokResp.Data["ssn"] + `","tok_does_not_exist"]}`
	resp, data = do(t, ts, http.MethodPost, "/detokenize", detokenizeKey, detReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detokenize status = %d, want 200; body=%s", resp.StatusCode, data)
	}

	var detResp struct {
		Results map[string]struct {
			Found bool   `json:"found"`
			Value string `json:"value"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &detResp); err != nil {
		t.Fatal(err)
	}
	if r := detResp.Results[tokResp.Data["ssn"]]; !r.Found || r.Value != "123-45-6789" {
		t.Fatalf("known token result wrong: %+v", r)
	}
	if detResp.Results["tok_does_not_exist"].Found {
		t.Fatal("unknown token should be found=false")
	}
}

func TestAuthMatrix(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t).Handler())
	defer ts.Close()
	body := `{"id":"x","data":{"a":"b"}}`

	cases := []struct {
		name string
		path string
		key  string
		want int
	}{
		{"tokenize no key -> 401", "/tokenize", "", http.StatusUnauthorized},
		{"tokenize bad key -> 401", "/tokenize", "wrong", http.StatusUnauthorized},
		{"tokenize wrong scope -> 403", "/tokenize", detokenizeKey, http.StatusForbidden},
		{"tokenize right scope -> 201", "/tokenize", tokenizeKey, http.StatusCreated},
		{"detokenize wrong scope -> 403", "/detokenize", tokenizeKey, http.StatusForbidden},
		{"detokenize no key -> 401", "/detokenize", "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reqBody := body
			if c.path == "/detokenize" {
				reqBody = `{"tokens":["tok_x"]}`
			}
			resp, data := do(t, ts, http.MethodPost, c.path, c.key, reqBody)
			if resp.StatusCode != c.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode, c.want, data)
			}
		})
	}
}

func TestBadRequests(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t).Handler())
	defer ts.Close()

	t.Run("malformed JSON -> 400", func(t *testing.T) {
		resp, _ := do(t, ts, http.MethodPost, "/tokenize", tokenizeKey, `{not json`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("missing data -> 400", func(t *testing.T) {
		resp, _ := do(t, ts, http.MethodPost, "/tokenize", tokenizeKey, `{"id":"x","data":{}}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("wrong content-type -> 415", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/tokenize", strings.NewReader(`{"id":"x","data":{"a":"b"}}`))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Authorization", "Bearer "+tokenizeKey)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want 415", resp.StatusCode)
		}
	})

	t.Run("oversized body -> 400", func(t *testing.T) {
		big := strings.Repeat("a", (1<<20)+100)
		payload := `{"id":"x","data":{"a":"` + big + `"}}`
		resp, _ := do(t, ts, http.MethodPost, "/tokenize", tokenizeKey, payload)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
	})
}

func TestHealthEndpoints(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t).Handler())
	defer ts.Close()
	for _, path := range []string{"/healthz", "/readyz"} {
		resp, _ := do(t, ts, http.MethodGet, path, "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, resp.StatusCode)
		}
	}
}

// TestTLSSmoke runs the handler over TLS (self-signed via httptest) and confirms
// a tokenize round-trip works and HSTS is set.
func TestTLSSmoke(t *testing.T) {
	ts := httptest.NewTLSServer(newTestServer(t).Handler())
	defer ts.Close()

	client := ts.Client()
	if tr, ok := client.Transport.(*http.Transport); ok {
		tr.TLSClientConfig.MinVersion = tls.VersionTLS12
	}

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/tokenize", strings.NewReader(`{"id":"x","data":{"a":"secret"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenizeKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("TLS tokenize status = %d, want 201", resp.StatusCode)
	}
	if resp.Header.Get("Strict-Transport-Security") == "" {
		t.Fatal("expected HSTS header over TLS")
	}
}
