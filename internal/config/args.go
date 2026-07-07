package config

import "fmt"

func SplitArgs(input string) ([]string, error) {
	var args []string
	var current []rune
	var quote rune
	escaped := false

	flush := func() {
		if len(current) > 0 {
			args = append(args, string(current))
			current = nil
		}
	}

	for _, r := range input {
		switch {
		case escaped:
			current = append(current, r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current = append(current, r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current = append(current, r)
		}
	}
	if escaped {
		current = append(current, '\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	flush()
	return args, nil
}
