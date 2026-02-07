package validator

// deepEqual compares two decoded JSON values the way enum, const and
// uniqueItems need: numbers by value (1, 1.0 and 10e-1 are equal) and objects
// regardless of key order.
func deepEqual(a, b interface{}) bool {
	if ra, okA := toRat(a); okA {
		rb, okB := toRat(b)
		if !okB {
			return false
		}
		return ra.Cmp(rb) == 0
	}
	if isNumber(b) {
		return false
	}

	switch av := a.(type) {
	case nil:
		return b == nil
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, va := range av {
			vb, present := bv[k]
			if !present || !deepEqual(va, vb) {
				return false
			}
		}
		return true
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	}
	return false
}
