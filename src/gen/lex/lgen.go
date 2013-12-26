package lex

import (
	"bufio"
	"io"
	"log"
	"os"
	"sort"

	"gen/codegen"
)

type BlockId int

const (
	BlockSpecial BlockId = iota
	BlockSymbol
	BlockKeyword
)

type Token struct {
	name, value string
	block       BlockId
}

// ReadTokens parses the tokens format.
func ReadTokens(r io.Reader) []*Token {
	var tokens []*Token
	var id BlockId
	s := bufio.NewScanner(r)
	s.Split(bufio.ScanWords)
	for s.Scan() {
		name := s.Text()
		if name[len(name)-1] == ':' {
			name = name[:len(name)-1]
			switch name {
			case "specials":
				id = BlockSpecial
			case "symbols":
				id = BlockSymbol
			case "keywords":
				id = BlockKeyword
			default:
				log.Fatalf("unknown block %q", name)
			}
			continue
		}

		if !s.Scan() {
			break
		}
		value := s.Text()
		tokens = append(tokens, &Token{name, value, id})
	}
	if err := s.Err(); err != nil {
		panic(err)
	}
	return tokens
}

// writeTokenIds writes the "tFoo, tBar" constant list.
func writeTokenIds(w *codegen.Writer, tokens []*Token) {
	w.Line("const (")
	for i, t := range tokens {
		if i == 0 {
			w.Linef("t%s TokenId = iota", t.name)
		} else {
			w.Linef("t%s", t.name)
		}
	}
	w.Line(")")
}

// writeTokenNames writes an array mapping TokenId integers to their
// string names.
func writeTokenNames(w *codegen.Writer, tokens []*Token) {
	w.Line("var TokNames = []string{")
	for _, t := range tokens {
		w.Linef("\"%s\",", t.value)
	}
	w.Line("}")
}

// writeTokenLookup writes a map of string names to token ids.
// E.g. "eof" => tEOF.
func writeTokenLookup(w *codegen.Writer, tokens []*Token) {
	w.Line("var TokIds = map[string]TokenId{")
	for _, t := range tokens {
		w.Linef("%q: t%s,", t.value, t.name)
	}
	w.Line("}")
}

// writeKeywords writes a map mapping keyword names to their TokenIds.
// It only does this for tokens in the "keyword" block.  This is used
// to distinguish plain identifiers ("foo") from keywords ("for").
func writeKeywords(w *codegen.Writer, tokens []*Token) {
	w.Line("var Keywords = map[string]TokenId{")
	for _, t := range tokens {
		if t.block == BlockKeyword {
			w.Linef("%q: t%s,", t.value, t.name)
		}
	}
	w.Line("}")
}

type symM struct {
	accept string
	next   map[byte]*symM
}

func (s *symM) add(input string, accept string) {
	if input == "" {
		s.accept = accept
		return
	}
	if s.next == nil {
		s.next = make(map[byte]*symM)
	}
	ns := s.next[input[0]]
	if ns == nil {
		ns = &symM{}
		s.next[input[0]] = ns
	}
	ns.add(input[1:], accept)
}

type Chars []byte

func (c Chars) Len() int           { return len(c) }
func (c Chars) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c Chars) Less(i, j int) bool { return c[i] < c[j] }

func (s *symM) writeSwitch(w *codegen.Writer, top bool) {
	if s.next != nil {
		w.Line("switch r.Next() {")

		var keys []byte
		for char := range s.next {
			keys = append(keys, char)
		}
		sort.Sort(Chars(keys))

		if top {
			w.Line("case 0: return tEOF")
		}

		for _, char := range keys {
			w.Linef("case '%c':", char)
			s.next[char].writeSwitch(w, false)
		}

		w.Linef("default:")
		w.Line("r.Back()")
		if s.accept != "" {
			w.Linef("return t%s", s.accept)
		} else {
			w.Line("// It's up to the caller to figure it out.")
			w.Line("return tNone")
		}
		w.Line("}")
	} else {
		w.Linef("return t%s", s.accept)
	}
}

// writeMachine writes out the recognizer machine, which handles
// symbols but not keywords.
func writeMachine(w *codegen.Writer, tokens []*Token) {
	var sm symM
	for _, tok := range tokens {
		if tok.block != BlockSymbol {
			continue
		}
		sm.add(tok.value, tok.name)
	}

	w.Line("func lex(r ByteReader) TokenId {")
	sm.writeSwitch(w, true)
	w.Line("}")
}

func Main(infile string, verbose bool) ([]byte, error) {
	ftokens, err := os.Open(infile)
	if err != nil {
		return nil, err
	}
	tokens := ReadTokens(ftokens)

	w := &codegen.Writer{}
	w.Line("package main")
	w.Line(`// ByteReader is the interface expected by the lex function.
type ByteReader interface {
  // Next reads another byte.  It should return 0 on EOF and panic on error.
  Next() byte
  // Back backs up by one byte.
  Back()
}
`)
	w.Line("type TokenId int")

	writeTokenIds(w, tokens)
	w.Line("")
	writeTokenNames(w, tokens)
	w.Line("")
	writeTokenLookup(w, tokens)
	w.Line("")
	writeKeywords(w, tokens)
	w.Line("")
	writeMachine(w, tokens)

	return w.Fmt()
}
