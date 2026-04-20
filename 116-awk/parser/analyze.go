package parser

import "goawk/ast"

// analyze determines, for each user function, which parameters are used as
// arrays. This is needed so the interpreter can pass arrays by reference and
// scalars by value. A fixpoint is computed because array-ness can propagate
// through calls to other user functions.
func analyze(prog *ast.Program) {
	for changed := true; changed; {
		changed = false
		for _, fn := range prog.Functions {
			idx := map[string]int{}
			for i, param := range fn.Params {
				idx[param] = i
			}
			mark := func(name string) {
				if i, ok := idx[name]; ok && !fn.Arrays[i] {
					fn.Arrays[i] = true
					changed = true
				}
			}
			a := &analyzer{prog: prog, mark: mark}
			a.block(fn.Body)
		}
	}
}

type analyzer struct {
	prog *ast.Program
	mark func(name string)
}

func (a *analyzer) block(b *ast.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		a.stmt(s)
	}
}

func (a *analyzer) stmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.Block:
		a.block(n)
	case *ast.ExprStmt:
		a.expr(n.X)
	case *ast.Print:
		a.exprs(n.Args)
		if n.Redir != nil {
			a.expr(n.Redir.Target)
		}
	case *ast.Printf:
		a.exprs(n.Args)
		if n.Redir != nil {
			a.expr(n.Redir.Target)
		}
	case *ast.If:
		a.expr(n.Cond)
		a.stmt(n.Then)
		if n.Else != nil {
			a.stmt(n.Else)
		}
	case *ast.While:
		a.expr(n.Cond)
		a.stmt(n.Body)
	case *ast.DoWhile:
		a.stmt(n.Body)
		a.expr(n.Cond)
	case *ast.For:
		if n.Init != nil {
			a.stmt(n.Init)
		}
		if n.Cond != nil {
			a.expr(n.Cond)
		}
		if n.Post != nil {
			a.stmt(n.Post)
		}
		a.stmt(n.Body)
	case *ast.ForIn:
		a.mark(n.Array)
		a.stmt(n.Body)
	case *ast.Exit:
		if n.Code != nil {
			a.expr(n.Code)
		}
	case *ast.Return:
		if n.Value != nil {
			a.expr(n.Value)
		}
	case *ast.Delete:
		a.mark(n.Name)
		a.exprs(n.Index)
	}
}

func (a *analyzer) exprs(es []ast.Expr) {
	for _, e := range es {
		a.expr(e)
	}
}

func (a *analyzer) expr(e ast.Expr) {
	switch n := e.(type) {
	case *ast.IndexExpr:
		a.mark(n.Name)
		a.exprs(n.Index)
	case *ast.In:
		a.mark(n.Array)
		a.exprs(n.Index)
	case *ast.Field:
		a.expr(n.Index)
	case *ast.Assign:
		a.expr(n.Target)
		a.expr(n.Value)
	case *ast.Binary:
		a.expr(n.Left)
		a.expr(n.Right)
	case *ast.Concat:
		a.expr(n.Left)
		a.expr(n.Right)
	case *ast.Unary:
		a.expr(n.X)
	case *ast.IncDec:
		a.expr(n.Target)
	case *ast.Ternary:
		a.expr(n.Cond)
		a.expr(n.Then)
		a.expr(n.Else)
	case *ast.Match:
		a.expr(n.Left)
		a.expr(n.Right)
	case *ast.Getline:
		if n.Target != nil {
			a.expr(n.Target)
		}
		if n.Source != nil {
			a.expr(n.Source)
		}
	case *ast.Grouping:
		a.exprs(n.Exprs)
	case *ast.Call:
		// split(s, arr[, fs]) -> arg 1 is an array
		if n.Name == "split" && len(n.Args) >= 2 {
			if v, ok := n.Args[1].(*ast.Var); ok {
				a.mark(v.Name)
			}
		}
		// propagate through user-function array parameters
		if fn, ok := a.prog.Functions[n.Name]; ok {
			for i, arg := range n.Args {
				if i < len(fn.Arrays) && fn.Arrays[i] {
					if v, ok := arg.(*ast.Var); ok {
						a.mark(v.Name)
					}
				}
			}
		}
		a.exprs(n.Args)
	}
}
