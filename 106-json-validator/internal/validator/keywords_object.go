package validator

import "fmt"

// validateObjectKeywords handles properties, required, patternProperties,
// additionalProperties, minProperties, maxProperties and propertyNames.
func (v *Validator) validateObjectKeywords(s map[string]interface{}, instance interface{}, ip, sp Pointer, depth int, c *ErrorCollector) {
	obj, ok := asMap(instance)
	if !ok {
		return
	}

	if req, ok := asSlice(s["required"]); ok {
		for _, item := range req {
			name, ok := item.(string)
			if !ok {
				continue
			}
			if _, exists := obj[name]; !exists {
				c.add(ValidationError{
					InstancePath: ip.String(),
					SchemaPath:   sp.push("required").String(),
					Keyword:      "required",
					Message:      fmt.Sprintf("missing required property %q", name),
				})
			}
		}
	}

	if n, ok := toInt(s["minProperties"]); ok && len(obj) < n {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("minProperties").String(),
			Keyword:      "minProperties",
			Message:      fmt.Sprintf("object has %d properties, fewer than minimum %d", len(obj), n),
		})
	}
	if n, ok := toInt(s["maxProperties"]); ok && len(obj) > n {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("maxProperties").String(),
			Keyword:      "maxProperties",
			Message:      fmt.Sprintf("object has %d properties, more than maximum %d", len(obj), n),
		})
	}

	if pnRaw, present := s["propertyNames"]; present {
		for key := range obj {
			if v.subValidate(pnRaw, key, ip.push(key), sp.push("propertyNames"), depth+1).hasErrors() {
				c.add(ValidationError{
					InstancePath: ip.push(key).String(),
					SchemaPath:   sp.push("propertyNames").String(),
					Keyword:      "propertyNames",
					Message:      fmt.Sprintf("property name %q does not satisfy propertyNames schema", key),
				})
			}
		}
	}

	properties, _ := asMap(s["properties"])
	patternProps, _ := asMap(s["patternProperties"])

	for name, subschema := range properties {
		if val, exists := obj[name]; exists {
			v.validate(subschema, val, ip.push(name), sp.push("properties").push(name), depth+1, c)
		}
	}

	for pat, subschema := range patternProps {
		re, err := compilePattern(pat)
		if err != nil {
			if v.opts.Strict {
				v.warnings = append(v.warnings,
					fmt.Sprintf("invalid patternProperties regex %q at %s: %v", pat, sp.Human(), err))
			}
			continue
		}
		for name, val := range obj {
			if re.MatchString(name) {
				v.validate(subschema, val, ip.push(name), sp.push("patternProperties").push(pat), depth+1, c)
			}
		}
	}

	// additionalProperties covers keys matched by neither properties nor
	// patternProperties.
	apRaw, present := s["additionalProperties"]
	if !present {
		return
	}
	for name, val := range obj {
		if _, inProps := properties[name]; inProps || matchesAnyPattern(name, patternProps) {
			continue
		}
		switch ap := apRaw.(type) {
		case bool:
			if !ap {
				c.add(ValidationError{
					InstancePath: ip.push(name).String(),
					SchemaPath:   sp.push("additionalProperties").String(),
					Keyword:      "additionalProperties",
					Message:      fmt.Sprintf("additional property %q is not allowed", name),
				})
			}
		default:
			v.validate(apRaw, val, ip.push(name), sp.push("additionalProperties"), depth+1, c)
		}
	}
}

func matchesAnyPattern(name string, patternProps map[string]interface{}) bool {
	for pat := range patternProps {
		if re, err := compilePattern(pat); err == nil && re.MatchString(name) {
			return true
		}
	}
	return false
}
