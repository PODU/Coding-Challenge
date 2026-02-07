package validator

// validateCombinators handles allOf, anyOf, oneOf and not.
func (v *Validator) validateCombinators(s map[string]interface{}, instance interface{}, ip, sp Pointer, depth int, c *ErrorCollector) {
	// allOf reports the branch errors themselves so the user sees what failed.
	if allOf, ok := asSlice(s["allOf"]); ok {
		for i, sub := range allOf {
			v.validate(sub, instance, ip, sp.push("allOf").pushIndex(i), depth+1, c)
		}
	}

	// anyOf and oneOf only report a summary; branch errors would mostly be noise.
	if anyOf, ok := asSlice(s["anyOf"]); ok {
		matched := false
		for i, sub := range anyOf {
			if !v.subValidate(sub, instance, ip, sp.push("anyOf").pushIndex(i), depth+1).hasErrors() {
				matched = true
				break
			}
		}
		if !matched {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("anyOf").String(),
				Keyword:      "anyOf",
				Message:      "value does not match any of the anyOf subschemas",
			})
		}
	}

	if oneOf, ok := asSlice(s["oneOf"]); ok {
		matches := 0
		for i, sub := range oneOf {
			if !v.subValidate(sub, instance, ip, sp.push("oneOf").pushIndex(i), depth+1).hasErrors() {
				matches++
			}
		}
		if matches == 0 {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("oneOf").String(),
				Keyword:      "oneOf",
				Message:      "value does not match any of the oneOf subschemas (expected exactly one)",
			})
		} else if matches > 1 {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("oneOf").String(),
				Keyword:      "oneOf",
				Message:      "value matches more than one of the oneOf subschemas (expected exactly one)",
			})
		}
	}

	if notRaw, present := s["not"]; present {
		if !v.subValidate(notRaw, instance, ip, sp.push("not"), depth+1).hasErrors() {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("not").String(),
				Keyword:      "not",
				Message:      "value must not match the 'not' subschema",
			})
		}
	}
}

// validateConditional handles if/then/else. Errors from "if" itself are never
// reported; it only picks which branch applies.
func (v *Validator) validateConditional(s map[string]interface{}, instance interface{}, ip, sp Pointer, depth int, c *ErrorCollector) {
	ifRaw, present := s["if"]
	if !present {
		return
	}

	branch := "else"
	if !v.subValidate(ifRaw, instance, ip, sp.push("if"), depth+1).hasErrors() {
		branch = "then"
	}
	if sub, ok := s[branch]; ok {
		v.validate(sub, instance, ip, sp.push(branch), depth+1, c)
	}
}
