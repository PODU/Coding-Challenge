package validator

import "fmt"

type Options struct {
	// AssertFormat makes "format" fail validation instead of being
	// informational only, which is the Draft 7 default.
	AssertFormat bool
	// Strict adds warnings for unknown keywords and unresolved $refs.
	Strict bool
	// MaxDepth limits recursion; 0 means DefaultMaxDepth.
	MaxDepth int
}

const DefaultMaxDepth = 256

type Result struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []string
}

// Validator checks instances against one root schema.
type Validator struct {
	root     interface{}
	opts     Options
	warnings []string

	// refVisited holds "ref@instancePath" keys currently being expanded, to
	// break $ref cycles.
	refVisited map[string]bool
}

// New returns a Validator for root, which should be a decoded object or bool.
func New(root interface{}, opts Options) *Validator {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = DefaultMaxDepth
	}
	return &Validator{
		root:       root,
		opts:       opts,
		refVisited: make(map[string]bool),
	}
}

// Validate returns every error for instance, in a stable order.
func (v *Validator) Validate(instance interface{}) Result {
	v.warnings = nil
	c := &ErrorCollector{}
	v.validate(v.root, instance, Pointer{}, Pointer{}, 0, c)

	errs := c.Errors()
	sortErrors(errs)

	return Result{
		Valid:    len(errs) == 0,
		Errors:   errs,
		Warnings: v.warnings,
	}
}

// validate applies schema to instance. ip and sp are where we are in the
// instance and the schema.
func (v *Validator) validate(schema, instance interface{}, ip, sp Pointer, depth int, c *ErrorCollector) {
	if depth > v.opts.MaxDepth {
		c.add(ValidationError{
			InstancePath: ip.String(),
			SchemaPath:   sp.String(),
			Keyword:      "$maxDepth",
			Message:      fmt.Sprintf("maximum validation depth %d exceeded", v.opts.MaxDepth),
		})
		return
	}

	switch s := schema.(type) {
	case bool:
		if !s {
			c.add(ValidationError{
				InstancePath: ip.String(),
				SchemaPath:   sp.String(),
				Keyword:      "false",
				Message:      "value is disallowed by a false schema",
			})
		}
		return
	case map[string]interface{}:
		v.validateObjectSchema(s, instance, ip, sp, depth, c)
	default:
		// Not a valid schema. Ignore it rather than fail the whole run.
		if v.opts.Strict {
			v.warnings = append(v.warnings,
				fmt.Sprintf("schema at %s is not an object or boolean; ignored", sp.Human()))
		}
	}
}

func (v *Validator) validateObjectSchema(s map[string]interface{}, instance interface{}, ip, sp Pointer, depth int, c *ErrorCollector) {
	// In Draft 7 a $ref replaces the whole schema; sibling keywords are ignored.
	if ref, ok := s["$ref"].(string); ok {
		v.validateRef(ref, instance, ip, sp.push("$ref"), depth, c)
		return
	}

	v.validateType(s, instance, ip, sp, c)
	v.validateEnumConst(s, instance, ip, sp, c)
	v.validateNumberKeywords(s, instance, ip, sp, c)
	v.validateStringKeywords(s, instance, ip, sp, c)
	v.validateObjectKeywords(s, instance, ip, sp, depth, c)
	v.validateArrayKeywords(s, instance, ip, sp, depth, c)
	v.validateCombinators(s, instance, ip, sp, depth, c)
	v.validateConditional(s, instance, ip, sp, depth, c)

	if v.opts.Strict {
		v.warnUnknownKeywords(s, sp)
	}
}

// subValidate validates into a separate collector, for keywords like anyOf
// and not that need to know whether a branch passed without reporting its
// errors.
func (v *Validator) subValidate(schema, instance interface{}, ip, sp Pointer, depth int) *ErrorCollector {
	sub := &ErrorCollector{}
	v.validate(schema, instance, ip, sp, depth, sub)
	return sub
}

// knownKeywords is only used for --strict warnings.
var knownKeywords = map[string]bool{
	"$ref": true, "$schema": true, "$id": true, "$comment": true,
	"definitions": true, "title": true, "description": true, "default": true,
	"examples": true, "readOnly": true, "writeOnly": true,
	"type": true, "enum": true, "const": true,
	"minimum": true, "maximum": true, "exclusiveMinimum": true,
	"exclusiveMaximum": true, "multipleOf": true,
	"minLength": true, "maxLength": true, "pattern": true, "format": true,
	"properties": true, "required": true, "additionalProperties": true,
	"patternProperties": true, "minProperties": true, "maxProperties": true,
	"propertyNames": true,
	"items":         true, "additionalItems": true, "minItems": true,
	"maxItems": true, "uniqueItems": true, "contains": true,
	"allOf": true, "anyOf": true, "oneOf": true, "not": true,
	"if": true, "then": true, "else": true,
}

func (v *Validator) warnUnknownKeywords(s map[string]interface{}, sp Pointer) {
	for k := range s {
		if !knownKeywords[k] {
			v.warnings = append(v.warnings,
				fmt.Sprintf("unknown keyword %q at %s (ignored)", k, sp.Human()))
		}
	}
}
