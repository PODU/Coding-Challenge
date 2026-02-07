package validator

import "fmt"

// validateRef follows a local $ref ("#" or "#/path/to/schema"). Remote refs
// aren't supported and are reported as errors.
func (v *Validator) validateRef(ref string, instance interface{}, ip, sp Pointer, depth int, c *ErrorCollector) {
	if len(ref) == 0 || ref[0] != '#' {
		if v.opts.Strict {
			v.warnings = append(v.warnings,
				fmt.Sprintf("unsupported non-local $ref %q at %s", ref, ip.Human()))
		}
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.String(),
			Keyword:      "$ref",
			Message:      fmt.Sprintf("unsupported non-local $ref %q (remote refs are out of scope)", ref),
		})
		return
	}

	target, ok := v.resolveRef(ref)
	if !ok {
		if v.opts.Strict {
			v.warnings = append(v.warnings,
				fmt.Sprintf("unresolved $ref %q at %s", ref, ip.Human()))
		}
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.String(),
			Keyword:      "$ref",
			Message:      fmt.Sprintf("cannot resolve $ref %q", ref),
		})
		return
	}

	// Seeing the same ref at the same instance location again means we're
	// looping without consuming any input, so treat it as satisfied.
	key := ref + "@" + ip.String()
	if v.refVisited[key] {
		return
	}
	v.refVisited[key] = true
	defer delete(v.refVisited, key)

	refSP, _ := parsePointer(ref[1:])
	v.validate(target, instance, ip, Pointer(refSP), depth+1, c)
}

// resolveRef looks up a "#/..." ref in the root schema.
func (v *Validator) resolveRef(ref string) (interface{}, bool) {
	tokens, ok := parsePointer(ref[1:])
	if !ok {
		return nil, false
	}

	cur := v.root
	for _, tok := range tokens {
		switch node := cur.(type) {
		case map[string]interface{}:
			next, present := node[tok]
			if !present {
				return nil, false
			}
			cur = next
		case []interface{}:
			idx, err := pointerIndex(tok, len(node))
			if err != nil {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func pointerIndex(tok string, length int) (int, error) {
	if tok == "" {
		return 0, fmt.Errorf("empty index")
	}
	idx := 0
	for _, r := range tok {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-numeric index")
		}
		idx = idx*10 + int(r-'0')
	}
	if idx >= length {
		return 0, fmt.Errorf("index out of range")
	}
	return idx, nil
}
