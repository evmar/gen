// Package codegen provides a Writer for generating source code.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
)

// Writer accumulates written source code.
type Writer struct {
	bytes.Buffer
}

// Line emits a line of text.
func (w *Writer) Line(text string) {
	io.WriteString(w, text+"\n")
}

// Linef emits a line of text via a fmt format string.
func (w *Writer) Linef(format string, a ...interface{}) {
	fmt.Fprintf(w, format+"\n", a...)
}

// Raw returns the raw generated source, useful for debugging.
func (w *Writer) Raw() []byte {
	return w.Bytes()
}

// Fmt returns the gofmt-formatted source.  It can return an error
// if the generated source fails to parse.
func (w *Writer) Fmt() ([]byte, error) {
	return format.Source(w.Raw())
}
