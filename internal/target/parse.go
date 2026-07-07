package target

import "strings"

func Parse(input string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string

	for _, part := range strings.Split(input, ",") {
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
