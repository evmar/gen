package ll

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"regexp"
	"strings"
)

type CodeGen interface {
	IsTerminal(token string) bool
	GenMatch(token string) string
	GenExpect(token string, args string) string
}

// Pat represents a single node in a syntax list.
// E.g. in `foo A=bar ;`, there are three Pats, and the second one has
// varname "A" and rulename "bar".
type Pat struct {
	varname  string
	rulename string
	args     string
}

type Arm struct {
	oneOf   bool
	pattern []*Pat

	// list are matching conditions if part of a switch statement, or
	// nil otherwise.
	list *[]ast.Expr

	// body is the code to execute upon matching.
	body *[]ast.Stmt
}

type Rule struct {
	// arms are the possible matches that should fire this rule.
	arms []*Arm

	// internalArms are arms that start with a self-call; they're
	// for handling self recursion for the second branch of rules
	// like:
	//   expr := number
	//         | expr + number
	internalArms []*Arm
}

type FirstSet map[string]map[string]string
type PGen struct {
	cg     CodeGen
	rules  map[string]*Rule
	firsts FirstSet
}

// MustParse converts a string to an ast.Expr, panicing on failure.
func MustParse(x string) ast.Expr {
	//log.Println("parse:", x)
	e, err := parser.ParseExpr(x)
	if err != nil {
		panic(fmt.Errorf("when parsing %q: %s", x, err))
	}
	//ast.Print(nil, e)
	return e
}

// GenDecl generates a "x, y := z" statement.
func GenDecl(vars []string, expr ast.Expr) ast.Stmt {
	var lhs []ast.Expr
	for _, v := range vars {
		lhs = append(lhs, &ast.Ident{Name: v})
	}

	return &ast.AssignStmt{
		Lhs: lhs,
		Tok: token.DEFINE,
		Rhs: []ast.Expr{expr},
	}
}

func parsePattern(input string) (pattern []*Pat, oneOf bool) {
	re := regexp.MustCompile(`^(?:([^=])=)?(\S+?)(\(.*\))?$`)

	words := strings.Split(input[1:len(input)-1], " ")

	for i, word := range words {
		match := re.FindStringSubmatch(word)

		pat := &Pat{varname: match[1], rulename: match[2], args: match[3]}

		if i == 0 && pat.rulename == "oneOf" {
			oneOf = true
			continue
		}
		pattern = append(pattern, pat)
	}

	// epsilon is handled specially.
	if len(pattern) == 1 && pattern[0].rulename == "e" {
		pattern = nil
	}

	return
}

func isSyntaxCall(s ast.Stmt) (pattern string, ok bool) {
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
	pattern = e.Args[0].(*ast.BasicLit).Value
	ok = true
	return
}

func isSyntaxSwitch(s ast.Stmt) (sw *ast.SwitchStmt) {
	n, ok := s.(*ast.SwitchStmt)
	if !ok {
		return
	}
	if n.Tag == nil {
		return
	}
	if id, ok := n.Tag.(*ast.Ident); !ok || id.Name != "syntax" {
		return
	}
	return n
}

func (pg *PGen) gatherFunc(n *ast.FuncDecl) {
	if n.Body == nil || len(n.Body.List) < 1 {
		return
	}
	syntax, ok := isSyntaxCall(n.Body.List[0])
	if !ok {
		return
	}
	n.Body.List = n.Body.List[1:]

	name := n.Name.Name
	rule := pg.rules[name]
	if rule == nil {
		rule = &Rule{}
		pg.rules[name] = rule
	}

	arm := &Arm{body: &n.Body.List}
	arm.pattern, arm.oneOf = parsePattern(syntax)
	if arm.oneOf {
		panic("notimpl")
	}
	rule.arms = append(rule.arms, arm)
}

func addDefaultToSwitch(context string, n *ast.SwitchStmt) {
	expr := fmt.Sprintf(`panic(fmt.Sprintf("%s: didn't expect %%s", p.tok))`, context)
	stmt := &ast.ExprStmt{X: MustParse(expr)}
	def := &ast.CaseClause{Body: []ast.Stmt{stmt}}
	n.Body.List = append(n.Body.List, def)
}

func (pg *PGen) gatherSwitch(curfunc *ast.FuncDecl, indexInFunc int, n *ast.SwitchStmt) {
	n.Tag = MustParse("p.tok.Id")

	rulename := curfunc.Name.Name
	var rule *Rule
	if indexInFunc > 0 {
		rulename = fmt.Sprintf("%s%d", rulename, indexInFunc)
	}
	rule = pg.rules[rulename]
	if rule == nil {
		rule = &Rule{}
		pg.rules[rulename] = rule
	}

	hasDefault := false
	var internalCases []ast.Stmt
	var newBody []ast.Stmt
	for _, s := range n.Body.List {
		c := s.(*ast.CaseClause)
		arm := &Arm{list: &c.List, body: &c.Body}
		syntax := c.List[0].(*ast.BasicLit).Value
		arm.pattern, arm.oneOf = parsePattern(syntax)

		if len(arm.pattern) > 0 && arm.pattern[0].rulename == curfunc.Name.Name {
			arm.pattern = arm.pattern[1:]
			rule.internalArms = append(rule.internalArms, arm)
			internalCases = append(internalCases, c)
		} else {
			if arm.pattern == nil {
				hasDefault = true
			}
			rule.arms = append(rule.arms, arm)
			newBody = append(newBody, s)
		}
	}
	n.Body.List = newBody

	if !hasDefault {
		addDefaultToSwitch(curfunc.Name.Name, n)
	}

	if internalCases != nil {
		sw := &ast.SwitchStmt{
			Tag:  MustParse("p.tok.Id"),
			Body: &ast.BlockStmt{List: internalCases},
		}

		def := &ast.CaseClause{Body: []ast.Stmt{&ast.ReturnStmt{}}}
		sw.Body.List = append(sw.Body.List, def)

		f := &ast.ForStmt{Body: &ast.BlockStmt{List: []ast.Stmt{sw}}}
		curfunc.Body.List = append(curfunc.Body.List, f)
		//curfunc.Body.List = append(curfunc.Body.List, &ast.ReturnStmt{})
	}
}

func (pg *PGen) gatherFuncs(f *ast.File) {
	pg.rules = make(map[string]*Rule)
	var curfunc *ast.FuncDecl
	indexInFunc := 0

	ast.Inspect(f, func(an ast.Node) bool {
		switch n := an.(type) {
		case *ast.FuncDecl:
			curfunc = n
			indexInFunc = 0
			pg.gatherFunc(n)
		case *ast.SwitchStmt:
			sw := isSyntaxSwitch(n)
			if sw != nil {
				pg.gatherSwitch(curfunc, indexInFunc, n)
			}
			indexInFunc++
		}
		return true
	})
}

func (pg *PGen) dumpFirsts(fs FirstSet) {
	for name, firsts := range fs {
		log.Println(name)
		for tok, via := range firsts {
			log.Println(" ", "given", tok, "use rule", via)
		}
	}
}

func (pg *PGen) gatherFirsts() {
	firsts := make(FirstSet)

	// Initialize by grabbing the first pats from each arm of each rule.
	// Given A -> w1 w2 | B w3
	// build A -> {w1:w1, B:B}
	for name, rule := range pg.rules {
		first := firsts[name]
		if first == nil {
			first = make(map[string]string)
			firsts[name] = first
		}

		for _, arm := range rule.arms {
			if arm.pattern != nil {
				for _, pat := range arm.pattern {
					target := pat.rulename
					first[target] = target
					if !arm.oneOf {
						break
					}
				}
			}
		}
	}

	//pg.dumpFirsts(firsts)

	// Recursively expand references to nonterminals.
	// Given A -> {w1:w1, B:B}
	// build A -> {w1:w1, first(B):B}
	for changed := true; changed; {
		changed = false
		for rulename, fs := range firsts {
			for name, via := range fs {
				if pg.cg.IsTerminal(string(name)) {
					continue
				}

				other := firsts[name]
				if other == nil {
					continue
				}
				for oname := range other {
					if _, hasEntry := fs[oname]; hasEntry {
						log.Fatalf("rule %q has multiple syntax for %s", rulename, oname)
					}
					fs[oname] = via
				}
				delete(fs, name)
				changed = true
			}
		}
	}

	//pg.dumpFirsts(firsts)

	pg.firsts = firsts
}

func (pg *PGen) genArm(arm *Arm) {
	var stmts []ast.Stmt
	if !arm.oneOf {
		for _, pat := range arm.pattern {
			tok := string(pat.rulename)
			expr := MustParse(pg.cg.GenExpect(tok, pat.args))
			trace := false
			if trace {
				stmts = append(stmts, &ast.ExprStmt{
					MustParse("log.Println(\"entering\", \"" + tok + "\")")})
			}
			if pat.varname != "" {
				stmts = append(stmts, GenDecl([]string{pat.varname}, expr))
			} else {
				stmts = append(stmts, &ast.ExprStmt{expr})
			}
		}
	}

	for _, stmt := range *arm.body {
		stmts = append(stmts, stmt)
	}

	if arm.list != nil {
		var list []ast.Expr
		if arm.pattern != nil {
			for _, pat := range arm.pattern {
				tok := pat.rulename
				if fs, ok := pg.firsts[tok]; ok {
					for t := range fs {
						list = append(list, MustParse(pg.cg.GenMatch(string(t))))
					}
				} else {
					list = append(list, MustParse(pg.cg.GenMatch(string(tok))))
				}
				if !arm.oneOf {
					break
				}
			}
		}
		*arm.list = list
	}
	*arm.body = stmts
}

func Pgen(cg CodeGen, infile string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, infile, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	pg := PGen{cg: cg}
	pg.gatherFuncs(f)
	pg.gatherFirsts()

	for _, rule := range pg.rules {
		for _, arm := range rule.arms {
			pg.genArm(arm)
		}
		for _, arm := range rule.internalArms {
			pg.genArm(arm)
		}
	}

	printer.Fprint(os.Stdout, fset, f)
}
