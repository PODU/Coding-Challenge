package validator

import (
	"fmt"
	"strings"
)

// validateType handles "type", which is either a name or a list of names.
func (v *Validator) validateType(s map[string]interface{}, instance interface{}, ip, sp Pointer, c *ErrorCollector) {
	raw, present := s["type"]
	if !present {
		return
	}

	switch t := raw.(type) {
	case string:
		if !matchesType(instance, t) {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("type").String(),
				Keyword:      "type",
				Message:      fmt.Sprintf("expected type %q but got %q", t, jsonTypeOf(instance)),
			})
		}
	case []interface{}:
		names := make([]string, 0, len(t))
		for _, item := range t {
			if name, ok := item.(string); ok {
				names = append(names, name)
				if matchesType(instance, name) {
					return
				}
			}
		}
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("type").String(),
			Keyword:      "type",
			Message: fmt.Sprintf("expected one of types [%s] but got %q",
				strings.Join(names, ", "), jsonTypeOf(instance)),
		})
	}
}

func (v *Validator) validateEnumConst(s map[string]interface{}, instance interface{}, ip, sp Pointer, c *ErrorCollector) {
	if cv, present := s["const"]; present && !deepEqual(instance, cv) {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("const").String(),
			Keyword:      "const",
			Message:      "value does not equal the required const",
		})
	}

	if allowed, ok := asSlice(s["enum"]); ok {
		for _, candidate := range allowed {
			if deepEqual(instance, candidate) {
				return
			}
		}
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("enum").String(),
			Keyword:      "enum",
			Message:      "value is not one of the allowed enum values",
		})
	}
}
