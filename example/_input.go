package example

const lrPrefix = "ex"

func start() *Grammar {
	syntax(`A=rules`)
	return &Grammar{rules: A}
}

func rules() []*Rule {
	syntax(`R=rules S=rule`)
	for _, r := range S {
		R = append(R, r)
	}
	return R

	syntax(`rule`)
	// use default behavior
}

func rule() []*Rule {
	syntax(`S=id T=id = P=patterns ;`)
	symbol := S.data
	typ := T.data
	patterns := P
	var rules []*Rule
	for _, pattern := range patterns {
		code := pattern[len(pattern)-1]
		pattern = pattern[0 : len(pattern)-1]
		vars := make([]string, len(pattern))
		for i, pat := range pattern {
			if len(pat) > 2 && pat[0] != '\'' && pat[1] == '=' {
				vars[i] = pat[0:1]
				pattern[i] = pat[2:]
			}
		}
		rule := &Rule{
			symbol:  symbol,
			typ:     typ,
			pattern: pattern,
			vars:    vars,
			code:    code,
		}
		rules = append(rules, rule)
	}
	return rules
}

func patterns() [][]string {
	syntax(`P=patterns C=patcode`)
	return append(P, C)

	syntax(`P=patcode`)
	return [][]string{P}
}

func patcode() []string {
	syntax(`P=pattern C=code`)
	return append(P, C.data)
}

func pattern() []string {
	syntax(`P=pattern T=id`)
	return append(P, T.data)

	syntax(`T=id`)
	return []string{T.data}
}
