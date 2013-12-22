package lr

// Functions for parsing Go source code, decorated with syntax(...) calls,
// into a set of grammar rules.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

// Params controls parameters to the generation process.
type Params struct {
	// Prefix is inserted as a prefix on all types; useful to prevent
	// inter-file conflicts.
	Prefix string
	// Package is the package name for the output.
	Package string
	// TokenType is the name of the type of tokens passed to the
	// generation function.
	TokenType string
	// Trace specifies whether to log the parse as it happens.
	Trace bool
}

func warn(fset *token.FileSet, pos token.Pos, message string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", fset.Position(pos), message)
}

// parsePattern parses a pattern string, which looks like
//   A=expr + B=expr
// into a list of patterns ["expr", "+", "expr"] and
// variable names ["A", "", "B"].
func parsePattern(patternStr string) ([]string, []string) {
	pattern := strings.Split(patternStr, " ")
	vars := make([]string, len(pattern))
	for i, pat := range pattern {
		if len(pat) > 2 && pat[0] != '\'' && pat[1] == '=' {
			vars[i] = pat[0:1]
			pattern[i] = pat[2:]
		}
	}
	return pattern, vars
}

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

func literalString(e ast.Expr, fset *token.FileSet) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		warn(fset, e.Pos(), "expected literal value")
		return "", false
	}
	if lit.Kind != token.STRING {
		warn(fset, e.Pos(), "expected string")
		return "", false
	}
	return lit.Value[1 : len(lit.Value)-1], true
}

func processDecl(d *ast.GenDecl, fset *token.FileSet, params *Params) {
	if d.Tok != token.CONST {
		warn(fset, d.Pos(), "unused decl")
		return
	}

	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			warn(fset, spec.Pos(), "unused spec")
			continue
		}
		for i := range vs.Names {
			switch vs.Names[i].Name {
			case "lrPrefix":
				if str, ok := literalString(vs.Values[i], fset); ok {
					params.Prefix = str
				}
			case "lrTokenType":
				if str, ok := literalString(vs.Values[i], fset); ok {
					params.TokenType = str
				}
			case "lrTrace":
				if ident, ok := vs.Values[i].(*ast.Ident); ok {
					switch ident.Name {
					case "true":
						params.Trace = true
					case "false":
						params.Trace = false
					default:
						warn(fset, vs.Values[i].Pos(), "expected bool")
					}
				} else {
					warn(fset, vs.Names[i].Pos(), "expected bool")
				}
			default:
				warn(fset, vs.Names[i].Pos(), "unknown parameter")
			}
		}
	}
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
func Parse(path string) (*Params, []*Rule, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, nil, err
	}

	params := &Params{
		Package:   f.Name.Name,
		TokenType: "Token",
	}
	var rules []*Rule
	ast.Inspect(f, func(an ast.Node) bool {
		switch n := an.(type) {
		case *ast.GenDecl:
			processDecl(n, fset, params)
			return false // don't examine children
		case *ast.FuncDecl:
			processFunction(n, fset, &rules)
			return false // don't examine children
		}
		return true // visit children
	})

	return params, rules, nil
}
