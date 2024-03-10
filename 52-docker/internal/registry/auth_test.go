package registry

import "testing"

func TestParseChallenge(t *testing.T) {
	header := `Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/ubuntu:pull"`
	ch, err := ParseChallenge(header)
	if err != nil {
		t.Fatalf("ParseChallenge error: %v", err)
	}
	if ch.Realm != "https://auth.docker.io/token" {
		t.Errorf("realm = %q", ch.Realm)
	}
	if ch.Service != "registry.docker.io" {
		t.Errorf("service = %q", ch.Service)
	}
	if ch.Scope != "repository:library/ubuntu:pull" {
		t.Errorf("scope = %q", ch.Scope)
	}
}

func TestParseChallengeUnquotedAndCaseInsensitive(t *testing.T) {
	ch, err := ParseChallenge(`bearer realm=https://auth.example.com/token,service=reg`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if ch.Realm != "https://auth.example.com/token" || ch.Service != "reg" {
		t.Errorf("got %+v", ch)
	}
}

func TestParseChallengeErrors(t *testing.T) {
	for _, h := range []string{"", "Basic realm=x", `Bearer service="x"`} {
		if _, err := ParseChallenge(h); err == nil {
			t.Errorf("ParseChallenge(%q) want error", h)
		}
	}
}

func TestTokenRequestURL(t *testing.T) {
	ch := Challenge{
		Realm:   "https://auth.docker.io/token",
		Service: "registry.docker.io",
		Scope:   "repository:library/ubuntu:pull",
	}
	got, err := TokenRequestURL(ch)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://auth.docker.io/token?scope=repository%3Alibrary%2Fubuntu%3Apull&service=registry.docker.io"
	if got != want {
		t.Errorf("TokenRequestURL = %q; want %q", got, want)
	}
}

func TestPullScope(t *testing.T) {
	if got := PullScope("library/ubuntu"); got != "repository:library/ubuntu:pull" {
		t.Errorf("PullScope = %q", got)
	}
}
