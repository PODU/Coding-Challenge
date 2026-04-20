package interp

import (
	"math"
	"regexp"

	"goawk/ast"
)

func (p *Interp) eval(e ast.Expr) value {
	switch n := e.(type) {
	case *ast.Num:
		return num(n.Value)
	case *ast.Str:
		return str(n.Value)
	case *ast.Regex:
		// bare regex means $0 ~ /re/
		return boolVal(p.compileRegex(n.Pattern).MatchString(p.line))
	case *ast.Var:
		return p.getVar(n.Name)
	case *ast.Field:
		return p.getField(int(p.eval(n.Index).num()))
	case *ast.IndexExpr:
		return p.getArrayElem(n)
	case *ast.Grouping:
		if len(n.Exprs) == 1 {
			return p.eval(n.Exprs[0])
		}
		// a bare multi-element grouping is unusual; evaluate to the last.
		var v value
		for _, e := range n.Exprs {
			v = p.eval(e)
		}
		return v
	case *ast.Assign:
		return p.evalAssign(n)
	case *ast.Binary:
		return p.evalBinary(n)
	case *ast.Concat:
		return str(p.eval(n.Left).str(p.convfmt) + p.eval(n.Right).str(p.convfmt))
	case *ast.Unary:
		return p.evalUnary(n)
	case *ast.IncDec:
		return p.evalIncDec(n)
	case *ast.Ternary:
		if p.eval(n.Cond).boolean() {
			return p.eval(n.Then)
		}
		return p.eval(n.Else)
	case *ast.Match:
		re := p.evalRegexExpr(n.Right)
		matched := re.MatchString(p.eval(n.Left).str(p.convfmt))
		if n.Negated {
			matched = !matched
		}
		return boolVal(matched)
	case *ast.In:
		return boolVal(p.inArray(n.Array, n.Index))
	case *ast.Call:
		return p.evalCall(n)
	case *ast.Getline:
		return p.evalGetline(n)
	}
	p.fatalf("cannot evaluate expression %T", e)
	return uninit
}

func (p *Interp) getArrayElem(n *ast.IndexExpr) value {
	arr := p.resolveArray(n.Name)
	key := p.indexKey(n.Index)
	v, ok := arr[key]
	if !ok {
		// referencing an element creates it (awk semantics)
		arr[key] = uninit
		return uninit
	}
	return v
}

func (p *Interp) indexKey(indices []ast.Expr) string {
	if len(indices) == 1 {
		return p.eval(indices[0]).str(p.convfmt)
	}
	parts := make([]string, len(indices))
	for i, e := range indices {
		parts[i] = p.eval(e).str(p.convfmt)
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += p.subsep + parts[i]
	}
	return out
}

func (p *Interp) inArray(name string, indices []ast.Expr) bool {
	arr := p.resolveArray(name)
	_, ok := arr[p.indexKey(indices)]
	return ok
}

func (p *Interp) evalAssign(n *ast.Assign) value {
	rval := p.eval(n.Value)
	if n.Op != "=" {
		cur := p.evalLvalue(n.Target)
		var r float64
		a, b := cur.num(), rval.num()
		switch n.Op {
		case "+=":
			r = a + b
		case "-=":
			r = a - b
		case "*=":
			r = a * b
		case "/=":
			if b == 0 {
				p.fatalf("division by zero in /=")
			}
			r = a / b
		case "%=":
			if b == 0 {
				p.fatalf("division by zero in %%=")
			}
			r = math.Mod(a, b)
		case "^=":
			r = math.Pow(a, b)
		}
		rval = num(r)
	}
	p.assignTo(n.Target, rval)
	return rval
}

func (p *Interp) evalLvalue(target ast.Expr) value {
	switch t := target.(type) {
	case *ast.Var:
		return p.getVar(t.Name)
	case *ast.Field:
		return p.getField(int(p.eval(t.Index).num()))
	case *ast.IndexExpr:
		return p.getArrayElem(t)
	}
	p.fatalf("invalid assignment target")
	return uninit
}

func (p *Interp) assignTo(target ast.Expr, v value) {
	switch t := target.(type) {
	case *ast.Var:
		p.setVar(t.Name, v)
	case *ast.Field:
		p.setField(int(p.eval(t.Index).num()), v.str(p.convfmt))
	case *ast.IndexExpr:
		arr := p.resolveArray(t.Name)
		arr[p.indexKey(t.Index)] = v
	default:
		p.fatalf("invalid assignment target")
	}
}

func (p *Interp) evalBinary(n *ast.Binary) value {
	switch n.Op {
	case "&&":
		if !p.eval(n.Left).boolean() {
			return num(0)
		}
		return boolVal(p.eval(n.Right).boolean())
	case "||":
		if p.eval(n.Left).boolean() {
			return num(1)
		}
		return boolVal(p.eval(n.Right).boolean())
	}
	l := p.eval(n.Left)
	r := p.eval(n.Right)
	switch n.Op {
	case "+":
		return num(l.num() + r.num())
	case "-":
		return num(l.num() - r.num())
	case "*":
		return num(l.num() * r.num())
	case "/":
		if r.num() == 0 {
			p.fatalf("division by zero")
		}
		return num(l.num() / r.num())
	case "%":
		if r.num() == 0 {
			p.fatalf("division by zero in %%")
		}
		return num(math.Mod(l.num(), r.num()))
	case "^":
		return num(math.Pow(l.num(), r.num()))
	case "<":
		return boolVal(compareValues(l, r, p.convfmt) < 0)
	case "<=":
		return boolVal(compareValues(l, r, p.convfmt) <= 0)
	case ">":
		return boolVal(compareValues(l, r, p.convfmt) > 0)
	case ">=":
		return boolVal(compareValues(l, r, p.convfmt) >= 0)
	case "==":
		return boolVal(compareValues(l, r, p.convfmt) == 0)
	case "!=":
		return boolVal(compareValues(l, r, p.convfmt) != 0)
	}
	p.fatalf("unknown operator %q", n.Op)
	return uninit
}

func (p *Interp) evalUnary(n *ast.Unary) value {
	switch n.Op {
	case "-":
		return num(-p.eval(n.X).num())
	case "+":
		return num(p.eval(n.X).num())
	case "!":
		return boolVal(!p.eval(n.X).boolean())
	}
	return uninit
}

func (p *Interp) evalIncDec(n *ast.IncDec) value {
	old := p.evalLvalue(n.Target).num()
	var nv float64
	if n.Op == "++" {
		nv = old + 1
	} else {
		nv = old - 1
	}
	p.assignTo(n.Target, num(nv))
	if n.Pre {
		return num(nv)
	}
	return num(old)
}

// evalRegexExpr returns a compiled regex from the RHS of ~, sub, gsub, etc.
func (p *Interp) evalRegexExpr(e ast.Expr) *regexp.Regexp {
	if r, ok := e.(*ast.Regex); ok {
		return p.compileRegex(r.Pattern)
	}
	return p.compileRegex(p.eval(e).str(p.convfmt))
}

func (p *Interp) compileRegex(pattern string) *regexp.Regexp {
	if re, ok := p.regexCache[pattern]; ok {
		return re
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		p.fatalf("invalid regular expression %q: %v", pattern, err)
	}
	p.regexCache[pattern] = re
	return re
}
