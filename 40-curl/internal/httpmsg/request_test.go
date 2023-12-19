package httpmsg

import (
	"testing"

	"github.com/codingchallenges/cc-curl/internal/urlparse"
)

func mustURL(t *testing.T, raw string) urlparse.URL {
	t.Helper()
	u, err := urlparse.ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL(%q): %v", raw, err)
	}
	return u
}

func TestBuildRequestGET(t *testing.T) {
	req := Request{
		Method: "GET",
		URL:    mustURL(t, "http://example.com/index.html"),
	}
	got := string(BuildRequest(req))
	want := "GET /index.html HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Accept: */*\r\n" +
		"Connection: close\r\n" +
		"\r\n"
	if got != want {
		t.Errorf("GET request mismatch\n got = %q\nwant = %q", got, want)
	}
}

func TestBuildRequestGETWithQueryAndPort(t *testing.T) {
	req := Request{
		URL: mustURL(t, "http://example.com:8080/search?q=go"),
	}
	got := string(BuildRequest(req))
	want := "GET /search?q=go HTTP/1.1\r\n" +
		"Host: example.com:8080\r\n" +
		"Accept: */*\r\n" +
		"Connection: close\r\n" +
		"\r\n"
	if got != want {
		t.Errorf("mismatch\n got = %q\nwant = %q", got, want)
	}
}

func TestBuildRequestPOSTBody(t *testing.T) {
	req := Request{
		Method: "POST",
		URL:    mustURL(t, "http://example.com/post"),
		Body:   []byte("name=fred&project=curl"),
	}
	got := string(BuildRequest(req))
	want := "POST /post HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Accept: */*\r\n" +
		"Connection: close\r\n" +
		"Content-Length: 22\r\n" +
		"Content-Type: application/x-www-form-urlencoded\r\n" +
		"\r\n" +
		"name=fred&project=curl"
	if got != want {
		t.Errorf("POST request mismatch\n got = %q\nwant = %q", got, want)
	}
}

func TestBuildRequestUserContentTypeOverride(t *testing.T) {
	req := Request{
		Method:  "POST",
		URL:     mustURL(t, "http://example.com/post"),
		Headers: []Header{{Name: "Content-Type", Value: "application/json"}},
		Body:    []byte(`{"k":"v"}`),
	}
	got := string(BuildRequest(req))
	want := "POST /post HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Accept: */*\r\n" +
		"Connection: close\r\n" +
		"Content-Length: 9\r\n" +
		"Content-Type: application/json\r\n" +
		"\r\n" +
		`{"k":"v"}`
	if got != want {
		t.Errorf("mismatch\n got = %q\nwant = %q", got, want)
	}
}

func TestBuildRequestHeaderOverrideCaseInsensitive(t *testing.T) {
	req := Request{
		URL: mustURL(t, "http://example.com/"),
		Headers: []Header{
			{Name: "accept", Value: "application/json"},
			{Name: "X-Custom", Value: "yes"},
		},
	}
	got := string(BuildRequest(req))
	want := "GET / HTTP/1.1\r\n" +
		"Host: example.com\r\n" +
		"Connection: close\r\n" +
		"accept: application/json\r\n" +
		"X-Custom: yes\r\n" +
		"\r\n"
	if got != want {
		t.Errorf("mismatch\n got = %q\nwant = %q", got, want)
	}
}

func TestBuildRequestHostOverride(t *testing.T) {
	req := Request{
		URL:     mustURL(t, "http://example.com/"),
		Headers: []Header{{Name: "Host", Value: "override.example"}},
	}
	got := string(BuildRequest(req))
	want := "GET / HTTP/1.1\r\n" +
		"Accept: */*\r\n" +
		"Connection: close\r\n" +
		"Host: override.example\r\n" +
		"\r\n"
	if got != want {
		t.Errorf("mismatch\n got = %q\nwant = %q", got, want)
	}
}

func TestParseHeaderArg(t *testing.T) {
	tests := []struct {
		raw   string
		name  string
		value string
	}{
		{"Content-Type: application/json", "Content-Type", "application/json"},
		{"X-Foo:bar", "X-Foo", "bar"},
		{"  Spaced  :  val  ", "Spaced", "val"},
		{"NoColon", "NoColon", ""},
	}
	for _, tc := range tests {
		h := ParseHeaderArg(tc.raw)
		if h.Name != tc.name || h.Value != tc.value {
			t.Errorf("ParseHeaderArg(%q) = {%q,%q}, want {%q,%q}",
				tc.raw, h.Name, h.Value, tc.name, tc.value)
		}
	}
}
