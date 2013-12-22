package lr

import (
	"fmt"
	"strings"
)

// Terminology:
// Grammar is a collection of Rules, which are statements like
//   expr := term + term
// in that, "+" is a terminal, while "expr"/"term" are nonterminals,
// and all are called symbols.

const middot = "\u00b7"

// SymbolSet is a set of symbols (strings).
type SymbolSet map[string]bool

func (ss SymbolSet) Add(s string)      { ss[s] = true }
func (ss SymbolSet) Has(s string) bool { return ss[s] }
func (ss SymbolSet) Merge(other SymbolSet) bool {
	l := len(ss)
	for k := range other {
		ss[k] = true
	}
	return len(ss) != l
}

// SymbolMap is a map of symbols to SymbolSets.
type SymbolMap map[string]SymbolSet

func (sm SymbolMap) Dump(log Logger, label string) {
	log.Println(label + ":")
	for sym, set := range sm {
		var setStr string
		for s := range set {
			if len(setStr) > 0 {
				setStr += " "
			}
			setStr += s
		}
		log.Printf("  %s: %s\n", sym, setStr)
	}
}

// Rule is the type of grammar rules.
// For example, consider
//   exp *Expr = A=num + B=num { return A+B } ;
type Rule struct {
	// The rule name; "exp" in the above.
	symbol  string
	// The rule type; "*Expr" in the above.
	typ     string
	// The pattern of symbols; ["num", "+", "num"] in the above.
	pattern []string
	// The pattern of variable names; ["A", "", "B"] in the above.
	vars    []string
	// The code to run on matching; "return A+B" in the above.
	code    string
}

func (r *Rule) Show(arrow string, mark int) string {
	str := fmt.Sprintf("%s %s ", r.symbol, arrow)
	for i, pat := range r.pattern {
		if i > 0 {
			str += " "
		}
		if i == mark {
			str += middot + " "
		}
		str += pat
	}
	if mark == len(r.pattern) {
		str += " " + middot
	}
	return str
}

// Grammar is a collection of rules.
type Grammar struct {
	rules        []*Rule
	symbols      SymbolSet
	terminals    SymbolSet
	nonterminals SymbolSet
}

// CollectSymbols walks all the rules to collect all symbols and label
// them terminal or not based on whether they have any productions.
func (g *Grammar) CollectSymbols(trace Logger) {
	g.terminals = make(SymbolSet)
	g.nonterminals = make(SymbolSet)
	g.symbols = make(SymbolSet)
	for _, rule := range g.rules {
		g.nonterminals.Add(rule.symbol)
		g.symbols.Add(rule.symbol)
	}
	for _, rule := range g.rules {
		for _, sym := range rule.pattern {
			g.symbols.Add(sym)
			if !g.nonterminals.Has(sym) {
				g.terminals.Add(sym)
			}
		}
	}

	var terms []string
	for term := range g.terminals {
		terms = append(terms, term)
	}
	if trace != nil {
		trace.Printf("terminals: %s\n", strings.Join(terms, " "))
	}
}

// First computes the "first" set: for each symbol, the first terminals
// in all its expansions.
func (g *Grammar) First(trace Logger) SymbolMap {
	g.CollectSymbols(trace)

	first := make(SymbolMap)

	// Initialize: terminals point to themselves.
	for sym := range g.terminals {
		first[sym] = make(SymbolSet)
		first[sym].Add(sym)
	}

	// Fill with grammar's first outputs.
	for _, rule := range g.rules {
		set := first[rule.symbol]
		if set == nil {
			set = make(SymbolSet)
			first[rule.symbol] = set
		}
		set.Add(rule.pattern[0])
	}

	// Iterate until stable.
	// (If you have E -> A and A -> x, you need to iterate to get E -> x.)
	for changed := true; changed; {
		changed = false
		for _, set := range first {
			for symbol := range set {
				if set.Merge(first[symbol]) {
					changed = true
				}
			}
		}
	}

	return first
}

// Follow computes the "follow" set: the set of symbols that can occur
// after a given symbol.
func (g *Grammar) Follow(first SymbolMap) SymbolMap {
	follow := make(SymbolMap)
	init := make(SymbolSet)
	init.Add("$")  // TODO: better EOF handling.
	follow[g.rules[0].symbol] = init

	// TODO: this can be optimized.
	for changed := true; changed; {
		changed = false
		for _, rule := range g.rules {
			for i, patSym := range rule.pattern {
				set := follow[patSym]
				if set == nil {
					set = make(SymbolSet)
					follow[patSym] = set
				}
				if i+1 < len(rule.pattern) {
					nextSym := rule.pattern[i+1]
					// TODO: I originally wrote this and it worked -- why?
					// It didn't even make use of first?
					// set.Add(nextSym)
					if set.Merge(first[nextSym]) {
						changed = true
					}
				} else {
					if set.Merge(follow[rule.symbol]) {
						changed = true
					}
				}
			}
		}
	}
	return follow
}
