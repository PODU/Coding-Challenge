// Package parser builds an AST from awk source text via recursive descent.
package parser

import (
	"fmt"
	"strconv"

	"goawk/ast"
	"goawk/lexer"
)

// Error is a parse error.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

type parser struct {
	toks []lexer.Token
	pos  int
	noGt int // >0 means '>' is redirection, not comparison (inside print args)
	noIn int // >0 means 'in' should not be consumed as an operator
}

// Parse parses awk source text into a Program.
func Parse(src string) (prog *ast.Program, err error) {
	toks, lerr := lexer.Tokenize(src)
	if lerr != nil {
		return nil, lerr
	}
	p := &parser{toks: toks}
	defer func() {
		if r := recover(); r != nil {
			if pe, ok := r.(*Error); ok {
				prog, err = nil, pe
				return
			}
			panic(r)
		}
	}()
	prog = p.parseProgram()
	analyze(prog)
	return prog, nil
}

func (p *parser) cur() lexer.Token    { return p.toks[p.pos] }
func (p *parser) curType() lexer.Type { return p.toks[p.pos].Type }
func (p *parser) peekType() lexer.Type {
	if p.pos+1 < len(p.toks) {
		return p.toks[p.pos+1].Type
	}
	return lexer.EOF
}

func (p *parser) advance() lexer.Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) at(t lexer.Type) bool { return p.curType() == t }

func (p *parser) accept(t lexer.Type) bool {
	if p.at(t) {
		p.advance()
		return true
	}
	return false
}

func (p *parser) expect(t lexer.Type) lexer.Token {
	if !p.at(t) {
		p.errf("expected %s, got %s", t, p.curType())
	}
	return p.advance()
}

func (p *parser) errf(format string, args ...any) {
	panic(&Error{Line: p.cur().Line, Msg: fmt.Sprintf(format, args...)})
}

func (p *parser) skipNewlines() {
	for p.at(lexer.NEWLINE) {
		p.advance()
	}
}

// skipTerminators consumes newlines and semicolons between statements.
func (p *parser) skipTerminators() {
	for p.at(lexer.NEWLINE) || p.at(lexer.SEMI) {
		p.advance()
	}
}

func (p *parser) parseProgram() *ast.Program {
	prog := &ast.Program{Functions: map[string]*ast.Function{}}
	p.skipTerminators()
	for !p.at(lexer.EOF) {
		if p.at(lexer.FUNCTION) {
			fn := p.parseFunction()
			if _, dup := prog.Functions[fn.Name]; dup {
				p.errf("function %q redefined", fn.Name)
			}
			prog.Functions[fn.Name] = fn
		} else {
			p.parseRule(prog)
		}
		p.skipTerminators()
	}
	return prog
}

func (p *parser) parseFunction() *ast.Function {
	p.expect(lexer.FUNCTION)
	var name string
	if p.at(lexer.NAME) || p.at(lexer.FUNC_NAME) {
		name = p.advance().Literal
	} else {
		p.errf("expected function name, got %s", p.curType())
	}
	p.expect(lexer.LPAREN)
	var params []string
	for !p.at(lexer.RPAREN) {
		params = append(params, p.expect(lexer.NAME).Literal)
		if !p.accept(lexer.COMMA) {
			break
		}
		p.skipNewlines()
	}
	p.expect(lexer.RPAREN)
	p.skipNewlines()
	body := p.parseBlock()
	return &ast.Function{Name: name, Params: params, Arrays: make([]bool, len(params)), Body: body}
}

func (p *parser) parseRule(prog *ast.Program) {
	switch p.curType() {
	case lexer.BEGIN:
		p.advance()
		p.skipNewlines()
		prog.Begin = append(prog.Begin, p.parseBlock())
	case lexer.END:
		p.advance()
		p.skipNewlines()
		prog.End = append(prog.End, p.parseBlock())
	case lexer.LBRACE:
		prog.Rules = append(prog.Rules, &ast.Rule{Action: p.parseBlock()})
	default:
		first := p.parseExpr()
		if p.accept(lexer.COMMA) {
			p.skipNewlines()
			second := p.parseExpr()
			pat := &ast.RangePattern{Start: first, Stop: second}
			var action *ast.Block
			if p.at(lexer.LBRACE) {
				action = p.parseBlock()
			}
			prog.Rules = append(prog.Rules, &ast.Rule{Pattern: pat, Action: action})
			return
		}
		pat := &ast.ExprPattern{X: first}
		var action *ast.Block
		if p.at(lexer.LBRACE) {
			action = p.parseBlock()
		}
		prog.Rules = append(prog.Rules, &ast.Rule{Pattern: pat, Action: action})
	}
}

func (p *parser) parseBlock() *ast.Block {
	p.expect(lexer.LBRACE)
	blk := &ast.Block{}
	p.skipTerminators()
	for !p.at(lexer.RBRACE) && !p.at(lexer.EOF) {
		blk.Stmts = append(blk.Stmts, p.parseStmt())
		p.skipTerminators()
	}
	p.expect(lexer.RBRACE)
	return blk
}

// parseSimpleStmt parses a single statement used where braces are optional
// (loop/if bodies). It does not consume trailing terminators.
func (p *parser) parseStmt() ast.Stmt {
	switch p.curType() {
	case lexer.LBRACE:
		return p.parseBlock()
	case lexer.IF:
		return p.parseIf()
	case lexer.WHILE:
		return p.parseWhile()
	case lexer.DO:
		return p.parseDoWhile()
	case lexer.FOR:
		return p.parseFor()
	case lexer.BREAK:
		p.advance()
		return &ast.Break{}
	case lexer.CONTINUE:
		p.advance()
		return &ast.Continue{}
	case lexer.NEXT:
		p.advance()
		return &ast.Next{}
	case lexer.NEXTFILE:
		p.advance()
		return &ast.NextFile{}
	case lexer.EXIT:
		p.advance()
		var code ast.Expr
		if p.startsExpr() {
			code = p.parseExpr()
		}
		return &ast.Exit{Code: code}
	case lexer.RETURN:
		p.advance()
		var val ast.Expr
		if p.startsExpr() {
			val = p.parseExpr()
		}
		return &ast.Return{Value: val}
	case lexer.DELETE:
		return p.parseDelete()
	case lexer.PRINT:
		return p.parsePrint(false)
	case lexer.PRINTF:
		return p.parsePrint(true)
	case lexer.SEMI:
		// empty statement
		return &ast.Block{}
	default:
		return &ast.ExprStmt{X: p.parseExpr()}
	}
}

func (p *parser) parseIf() ast.Stmt {
	p.expect(lexer.IF)
	p.expect(lexer.LPAREN)
	cond := p.parseExpr()
	p.expect(lexer.RPAREN)
	p.skipNewlines()
	then := p.parseStmt()
	stmt := &ast.If{Cond: cond, Then: then}
	// look for else, tolerating terminators in between
	save := p.pos
	p.skipTerminators()
	if p.accept(lexer.ELSE) {
		p.skipNewlines()
		stmt.Else = p.parseStmt()
	} else {
		p.pos = save
	}
	return stmt
}

func (p *parser) parseWhile() ast.Stmt {
	p.expect(lexer.WHILE)
	p.expect(lexer.LPAREN)
	cond := p.parseExpr()
	p.expect(lexer.RPAREN)
	p.skipNewlines()
	body := p.parseStmt()
	return &ast.While{Cond: cond, Body: body}
}

func (p *parser) parseDoWhile() ast.Stmt {
	p.expect(lexer.DO)
	p.skipNewlines()
	body := p.parseStmt()
	p.skipTerminators()
	p.expect(lexer.WHILE)
	p.expect(lexer.LPAREN)
	cond := p.parseExpr()
	p.expect(lexer.RPAREN)
	return &ast.DoWhile{Body: body, Cond: cond}
}

func (p *parser) parseFor() ast.Stmt {
	p.expect(lexer.FOR)
	p.expect(lexer.LPAREN)
	// for (k in arr)
	if p.at(lexer.NAME) && p.peekType() == lexer.IN {
		v := p.advance().Literal
		p.expect(lexer.IN)
		arr := p.expect(lexer.NAME).Literal
		p.expect(lexer.RPAREN)
		p.skipNewlines()
		body := p.parseStmt()
		return &ast.ForIn{Var: v, Array: arr, Body: body}
	}
	var init ast.Stmt
	if !p.at(lexer.SEMI) {
		init = p.parseSimpleForStmt()
	}
	p.expect(lexer.SEMI)
	var cond ast.Expr
	if !p.at(lexer.SEMI) {
		cond = p.parseExpr()
	}
	p.expect(lexer.SEMI)
	var post ast.Stmt
	if !p.at(lexer.RPAREN) {
		post = p.parseSimpleForStmt()
	}
	p.expect(lexer.RPAREN)
	p.skipNewlines()
	body := p.parseStmt()
	return &ast.For{Init: init, Cond: cond, Post: post, Body: body}
}

func (p *parser) parseSimpleForStmt() ast.Stmt {
	if p.at(lexer.DELETE) {
		return p.parseDelete()
	}
	return &ast.ExprStmt{X: p.parseExpr()}
}

func (p *parser) parseDelete() ast.Stmt {
	p.expect(lexer.DELETE)
	name := p.expect(lexer.NAME).Literal
	d := &ast.Delete{Name: name}
	if p.accept(lexer.LBRACKET) {
		d.Index = p.parseExprList(lexer.RBRACKET)
		p.expect(lexer.RBRACKET)
	}
	return d
}

func (p *parser) parsePrint(isPrintf bool) ast.Stmt {
	p.advance() // print / printf
	var args []ast.Expr
	if p.startsExpr() {
		p.noGt++
		args = p.parsePrintArgs()
		p.noGt--
	}
	// unwrap a single parenthesised list: print (a, b)
	if len(args) == 1 {
		if g, ok := args[0].(*ast.Grouping); ok {
			args = g.Exprs
		}
	}
	var redir *ast.Redirect
	switch p.curType() {
	case lexer.GT:
		p.advance()
		redir = &ast.Redirect{Mode: ">", Target: p.parseExpr()}
	case lexer.APPEND:
		p.advance()
		redir = &ast.Redirect{Mode: ">>", Target: p.parseExpr()}
	case lexer.PIPE:
		p.advance()
		redir = &ast.Redirect{Mode: "|", Target: p.parseExpr()}
	}
	if isPrintf {
		return &ast.Printf{Args: args, Redir: redir}
	}
	return &ast.Print{Args: args, Redir: redir}
}

func (p *parser) parsePrintArgs() []ast.Expr {
	var list []ast.Expr
	list = append(list, p.parseExpr())
	for p.accept(lexer.COMMA) {
		p.skipNewlines()
		list = append(list, p.parseExpr())
	}
	return list
}

func (p *parser) parseExprList(end lexer.Type) []ast.Expr {
	var list []ast.Expr
	if p.at(end) {
		return list
	}
	list = append(list, p.parseExpr())
	for p.accept(lexer.COMMA) {
		p.skipNewlines()
		list = append(list, p.parseExpr())
	}
	return list
}

// startsExpr reports whether the current token can begin an expression.
func (p *parser) startsExpr() bool {
	switch p.curType() {
	case lexer.NUMBER, lexer.STRING, lexer.REGEX, lexer.NAME, lexer.FUNC_NAME,
		lexer.DOLLAR, lexer.NOT, lexer.SUB, lexer.ADD, lexer.LPAREN,
		lexer.INCR, lexer.DECR, lexer.GETLINE:
		return true
	}
	return false
}

func (p *parser) parseExpr() ast.Expr { return p.parseAssign() }

func isAssignOp(t lexer.Type) bool {
	switch t {
	case lexer.ASSIGN, lexer.ADD_ASSIGN, lexer.SUB_ASSIGN, lexer.MUL_ASSIGN,
		lexer.DIV_ASSIGN, lexer.MOD_ASSIGN, lexer.POW_ASSIGN:
		return true
	}
	return false
}

func assignOpString(t lexer.Type) string {
	switch t {
	case lexer.ASSIGN:
		return "="
	case lexer.ADD_ASSIGN:
		return "+="
	case lexer.SUB_ASSIGN:
		return "-="
	case lexer.MUL_ASSIGN:
		return "*="
	case lexer.DIV_ASSIGN:
		return "/="
	case lexer.MOD_ASSIGN:
		return "%="
	case lexer.POW_ASSIGN:
		return "^="
	}
	return ""
}

func isLvalue(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Var, *ast.Field, *ast.IndexExpr:
		return true
	}
	return false
}

func (p *parser) parseAssign() ast.Expr {
	left := p.parseTernary()
	if isAssignOp(p.curType()) && isLvalue(left) {
		op := assignOpString(p.advance().Type)
		p.skipNewlines()
		right := p.parseAssign()
		return &ast.Assign{Op: op, Target: left, Value: right}
	}
	return left
}

func (p *parser) parseTernary() ast.Expr {
	cond := p.parseOr()
	if p.accept(lexer.QUESTION) {
		p.skipNewlines()
		then := p.parseAssign()
		p.expect(lexer.COLON)
		p.skipNewlines()
		els := p.parseAssign()
		return &ast.Ternary{Cond: cond, Then: then, Else: els}
	}
	return cond
}

func (p *parser) parseOr() ast.Expr {
	left := p.parseAnd()
	for p.at(lexer.OR) {
		p.advance()
		p.skipNewlines()
		right := p.parseAnd()
		left = &ast.Binary{Op: "||", Left: left, Right: right}
	}
	return left
}

func (p *parser) parseAnd() ast.Expr {
	left := p.parseIn()
	for p.at(lexer.AND) {
		p.advance()
		p.skipNewlines()
		right := p.parseIn()
		left = &ast.Binary{Op: "&&", Left: left, Right: right}
	}
	return left
}

func (p *parser) parseIn() ast.Expr {
	left := p.parseMatch()
	for p.noIn == 0 && p.at(lexer.IN) {
		p.advance()
		arr := p.expect(lexer.NAME).Literal
		left = &ast.In{Index: []ast.Expr{left}, Array: arr}
	}
	return left
}

func (p *parser) parseMatch() ast.Expr {
	left := p.parseComparison()
	for p.at(lexer.MATCH) || p.at(lexer.NOTMATCH) {
		neg := p.curType() == lexer.NOTMATCH
		p.advance()
		right := p.parseComparison()
		left = &ast.Match{Negated: neg, Left: left, Right: right}
	}
	return left
}

func (p *parser) parseComparison() ast.Expr {
	left := p.parseConcat()
	switch p.curType() {
	case lexer.LT, lexer.LE, lexer.EQ, lexer.NE, lexer.GE:
		op := p.advance().Type.String()
		right := p.parseConcat()
		return &ast.Binary{Op: op, Left: left, Right: right}
	case lexer.GT:
		if p.noGt > 0 {
			return left
		}
		p.advance()
		right := p.parseConcat()
		return &ast.Binary{Op: ">", Left: left, Right: right}
	}
	return left
}

func (p *parser) startsConcatOperand() bool {
	switch p.curType() {
	case lexer.NUMBER, lexer.STRING, lexer.REGEX, lexer.NAME, lexer.FUNC_NAME,
		lexer.DOLLAR, lexer.LPAREN, lexer.NOT, lexer.INCR, lexer.DECR:
		return true
	}
	return false
}

func (p *parser) parseConcat() ast.Expr {
	left := p.parseAdd()
	for p.startsConcatOperand() {
		// a '|' getline binds tighter; do not concat across it.
		right := p.parseAdd()
		left = &ast.Concat{Left: left, Right: right}
	}
	return left
}

func (p *parser) parseAdd() ast.Expr {
	left := p.parsePipeGetline()
	for p.at(lexer.ADD) || p.at(lexer.SUB) {
		op := p.advance().Type.String()
		right := p.parsePipeGetline()
		left = &ast.Binary{Op: op, Left: left, Right: right}
	}
	return left
}

// parsePipeGetline handles "cmd" | getline [var] at a precedence just above
// additive operators (so `"sort " f | getline` reads the filename first).
func (p *parser) parsePipeGetline() ast.Expr {
	left := p.parseMul()
	for p.at(lexer.PIPE) && p.peekType() == lexer.GETLINE {
		p.advance() // |
		p.advance() // getline
		var target ast.Expr
		if p.at(lexer.DOLLAR) || (p.at(lexer.NAME) && p.peekType() != lexer.LPAREN) {
			target = p.parseLvalue()
		}
		left = &ast.Getline{Mode: ast.GetlineCmd, Source: left, Target: target}
	}
	return left
}

func (p *parser) parseMul() ast.Expr {
	left := p.parseUnary()
	for p.at(lexer.MUL) || p.at(lexer.DIV) || p.at(lexer.MOD) {
		op := p.advance().Type.String()
		right := p.parseUnary()
		left = &ast.Binary{Op: op, Left: left, Right: right}
	}
	return left
}

func (p *parser) parseUnary() ast.Expr {
	switch p.curType() {
	case lexer.NOT:
		p.advance()
		return &ast.Unary{Op: "!", X: p.parseUnary()}
	case lexer.SUB:
		p.advance()
		return &ast.Unary{Op: "-", X: p.parseUnary()}
	case lexer.ADD:
		p.advance()
		return &ast.Unary{Op: "+", X: p.parseUnary()}
	case lexer.INCR:
		p.advance()
		return &ast.IncDec{Pre: true, Op: "++", Target: p.parseUnary()}
	case lexer.DECR:
		p.advance()
		return &ast.IncDec{Pre: true, Op: "--", Target: p.parseUnary()}
	}
	return p.parsePow()
}

func (p *parser) parsePow() ast.Expr {
	base := p.parsePostfix()
	if p.at(lexer.POW) {
		p.advance()
		// right-associative; allow unary on the exponent (2^-3)
		exp := p.parseUnary()
		return &ast.Binary{Op: "^", Left: base, Right: exp}
	}
	return base
}

func (p *parser) parsePostfix() ast.Expr {
	x := p.parseField()
	for (p.at(lexer.INCR) || p.at(lexer.DECR)) && isLvalue(x) {
		op := "++"
		if p.curType() == lexer.DECR {
			op = "--"
		}
		p.advance()
		x = &ast.IncDec{Pre: false, Op: op, Target: x}
	}
	return x
}

func (p *parser) parseField() ast.Expr {
	if p.accept(lexer.DOLLAR) {
		return &ast.Field{Index: p.parseField()}
	}
	return p.parsePrimary()
}

// parseLvalue parses a $field, name, or name[...] as an lvalue.
func (p *parser) parseLvalue() ast.Expr {
	if p.accept(lexer.DOLLAR) {
		return &ast.Field{Index: p.parseField()}
	}
	name := p.expect(lexer.NAME).Literal
	if p.accept(lexer.LBRACKET) {
		idx := p.parseExprList(lexer.RBRACKET)
		p.expect(lexer.RBRACKET)
		return &ast.IndexExpr{Name: name, Index: idx}
	}
	return &ast.Var{Name: name}
}

func (p *parser) parsePrimary() ast.Expr {
	switch p.curType() {
	case lexer.NUMBER:
		t := p.advance()
		v, err := strconv.ParseFloat(t.Literal, 64)
		if err != nil {
			p.errf("invalid number %q", t.Literal)
		}
		return &ast.Num{Value: v}
	case lexer.STRING:
		return &ast.Str{Value: p.advance().Literal}
	case lexer.REGEX:
		return &ast.Regex{Pattern: p.advance().Literal}
	case lexer.GETLINE:
		return p.parseGetline()
	case lexer.LPAREN:
		return p.parseGrouping()
	case lexer.NAME:
		name := p.advance().Literal
		if p.accept(lexer.LBRACKET) {
			idx := p.parseExprList(lexer.RBRACKET)
			p.expect(lexer.RBRACKET)
			return &ast.IndexExpr{Name: name, Index: idx}
		}
		if isBuiltinName(name) && p.at(lexer.LPAREN) {
			return p.parseCall(name)
		}
		// "length" may be used without parentheses
		if name == "length" {
			return &ast.Call{Name: "length"}
		}
		return &ast.Var{Name: name}
	case lexer.FUNC_NAME:
		name := p.advance().Literal
		return p.parseCall(name)
	default:
		p.errf("unexpected token %s", p.curType())
	}
	return nil
}

func (p *parser) parseCall(name string) ast.Expr {
	p.expect(lexer.LPAREN)
	saveGt, saveIn := p.noGt, p.noIn
	p.noGt, p.noIn = 0, 0
	args := p.parseExprList(lexer.RPAREN)
	p.noGt, p.noIn = saveGt, saveIn
	p.expect(lexer.RPAREN)
	return &ast.Call{Name: name, Args: args}
}

func (p *parser) parseGrouping() ast.Expr {
	p.expect(lexer.LPAREN)
	saveGt := p.noGt
	p.noGt = 0
	first := p.parseExpr()
	if p.at(lexer.COMMA) {
		exprs := []ast.Expr{first}
		for p.accept(lexer.COMMA) {
			p.skipNewlines()
			exprs = append(exprs, p.parseExpr())
		}
		p.expect(lexer.RPAREN)
		p.noGt = saveGt
		// (i, j) in arr
		if p.accept(lexer.IN) {
			arr := p.expect(lexer.NAME).Literal
			return &ast.In{Index: exprs, Array: arr}
		}
		return &ast.Grouping{Exprs: exprs}
	}
	p.expect(lexer.RPAREN)
	p.noGt = saveGt
	return first
}

func (p *parser) parseGetline() ast.Expr {
	p.expect(lexer.GETLINE)
	g := &ast.Getline{Mode: ast.GetlineMain}
	if p.at(lexer.DOLLAR) || (p.at(lexer.NAME) && p.peekType() != lexer.LPAREN) {
		g.Target = p.parseLvalue()
	}
	if p.accept(lexer.LT) {
		g.Mode = ast.GetlineFile
		g.Source = p.parseConcat()
	}
	return g
}

func isBuiltinName(name string) bool {
	switch name {
	case "length", "substr", "index", "split", "sub", "gsub", "match",
		"sprintf", "tolower", "toupper", "sin", "cos", "atan2", "exp",
		"log", "sqrt", "int", "rand", "srand", "system", "close", "fflush":
		return true
	}
	return false
}
