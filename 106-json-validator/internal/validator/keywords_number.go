package validator

import (
	"fmt"
	"math/big"
	"strings"
)

// validateNumberKeywords handles minimum, maximum, exclusiveMinimum,
// exclusiveMaximum and multipleOf. Everything is done with big.Rat so that
// e.g. 0.3 is a multiple of 0.1 and huge literals keep their precision.
func (v *Validator) validateNumberKeywords(s map[string]interface{}, instance interface{}, ip, sp Pointer, c *ErrorCollector) {
	value, ok := toRat(instance)
	if !ok {
		return
	}

	if bound, ok := toRat(s["minimum"]); ok && value.Cmp(bound) < 0 {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("minimum").String(),
			Keyword:      "minimum",
			Message:      fmt.Sprintf("value must be >= %s", ratString(bound)),
		})
	}

	if bound, ok := toRat(s["maximum"]); ok && value.Cmp(bound) > 0 {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("maximum").String(),
			Keyword:      "maximum",
			Message:      fmt.Sprintf("value must be <= %s", ratString(bound)),
		})
	}

	if bound, ok := toRat(s["exclusiveMinimum"]); ok && value.Cmp(bound) <= 0 {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("exclusiveMinimum").String(),
			Keyword:      "exclusiveMinimum",
			Message:      fmt.Sprintf("value must be > %s", ratString(bound)),
		})
	}

	if bound, ok := toRat(s["exclusiveMaximum"]); ok && value.Cmp(bound) >= 0 {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("exclusiveMaximum").String(),
			Keyword:      "exclusiveMaximum",
			Message:      fmt.Sprintf("value must be < %s", ratString(bound)),
		})
	}

	if divisor, ok := toRat(s["multipleOf"]); ok {
		if divisor.Sign() <= 0 {
			return // invalid schema; ignore it
		}
		if !new(big.Rat).Quo(value, divisor).IsInt() {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("multipleOf").String(),
				Keyword:      "multipleOf",
				Message:      fmt.Sprintf("value must be a multiple of %s", ratString(divisor)),
			})
		}
	}
}

// ratString formats r for error messages, with at most 10 decimal places.
func ratString(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	return strings.TrimSuffix(strings.TrimRight(r.FloatString(10), "0"), ".")
}
