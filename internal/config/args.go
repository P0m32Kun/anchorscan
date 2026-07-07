package config

import "fmt"

func SplitArgs(input string) ([]string, error) {
	var args []string
	var current []rune
	var quote rune
	escaped := false
	tokenStarted := false

	flush := func() {
		if tokenStarted {
			args = append(args, string(current))
			current = nil
			tokenStarted = false
		}
	}

	for _, r := range input {
		switch {
		case escaped:
			current = append(current, r)
			tokenStarted = true
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current = append(current, r)
				tokenStarted = true
			}
		case r == '\'' || r == '"':
			quote = r
			tokenStarted = true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current = append(current, r)
			tokenStarted = true
		}
	}
	if escaped {
		current = append(current, '\\')
		tokenStarted = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	flush()
	return args, nil
}
