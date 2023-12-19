package urlparse

import "strings"

// Resolve resolves a Location header value against the URL that returned it.
// It handles full URLs, "//host/path", "/path", relative paths such as
// "../up", and query-only references.
func Resolve(base URL, ref string) (URL, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return base, nil
	}

	if strings.Contains(ref, "://") {
		return ParseURL(ref)
	}

	if strings.HasPrefix(ref, "//") {
		return ParseURL(base.Scheme + ":" + ref)
	}

	result := base

	// The fragment is never sent.
	if h := strings.IndexByte(ref, '#'); h >= 0 {
		ref = ref[:h]
	}

	path := ref
	query := ""
	if q := strings.IndexByte(ref, '?'); q >= 0 {
		path = ref[:q]
		query = ref[q+1:]
	}

	switch {
	case path == "":
		result.Query = query
		return result, nil
	case strings.HasPrefix(path, "/"):
		result.Path = path
	default:
		dir := base.Path
		if i := strings.LastIndexByte(dir, '/'); i >= 0 {
			dir = dir[:i+1]
		} else {
			dir = "/"
		}
		result.Path = removeDotSegments(dir + path)
	}
	result.Query = query
	return result, nil
}

// removeDotSegments resolves "." and ".." as in RFC 3986.
func removeDotSegments(p string) string {
	segments := strings.Split(p, "/")
	out := make([]string, 0, len(segments))
	for _, seg := range segments {
		switch seg {
		case ".":
		case "..":
			if len(out) > 0 && out[len(out)-1] != "" {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, seg)
		}
	}
	res := strings.Join(out, "/")
	if !strings.HasPrefix(res, "/") {
		res = "/" + res
	}
	return res
}
