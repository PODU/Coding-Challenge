// Package ast defines the node types for the awk abstract syntax tree.
package ast

// Program is the root of a parsed awk program.
type Program struct {
	Functions map[string]*Function
	Begin     []*Block
	End       []*Block
	Rules     []*Rule
}

// Function is a user-defined function.
type Function struct {
	Name   string
	Params []string
	Arrays []bool // Arrays[i] true if param i is used as an array (filled by analysis)
	Body   *Block
}

// Rule is a pattern/action pair. A nil Pattern means "match every record";
// a nil Action means "{ print $0 }".
type Rule struct {
	Pattern Pattern
	Action  *Block
}

// Pattern is the marker interface for rule patterns.
type Pattern interface{ patternNode() }

// ExprPattern matches when its expression is truthy.
type ExprPattern struct{ X Expr }

// RangePattern matches records from a line matching Start through a line
// matching Stop (inclusive, re-armable).
type RangePattern struct {
	Start, Stop Expr
}

func (*ExprPattern) patternNode()  {}
func (*RangePattern) patternNode() {}

// Expr is the marker interface for expressions.
type Expr interface{ exprNode() }

type (
	// Num is a numeric literal.
	Num struct{ Value float64 }
	// Str is a string literal.
	Str struct{ Value string }
	// Regex is a /regex/ literal.
	Regex struct{ Pattern string }
	// Var is a scalar variable reference.
	Var struct{ Name string }
	// Field is a $expr field reference.
	Field struct{ Index Expr }
	// IndexExpr is an array element reference name[i,j,...].
	IndexExpr struct {
		Name  string
		Index []Expr
	}
	// Assign is an assignment with an optional compound operator.
	Assign struct {
		Op     string // "=", "+=", "-=", "*=", "/=", "%=", "^="
		Target Expr
		Value  Expr
	}
	// Binary is a binary arithmetic/comparison/logical operation.
	Binary struct {
		Op          string
		Left, Right Expr
	}
	// Concat is string concatenation by juxtaposition.
	Concat struct{ Left, Right Expr }
	// Unary is a unary operation (- + !).
	Unary struct {
		Op string
		X  Expr
	}
	// IncDec is a pre/post increment or decrement.
	IncDec struct {
		Pre    bool
		Op     string // "++" or "--"
		Target Expr
	}
	// Ternary is c ? a : b.
	Ternary struct{ Cond, Then, Else Expr }
	// Match is the ~ / !~ operator.
	Match struct {
		Negated     bool
		Left, Right Expr
	}
	// In tests array membership: (i,j) in arr.
	In struct {
		Index []Expr
		Array string
	}
	// Call invokes a builtin or user-defined function.
	Call struct {
		Name string
		Args []Expr
	}
	// Getline is one of the getline variants.
	Getline struct {
		Target Expr // nil => sets $0
		Mode   GetlineMode
		Source Expr // file name or command, depending on Mode
	}
	// Grouping is a parenthesised list of expressions (used by print).
	Grouping struct{ Exprs []Expr }
)

// GetlineMode distinguishes the getline forms.
type GetlineMode int

const (
	// GetlineMain reads from the main input stream.
	GetlineMain GetlineMode = iota
	// GetlineFile reads from a file: getline < "file".
	GetlineFile
	// GetlineCmd reads from a command: "cmd" | getline.
	GetlineCmd
)

func (*Num) exprNode()       {}
func (*Str) exprNode()       {}
func (*Regex) exprNode()     {}
func (*Var) exprNode()       {}
func (*Field) exprNode()     {}
func (*IndexExpr) exprNode() {}
func (*Assign) exprNode()    {}
func (*Binary) exprNode()    {}
func (*Concat) exprNode()    {}
func (*Unary) exprNode()     {}
func (*IncDec) exprNode()    {}
func (*Ternary) exprNode()   {}
func (*Match) exprNode()     {}
func (*In) exprNode()        {}
func (*Call) exprNode()      {}
func (*Getline) exprNode()   {}
func (*Grouping) exprNode()  {}

// Stmt is the marker interface for statements.
type Stmt interface{ stmtNode() }

type (
	// Block is a brace-delimited list of statements.
	Block struct{ Stmts []Stmt }
	// ExprStmt is an expression used as a statement.
	ExprStmt struct{ X Expr }
	// Print is a print statement.
	Print struct {
		Args  []Expr
		Redir *Redirect
	}
	// Printf is a printf statement.
	Printf struct {
		Args  []Expr
		Redir *Redirect
	}
	// If is an if/else statement.
	If struct {
		Cond Expr
		Then Stmt
		Else Stmt
	}
	// While is a while loop.
	While struct {
		Cond Expr
		Body Stmt
	}
	// DoWhile is a do/while loop.
	DoWhile struct {
		Body Stmt
		Cond Expr
	}
	// For is a C-style for loop.
	For struct {
		Init Stmt
		Cond Expr
		Post Stmt
		Body Stmt
	}
	// ForIn is for (k in arr).
	ForIn struct {
		Var   string
		Array string
		Body  Stmt
	}
	// Next aborts processing of the current record.
	Next struct{}
	// NextFile skips the rest of the current input file.
	NextFile struct{}
	// Exit terminates the program (running END first).
	Exit struct{ Code Expr }
	// Return returns from a user function.
	Return struct{ Value Expr }
	// Delete removes an array element or clears an array.
	Delete struct {
		Name  string
		Index []Expr // nil => delete whole array
	}
	// Break breaks out of a loop.
	Break struct{}
	// Continue continues a loop.
	Continue struct{}
)

// Redirect describes output redirection for print/printf.
type Redirect struct {
	Mode   string // ">", ">>", "|"
	Target Expr
}

func (*Block) stmtNode()    {}
func (*ExprStmt) stmtNode() {}
func (*Print) stmtNode()    {}
func (*Printf) stmtNode()   {}
func (*If) stmtNode()       {}
func (*While) stmtNode()    {}
func (*DoWhile) stmtNode()  {}
func (*For) stmtNode()      {}
func (*ForIn) stmtNode()    {}
func (*Next) stmtNode()     {}
func (*NextFile) stmtNode() {}
func (*Exit) stmtNode()     {}
func (*Return) stmtNode()   {}
func (*Delete) stmtNode()   {}
func (*Break) stmtNode()    {}
func (*Continue) stmtNode() {}
