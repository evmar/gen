package lr

// Functions for parsing Go source code, decorated with syntax(...) calls,
// into a set of grammar rules.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
)

// isSyntaxCall analyzes an ast.Stmt and returns (true, "...") if the
// statement is the special call to syntax("...").
func isSyntaxCall(s ast.Stmt) (matched bool, pattern string) {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return
	}
	e, ok := es.X.(*ast.CallExpr)
	if !ok {
		return
	}
	f, ok := e.Fun.(*ast.Ident)
	if !ok || f.Name != "syntax" {
		return
	}

	if len(e.Args) != 1 {
		return
	}
	arg := e.Args[0].(*ast.BasicLit).Value

	return true, arg[1 : len(arg)-1]
}

// astStr converts an ast node to its textual code representation.
func astStr(fset *token.FileSet, n interface{}) string {
	buf := &bytes.Buffer{}
	err := printer.Fprint(buf, fset, n)
	if err != nil {
		panic(err)
	}
	return string(buf.Bytes())
}

// processFunction analyzes a single func ast, extracting rules (and code)
// from it.
func processFunction(fn *ast.FuncDecl, fset *token.FileSet, rules *[]*Rule) {
	var rule *Rule
	var code []ast.Stmt
	for _, stmt := range fn.Body.List {
		if match, patternStr := isSyntaxCall(stmt); match {
			if rule != nil {
				rule.code = astStr(fset, code)
				*rules = append(*rules, rule)
			}

			pattern, vars := parsePattern(patternStr)
			rule = &Rule{
				symbol:  fn.Name.Name,
				typ:     astStr(fset, fn.Type.Results.List[0].Type),
				pattern: pattern,
				vars:    vars,
			}
			code = nil
		} else {
			code = append(code, stmt)
		}
	}

	if rule != nil {
		rule.code = astStr(fset, code)
		*rules = append(*rules, rule)
	}
	return
}

// Parse loads a go source file and extracts all the Rules from it.
func Parse(path string) ([]*Rule, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var rules []*Rule
	ast.Inspect(f, func(an ast.Node) bool {
		if fn, ok := an.(*ast.FuncDecl); ok {
			processFunction(fn, fset, &rules)
			return false // don't examine children
		}
		return true // visit children
	})

	return rules, nil
}
