package lr

import (
	"log"
	"os"
)

// Logger wraps the standard log package in an interface.
type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

// log is a global Logger that is used in all logging statements.
var traceLog = log.New(os.Stderr, "", log.Lshortfile)
