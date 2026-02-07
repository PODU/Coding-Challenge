package validator

import (
	"fmt"
	"unicode/utf8"
)

// validateStringKeywords handles minLength, maxLength, pattern and format.
func (v *Validator) validateStringKeywords(s map[string]interface{}, instance interface{}, ip, sp Pointer, c *ErrorCollector) {
	str, ok := instance.(string)
	if !ok {
		return
	}

	// JSON Schema counts code points, not bytes.
	length := utf8.RuneCountInString(str)

	if n, ok := toInt(s["minLength"]); ok && length < n {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("minLength").String(),
			Keyword:      "minLength",
			Message:      fmt.Sprintf("string length %d is below minimum %d", length, n),
		})
	}

	if n, ok := toInt(s["maxLength"]); ok && length > n {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("maxLength").String(),
			Keyword:      "maxLength",
			Message:      fmt.Sprintf("string length %d exceeds maximum %d", length, n),
		})
	}

	if pat, ok := s["pattern"].(string); ok {
		re, err := compilePattern(pat)
		if err != nil {
			if v.opts.Strict {
				v.warnings = append(v.warnings,
					fmt.Sprintf("invalid pattern %q at %s: %v", pat, sp.Human(), err))
			}
		} else if !re.MatchString(str) {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.push("pattern").String(),
				Keyword:      "pattern",
				Message:      fmt.Sprintf("string does not match pattern %q", pat),
			})
		}
	}

	if format, ok := s["format"].(string); ok && v.opts.AssertFormat && !checkFormat(format, str) {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.push("format").String(),
			Keyword:      "format",
			Message:      fmt.Sprintf("string is not a valid %s", format),
		})
	}
}

// toInt reads an integer keyword value such as minLength. A non-integer
// value is treated as if the keyword were absent.
func toInt(v interface{}) (int, bool) {
	r, ok := toRat(v)
	if !ok || !r.IsInt() || !r.Num().IsInt64() {
		return 0, false
	}
	return int(r.Num().Int64()), true
}
