package lr

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"gen/codegen"
)

// Action is an entry in the action table, indicating which way to transition
// the state machine.
type Action interface{}

// Shift is an action that means "accept the next token and change states".
type Shift struct {
	state int
}

// Reduce is an action that means "pop up the stack based on a rule".
// Reducing to the root rule means the input is accepted.
type Reduce struct {
	rule *Rule
}

// ActionTable maps parser states to rows; each row maps tokens to actions.
type ActionTable []map[string]Action

func (at ActionTable) Dump(log Logger) {
	log.Println("parsing table:")
	for i, actions := range at {
		line := fmt.Sprintf("%2d", i)
		for term, action := range actions {
			line += fmt.Sprintf(" %s:%s", term, action)
		}
		log.Println(line)
	}
}

// Item is a partially-parsed production.
type Item struct {
	rule *Rule
	// pos represents the character offset that the dot is to the left of,
	// representing a parse up to that character.  If pos == len(rule.pattern)
	// the Item represents a dot to the right of all characters.
	pos int
}

// NextSym returns the next symbol the Item would match and whether
// the Item is at the end of its pattern.
func (i Item) NextSym() (sym string, end bool) {
	if i.pos == len(i.rule.pattern) {
		return "", true
	}
	return i.rule.pattern[i.pos], false
}

// ItemSet is a set of Items.
type ItemSet map[Item]bool

func (is ItemSet) Add(it Item)      { is[it] = true }
func (is ItemSet) Has(it Item) bool { return is[it] }
func (is ItemSet) Empty() bool      { return len(is) == 0 }
func (is ItemSet) Equals(os ItemSet) bool {
	// This isn't generic set equality, but rather relies on the maps
	// only containing true values.
	if len(is) != len(os) {
		return false
	}
	for k := range is {
		if !os[k] {
			return false
		}
	}
	return true
}

func (is ItemSet) Dump(log Logger) {
	for item := range is {
		log.Println(" ", item.rule.Show("->", item.pos))
	}
}

// Closure computes the closure of an ItemSet.
// For example, if the set contains
//    x -> a.b
// then the closure should also contain all expansions of b.
func (is ItemSet) Closure(grammar *Grammar) {
	added := make(map[string]bool)

	for changed := true; changed; {
		changed = false
		for item := range is {
			// Given an item like x -> a.b, grab b.
			sym, end := item.NextSym()
			if end {
				continue
			}
			// If we haven't yet added b, find its expansions in the grammar.
			if !added[sym] {
				for _, rule := range grammar.rules {
					if rule.symbol == sym {
						is.Add(Item{rule, 0})
						changed = true
						added[sym] = true
					}
				}
			}
		}
	}

	// TODO: prune nonkernel items?
}

// Goto computes the resulting set of states given an input token.
func (is ItemSet) Goto(grammar *Grammar, x string) ItemSet {
	out := make(ItemSet)
	for item := range is {
		if sym, end := item.NextSym(); !end && sym == x {
			out.Add(Item{item.rule, item.pos + 1})
		}
	}
	out.Closure(grammar)
	return out
}

func ComputeActions(grammar *Grammar, trace Logger) ActionTable {
	first := grammar.First(trace)
	follow := grammar.Follow(first)
	if trace != nil {
		follow.Dump(trace, "follow set")
	}

	var allActions ActionTable

	states := []ItemSet{
		ItemSet{Item{grammar.rules[0], 0}: true},
	}
	states[0].Closure(grammar)

	// Construct the parsing states list by computing goto() for each
	// state and terminal.
	for i := 0; i < len(states); i++ {
		set := states[i]
		actions := make(map[string]Action)
		allActions = append(allActions, actions)

		for term := range grammar.symbols {
			c := set.Goto(grammar, term)
			if c.Empty() {
				continue
			}

			// Save this set if new.
			id := -1
			for j, oset := range states {
				if c.Equals(oset) {
					id = j
					break
				}
			}
			if id == -1 {
				states = append(states, c)
				id = len(states) - 1
			}

			actions[term] = Shift{state: id}
		}
	}

	// Add a reduce action for all items that have consumed the full rule.
	for i, set := range states {
		actions := allActions[i]
		for item := range set {
			if _, end := item.NextSym(); !end {
				// Still more terminals on this item.
				continue
			}

			f := follow[item.rule.symbol]
			for term := range f {
				if actions[term] != nil {
					// TODO: don't use traceLog
					traceLog.Println("reduce conflict!")
					set.Dump(traceLog)
					traceLog.Printf("in state %d on input %s, %#v vs %v", i, term, actions[term], item.rule)
				}
				actions[term] = Reduce{rule: item.rule}
			}
		}
	}

	if trace != nil {
		for i, set := range states {
			trace.Printf("set %d:\n", i)
			set.Dump(trace)
		}
	}

	return allActions
}

func writeTables(w *codegen.Writer, prefix string, grammar *Grammar, table ActionTable) {
	types := make(map[string]string)
	for _, rule := range grammar.rules {
		types[rule.symbol] = rule.typ
	}

	ruleIds := make(map[*Rule]int)

	w.Linef(`var %sRules = []*%sRule{`, prefix, prefix)
	for i, rule := range grammar.rules {
		ruleIds[rule] = i
		w.Linef(`{%q, %#v,`, rule.symbol, rule.pattern)
		if rule.code != "" {
			w.Line("func(data []interface{}) interface{} {")
			for j, varname := range rule.vars {
				if varname != "" {
					typ := types[rule.pattern[j]]
					if typ == "" {
						typ = "Tok"
					}
					w.Linef("%s := data[%d].(%s)", varname, j, typ)
				}
			}
			w.Line(strings.Trim(rule.code, " \t\n"))
			w.Line("},")
		} else {
			w.Line("nil,")
		}
		w.Line(`},`)
	}
	w.Line(`}`)

	w.Line("")

	w.Linef(`var %sActions = %sActionTable{`, prefix, prefix)
	for _, state := range table {
		w.Line(`{`)
		var keys []string
		for k := range state {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, tok := range keys {
			action := state[tok]
			var str string
			switch a := action.(type) {
			case Shift:
				str = fmt.Sprintf("%d", a.state)
			case Reduce:
				str = fmt.Sprintf("%d", -ruleIds[a.rule])
			default:
				panic("unhandled case")
			}
			w.Linef(`%q: %s,`, tok, str)
		}
		w.Line(`},`)
	}
	w.Line(`}`)
}

func Main(infile string, verbose bool) ([]byte, error) {
	var trace Logger
	if verbose {
		trace = traceLog
	}

	params, rules, err := Parse(infile)
	if err != nil {
		return nil, err
	}

	if trace != nil {
		trace.Println("loaded rule table")
		for i, rule := range rules {
			trace.Printf("  %d: %s\n", i, rule.Show("->", -1))
		}
	}

	g := &Grammar{rules:rules}
	actions := ComputeActions(g, trace)

	// Graph(g, actions)
	// return

	w := &codegen.Writer{}
	tmpl := template.Must(template.New("parse").Parse(
		strings.Replace(parseTemplate, "$", params.Prefix, -1)))
	tmpl.Execute(w, params)

	w.Line("// Result returns the final result of a successful parse.")
	w.Linef("func (p *%sParser) Result() %s {", params.Prefix, g.rules[0].typ)
	w.Linef("return p.data[0].(%s)", g.rules[0].typ)
	w.Line("}")

	writeTables(w, params.Prefix, g, actions)

	code, err := w.Fmt()
	if err != nil {
		return nil, fmt.Errorf("error formatting code: %s\ncode: %s\n", err, w.Raw())
	}
	return code, nil
}
