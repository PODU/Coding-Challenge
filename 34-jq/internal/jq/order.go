package jq

import "strings"

// typeRank gives the jq cross-type ordering:
// null < false < true < numbers < strings < arrays < objects.
func typeRank(v Value) int {
	switch t := v.(type) {
	case nil:
		return 0
	case bool:
		if !t {
			return 1
		}
		return 2
	case float64:
		return 3
	case string:
		return 4
	case []Value:
		return 5
	case *Object:
		return 6
	default:
		return 7
	}
}

// Compare implements the canonical jq total ordering over JSON values.
// It returns -1, 0 or 1.
func Compare(a, b Value) int {
	ra, rb := typeRank(a), typeRank(b)
	if ra != rb {
		if ra < rb {
			return -1
		}
		return 1
	}
	switch av := a.(type) {
	case nil:
		return 0
	case bool:
		// Same bool rank already distinguishes false/true.
		return 0
	case float64:
		bv := b.(float64)
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		default:
			return 0
		}
	case string:
		return strings.Compare(av, b.(string))
	case []Value:
		bv := b.([]Value)
		for i := 0; i < len(av) && i < len(bv); i++ {
			if c := Compare(av[i], bv[i]); c != 0 {
				return c
			}
		}
		return intCmp(len(av), len(bv))
	case *Object:
		bv := b.(*Object)
		// jq compares objects by sorted keys first, then by values in that order.
		ak := sortedKeys(av)
		bk := sortedKeys(bv)
		if c := compareStringSlices(ak, bk); c != 0 {
			return c
		}
		for _, k := range ak {
			va, _ := av.Get(k)
			vb, _ := bv.Get(k)
			if c := Compare(va, vb); c != 0 {
				return c
			}
		}
		return 0
	}
	return 0
}

// Equal reports deep equality using the total order.
func Equal(a, b Value) bool { return Compare(a, b) == 0 }

func intCmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func sortedKeys(o *Object) []string {
	ks := make([]string, len(o.keys))
	copy(ks, o.keys)
	// simple insertion sort keeps allocation low; objects are usually small
	for i := 1; i < len(ks); i++ {
		for j := i; j > 0 && ks[j-1] > ks[j]; j-- {
			ks[j-1], ks[j] = ks[j], ks[j-1]
		}
	}
	return ks
}

func compareStringSlices(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := strings.Compare(a[i], b[i]); c != 0 {
			return c
		}
	}
	return intCmp(len(a), len(b))
}
