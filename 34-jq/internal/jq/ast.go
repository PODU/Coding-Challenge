package jq

// Node is an AST node / compiled filter. eval feeds every output value to emit;
// returning an error aborts the current evaluation unless caught by Optional or
// TryCatch.
type Node interface {
	eval(in Value, emit func(Value) error) error
}

type (
	// identity: `.`
	identity struct{}

	// recurseDefault: `..`
	recurseDefault struct{}

	// field access: `.name`
	field struct{ name string }

	// index by an evaluated expression: `.[expr]`
	indexExpr struct{ idx Node }

	// slice: `.[lo:hi]`; lo or hi may be nil meaning open-ended
	sliceExpr struct{ lo, hi Node }

	// iterate: `.[]`
	iterate struct{}

	// optional: postfix `?`
	optional struct{ inner Node }

	// pipe: `l | r`
	pipe struct{ l, r Node }

	// comma: `l , r`
	comma struct{ l, r Node }

	// array construction: `[ inner ]`
	arrayConstruct struct{ inner Node } // inner nil means []

	// object construction: `{ ... }`
	objectConstruct struct{ entries []objEntry }

	literal struct{ val Value }

	// binary operator: arithmetic / comparison / boolean
	binOp struct {
		op   string
		l, r Node
	}

	// unary negation: `- x`
	unaryNeg struct{ x Node }

	// alternative: `l // r`
	alternative struct{ l, r Node }

	// if / elif / else; elif chains are nested ifs
	ifExpr struct {
		conds []Node
		thens []Node
		els   Node // nil means identity
	}

	// try / catch
	tryCatch struct {
		body    Node
		handler Node // nil means swallow
	}

	// function call: name(args...)
	funcCall struct {
		name string
		args []Node
	}

	// string interpolation
	strInterp struct {
		parts []interpPart
		// format applied to interpolated expressions, e.g. @base64 "\(x)"
		format string
	}

	// @base64 / @json / @text / ... applied to the input
	formatExpr struct{ name string }
)

type objEntry struct {
	key Node // emits a string
	val Node
}

type interpPart struct {
	isExpr bool
	text   string
	expr   Node
}
