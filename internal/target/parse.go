package target

import "strings"

func Parse(input string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string

	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	for _, part := range strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == '\n'
	}) {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	return out, nil
}
