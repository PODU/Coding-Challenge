package validator

// jsonTypeOf returns the JSON type name of v. It never returns "integer";
// matchesType deals with that.
func jsonTypeOf(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	default:
		if isNumber(v) {
			return "number"
		}
		return "unknown"
	}
}

func matchesType(v interface{}, typeName string) bool {
	switch typeName {
	case "integer":
		return isInteger(v)
	case "number":
		return isNumber(v)
	default:
		return jsonTypeOf(v) == typeName
	}
}

func asMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func asSlice(v interface{}) ([]interface{}, bool) {
	s, ok := v.([]interface{})
	return s, ok
}
