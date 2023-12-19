package urlparse

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		scheme    string
		host      string
		port      string
		path      string
		query     string
		hostHdr   string
		dialAddr  string
		reqTarget string
	}{
		{
			name:   "http default port no path",
			raw:    "http://example.com",
			scheme: "http", host: "example.com", port: "80", path: "/",
			hostHdr: "example.com", dialAddr: "example.com:80", reqTarget: "/",
		},
		{
			name:   "https default port with path",
			raw:    "https://example.com/index.html",
			scheme: "https", host: "example.com", port: "443", path: "/index.html",
			hostHdr: "example.com", dialAddr: "example.com:443", reqTarget: "/index.html",
		},
		{
			name:   "explicit non-default port shows in host header",
			raw:    "http://example.com:8080/p",
			scheme: "http", host: "example.com", port: "8080", path: "/p",
			hostHdr: "example.com:8080", dialAddr: "example.com:8080", reqTarget: "/p",
		},
		{
			name:   "query string preserved",
			raw:    "http://h/search?q=go&x=1",
			scheme: "http", host: "h", port: "80", path: "/search", query: "q=go&x=1",
			hostHdr: "h", dialAddr: "h:80", reqTarget: "/search?q=go&x=1",
		},
		{
			name:   "fragment stripped",
			raw:    "http://h/p#frag",
			scheme: "http", host: "h", port: "80", path: "/p",
			hostHdr: "h", dialAddr: "h:80", reqTarget: "/p",
		},
		{
			name:   "ipv6 literal default port",
			raw:    "http://[::1]/p",
			scheme: "http", host: "::1", port: "80", path: "/p",
			hostHdr: "[::1]", dialAddr: "[::1]:80", reqTarget: "/p",
		},
		{
			name:   "ipv6 literal explicit port",
			raw:    "http://[::1]:8080/",
			scheme: "http", host: "::1", port: "8080", path: "/",
			hostHdr: "[::1]:8080", dialAddr: "[::1]:8080", reqTarget: "/",
		},
		{
			name:   "uppercase scheme normalized",
			raw:    "HTTP://Example.com/",
			scheme: "http", host: "Example.com", port: "80", path: "/",
			hostHdr: "Example.com", dialAddr: "Example.com:80", reqTarget: "/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := ParseURL(tc.raw)
			if err != nil {
				t.Fatalf("ParseURL(%q) error: %v", tc.raw, err)
			}
			if u.Scheme != tc.scheme {
				t.Errorf("scheme = %q, want %q", u.Scheme, tc.scheme)
			}
			if u.Host != tc.host {
				t.Errorf("host = %q, want %q", u.Host, tc.host)
			}
			if u.Port != tc.port {
				t.Errorf("port = %q, want %q", u.Port, tc.port)
			}
			if u.Path != tc.path {
				t.Errorf("path = %q, want %q", u.Path, tc.path)
			}
			if u.Query != tc.query {
				t.Errorf("query = %q, want %q", u.Query, tc.query)
			}
			if got := u.HostHeader(); got != tc.hostHdr {
				t.Errorf("HostHeader() = %q, want %q", got, tc.hostHdr)
			}
			if got := u.DialAddress(); got != tc.dialAddr {
				t.Errorf("DialAddress() = %q, want %q", got, tc.dialAddr)
			}
			if got := u.RequestTarget(); got != tc.reqTarget {
				t.Errorf("RequestTarget() = %q, want %q", got, tc.reqTarget)
			}
		})
	}
}

func TestParseURLErrors(t *testing.T) {
	tests := []struct {
		raw  string
		want error
	}{
		{"", ErrEmptyURL},
		{"example.com/path", ErrNoScheme},
		{"ftp://example.com", ErrBadScheme},
		{"http://", ErrNoHost},
		{"http://h:notaport/", ErrBadPort},
		{"http://h:99999/", ErrBadPort},
		{"http://[::1/", ErrUnclosedIPv6},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			_, err := ParseURL(tc.raw)
			if err != tc.want {
				t.Errorf("ParseURL(%q) err = %v, want %v", tc.raw, err, tc.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	base, _ := ParseURL("http://example.com/a/b/c?x=1")
	tests := []struct {
		ref  string
		want string
	}{
		{"https://other.com/p", "https://other.com/p"},
		{"//other.com/p", "http://other.com/p"},
		{"/new", "http://example.com/new"},
		{"page.html", "http://example.com/a/b/page.html"},
		{"../up", "http://example.com/a/up"},
		{"?y=2", "http://example.com/a/b/c?y=2"},
		{"sub/deep", "http://example.com/a/b/sub/deep"},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			got, err := Resolve(base, tc.ref)
			if err != nil {
				t.Fatalf("Resolve(%q) error: %v", tc.ref, err)
			}
			if got.String() != tc.want {
				t.Errorf("Resolve(%q) = %q, want %q", tc.ref, got.String(), tc.want)
			}
		})
	}
}
