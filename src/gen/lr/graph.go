package lr

import (
	"os"
	"strings"
	
	"gen/codegen"
)

// Graph prints a graphviz graph of a parser state machine.
func Graph(grammar *Grammar, actions ActionTable) {
	ruleIds := make(map[*Rule]int)
	for i, rule := range grammar.rules {
		ruleIds[rule] = i
	}
	
	w := &codegen.Writer{}

	w.Line("digraph G {")
	w.Line("node [fontsize=10, shape=box, height=0.25]")
	w.Line("edge [fontsize=10]")
	for i, row := range actions {
		reduces := make(map[int][]string)
		for in, action := range row {
			switch a := action.(type) {
			case Shift:
				w.Linef("s%d -> s%d [label=%q]", i, a.state, in)
			case Reduce:
				target := ruleIds[a.rule]
				reduces[target] = append(reduces[target], in)
			}
		}
		for target, ins := range reduces {
			w.Linef("s%d -> s%d [label=%q, constraint=false]", i, target, strings.Join(ins, " "))
		}
	}
	w.Line("}")
	
	_, err := os.Stdout.Write(w.Raw())
	if err != nil {
		panic(err)
	}
}
