func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n'
}

type lexer struct {
	r *bytereader.Reader
}

func NewLexer(r io.Reader) *lexer {
	return &lexer{bytereader.NewReader(bufio.NewReader(r))}
}

type TokenId int

const (
	tNone TokenId = iota
	tEOF
	tSemi
	tEquals
	tIdent
	tCode
)

type Tok struct {
	Pos  bytereader.Pos
	id   TokenId
	data string
}

func (t Tok) String() string {
	switch t.id {
	case tEOF:
		return "EOF"
	case tSemi:
		return ";"
	case tEquals:
		return "="
	case tIdent:
		return fmt.Sprintf("%q", t.data)
	case tCode:
		return fmt.Sprintf("{%q}", t.data)
	}
	panic("not reached")
}

func (t *Tok) Id() string {
	switch t.id {
	case tEOF:
		return "$"
	case tSemi:
		return ";"
	case tEquals:
		return "="
	case tIdent:
		return "id"
	case tCode:
		return "code"
	default:
		panic("unhandled tok " + t.String())
	}
}

func (r *lexer) Read(tok *Tok) {
	for isWhitespace(r.r.Next()) {
	}
	r.r.Back()

	tok.Pos = r.r.Pos
	switch r.r.Next() {
	case 0:
		tok.id = tEOF
		tok.data = ""
		return
	case ';':
		tok.id = tSemi
		tok.data = ""
		return
	case '=':
		tok.id = tEquals
		tok.data = ""
		return
	case '{':
		tok.id = tCode
		tok.data = r.ReadCode()
		return
	default:
		r.r.Back()
	}

	var buf bytes.Buffer
	for {
		b := r.r.Next()
		if isWhitespace(b) || b == 0 {
			tok.id = tIdent
			tok.data = buf.String()
			if tok.data[0] == '\'' && tok.data[len(tok.data)-1] == '\'' {
				tok.data = tok.data[1 : len(tok.data)-1]
			}
			return
		}
		buf.WriteByte(b)
	}
}

func (r *lexer) ReadCode() string {
	braces := 1
	var buf bytes.Buffer
	for {
		b := r.r.Next()
		switch b {
		case 0:
			panic("unexpected EOF while scanning code")
		case '{':
			braces++
		case '}':
			braces--
		}
		if braces == 0 {
			return buf.String()
		}
		buf.WriteByte(b)
	}
}
