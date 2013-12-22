package lr

import (
	"strings"

)

// Params controls parameters to the generation process.
type Params struct {
	// Prefix is inserted as a prefix on all types; useful to prevent
	// inter-file conflicts.
	Prefix string
	// Package is the package name for the output.
	Package string
	// Token is the name of the type of tokens passed to the
	// generation function.
	Token string
	// Trace specifies whether to log the parse as it happens.
	Trace bool
}

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

