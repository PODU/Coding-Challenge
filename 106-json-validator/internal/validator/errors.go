package validator

import "sort"

// ValidationError is one failed schema check.
type ValidationError struct {
	// InstancePath and SchemaPath are RFC 6901 JSON Pointers into the instance
	// and the schema respectively.
	InstancePath string `json:"instancePath"`
	SchemaPath   string `json:"schemaPath"`
	Keyword      string `json:"keyword"`
	Message      string `json:"message"`
}

// ErrorCollector gathers errors from one validation pass.
type ErrorCollector struct {
	errors []ValidationError
}

func (c *ErrorCollector) add(e ValidationError) {
	c.errors = append(c.errors, e)
}

func (c *ErrorCollector) addAll(other *ErrorCollector) {
	c.errors = append(c.errors, other.errors...)
}

func (c *ErrorCollector) hasErrors() bool {
	return len(c.errors) > 0
}

// Errors returns what has been collected so far.
func (c *ErrorCollector) Errors() []ValidationError {
	return c.errors
}

// sortErrors gives errors a stable order so output doesn't depend on map
// iteration (the golden tests rely on this).
func sortErrors(errs []ValidationError) {
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].InstancePath != errs[j].InstancePath {
			return errs[i].InstancePath < errs[j].InstancePath
		}
		if errs[i].Keyword != errs[j].Keyword {
			return errs[i].Keyword < errs[j].Keyword
		}
		if errs[i].SchemaPath != errs[j].SchemaPath {
			return errs[i].SchemaPath < errs[j].SchemaPath
		}
		return errs[i].Message < errs[j].Message
	})
}
