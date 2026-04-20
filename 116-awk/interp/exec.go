package interp

import (
	"strings"

	"goawk/ast"
)

func (p *Interp) execBlock(b *ast.Block) ctrl {
	if b == nil {
		return ctrlNone
	}
	for _, s := range b.Stmts {
		if c := p.execStmt(s); c != ctrlNone {
			return c
		}
	}
	return ctrlNone
}

func (p *Interp) execStmt(s ast.Stmt) ctrl {
	switch n := s.(type) {
	case *ast.Block:
		return p.execBlock(n)
	case *ast.ExprStmt:
		p.eval(n.X)
	case *ast.Print:
		p.execPrint(n)
	case *ast.Printf:
		p.execPrintf(n)
	case *ast.If:
		if p.eval(n.Cond).boolean() {
			return p.execStmt(n.Then)
		} else if n.Else != nil {
			return p.execStmt(n.Else)
		}
	case *ast.While:
		for p.eval(n.Cond).boolean() {
			c := p.execStmt(n.Body)
			if c == ctrlBreak {
				break
			}
			if c == ctrlReturn {
				return c
			}
		}
	case *ast.DoWhile:
		for {
			c := p.execStmt(n.Body)
			if c == ctrlBreak {
				break
			}
			if c == ctrlReturn {
				return c
			}
			if !p.eval(n.Cond).boolean() {
				break
			}
		}
	case *ast.For:
		if n.Init != nil {
			p.execStmt(n.Init)
		}
		for n.Cond == nil || p.eval(n.Cond).boolean() {
			c := p.execStmt(n.Body)
			if c == ctrlBreak {
				break
			}
			if c == ctrlReturn {
				return c
			}
			if n.Post != nil {
				p.execStmt(n.Post)
			}
		}
	case *ast.ForIn:
		return p.execForIn(n)
	case *ast.Break:
		return ctrlBreak
	case *ast.Continue:
		return ctrlContinue
	case *ast.Next:
		panic(nextSignal{})
	case *ast.NextFile:
		panic(nextFileSignal{})
	case *ast.Exit:
		code := p.exitCode
		if n.Code != nil {
			code = int(p.eval(n.Code).num())
		}
		panic(exitSignal{code: code})
	case *ast.Return:
		if n.Value != nil {
			p.returnValue = p.eval(n.Value)
		} else {
			p.returnValue = uninit
		}
		return ctrlReturn
	case *ast.Delete:
		p.execDelete(n)
	default:
		p.fatalf("unknown statement %T", s)
	}
	return ctrlNone
}

func (p *Interp) execForIn(n *ast.ForIn) ctrl {
	arr := p.resolveArray(n.Array)
	keys := make([]string, 0, len(arr))
	for k := range arr {
		keys = append(keys, k)
	}
	for _, k := range keys {
		p.setVar(n.Var, strnum(k))
		c := p.execStmt(n.Body)
		if c == ctrlBreak {
			break
		}
		if c == ctrlReturn {
			return c
		}
	}
	return ctrlNone
}

func (p *Interp) execDelete(n *ast.Delete) {
	arr := p.resolveArray(n.Name)
	if n.Index == nil {
		for k := range arr {
			delete(arr, k)
		}
		return
	}
	delete(arr, p.indexKey(n.Index))
}

func (p *Interp) execPrint(n *ast.Print) {
	var s string
	if len(n.Args) == 0 {
		s = p.line
	} else {
		parts := make([]string, len(n.Args))
		for i, a := range n.Args {
			parts[i] = p.outputStr(p.eval(a))
		}
		s = strings.Join(parts, p.ofs)
	}
	out := s + p.ors
	if n.Redir != nil {
		p.redirectWrite(n.Redir, out)
	} else {
		p.writeOutput(out)
	}
}

func (p *Interp) execPrintf(n *ast.Printf) {
	if len(n.Args) == 0 {
		return
	}
	format := p.eval(n.Args[0]).str(p.convfmt)
	args := make([]value, len(n.Args)-1)
	for i := 1; i < len(n.Args); i++ {
		args[i-1] = p.eval(n.Args[i])
	}
	out := p.sprintf(format, args)
	if n.Redir != nil {
		p.redirectWrite(n.Redir, out)
	} else {
		p.writeOutput(out)
	}
}

// outputStr renders a value for print, using OFMT for non-integral numbers.
func (p *Interp) outputStr(v value) string {
	if v.typ == typeNum {
		return formatNum(v.n, p.ofmt)
	}
	return v.str(p.convfmt)
}
