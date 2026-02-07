package validator

import "fmt"

// validateArrayKeywords handles items, additionalItems, minItems, maxItems,
// uniqueItems and contains.
func (v *Validator) validateArrayKeywords(s map[string]interface{}, instance interface{}, ip, sp Pointer, depth int, c *ErrorCollector) {
	arr, ok := asSlice(instance)
	if !ok {
		return
	}

	if n, ok := toInt(s["minItems"]); ok && len(arr) < n {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("minItems").String(),
			Keyword:      "minItems",
			Message:      fmt.Sprintf("array has %d items, fewer than minimum %d", len(arr), n),
		})
	}
	if n, ok := toInt(s["maxItems"]); ok && len(arr) > n {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("maxItems").String(),
			Keyword:      "maxItems",
			Message:      fmt.Sprintf("array has %d items, more than maximum %d", len(arr), n),
		})
	}

	if u, ok := s["uniqueItems"].(bool); ok && u {
		if i, j, dup := firstDuplicate(arr); dup {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("uniqueItems").String(),
				Keyword:      "uniqueItems",
				Message:      fmt.Sprintf("array items at indices %d and %d are not unique", i, j),
			})
		}
	}

	if itemsRaw, present := s["items"]; present {
		if tuple, isTuple := asSlice(itemsRaw); isTuple {
			// Tuple form: one schema per position.
			for i, val := range arr {
				if i < len(tuple) {
					v.validate(tuple[i], val, ip.pushIndex(i), sp.push("items").pushIndex(i), depth+1, c)
				} else if aiRaw, ok := s["additionalItems"]; ok {
					switch ai := aiRaw.(type) {
					case bool:
						if !ai {
							c.add(ValidationError{
								InstancePath: ip.pushIndex(i).String(),
								SchemaPath:   sp.push("additionalItems").String(),
								Keyword:      "additionalItems",
								Message:      fmt.Sprintf("array item at index %d is not allowed (additionalItems: false)", i),
							})
						}
					default:
						v.validate(aiRaw, val, ip.pushIndex(i), sp.push("additionalItems"), depth+1, c)
					}
				}
			}
		} else {
			for i, val := range arr {
				v.validate(itemsRaw, val, ip.pushIndex(i), sp.push("items"), depth+1, c)
			}
		}
	}

	if containsRaw, present := s["contains"]; present {
		matched := false
		for _, val := range arr {
			if !v.subValidate(containsRaw, val, ip, sp.push("contains"), depth+1).hasErrors() {
				matched = true
				break
			}
		}
		if !matched {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("contains").String(),
				Keyword:      "contains",
				Message:      "no array item matches the contains schema",
			})
		}
	}
}

func firstDuplicate(arr []interface{}) (int, int, bool) {
	for i := 0; i < len(arr); i++ {
		for j := i + 1; j < len(arr); j++ {
			if deepEqual(arr[i], arr[j]) {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}
