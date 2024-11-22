package template

import "testing"

func TestRender(t *testing.T) {
	vars := map[string]string{"Title": "Hello", "Content": "<p>Body</p>"}
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"both tags", "<h1>{{ Title }}</h1>{{ Content }}", "<h1>Hello</h1><p>Body</p>"},
		{"no spaces", "{{Title}}", "Hello"},
		{"extra spaces", "{{   Title   }}", "Hello"},
		{"unknown preserved", "{{ Author }}", "{{ Author }}"},
		{"missing content tolerated", "<h1>{{ Title }}</h1>", "<h1>Hello</h1>"},
		{"repeated", "{{ Title }}-{{ Title }}", "Hello-Hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Render(c.in, vars); got != c.want {
				t.Errorf("Render(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestHasTag(t *testing.T) {
	if !HasTag("a {{ Content }} b", "Content") {
		t.Error("expected HasTag to find Content")
	}
	if HasTag("a {{ Title }} b", "Content") {
		t.Error("did not expect Content tag")
	}
	if !HasTag("{{Content}}", "Content") {
		t.Error("expected to match whitespace-free tag")
	}
}
