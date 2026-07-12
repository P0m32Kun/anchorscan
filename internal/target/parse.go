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

func Exclude(targets []string, excludeSpec string) ([]string, error) {
	excluded, err := Parse(excludeSpec)
	if err != nil {
		return nil, err
	}
	if len(excluded) == 0 {
		return targets, nil
	}

	blocked := make(map[string]struct{}, len(excluded))
	for _, item := range excluded {
		blocked[item] = struct{}{}
	}
	out := make([]string, 0, len(targets))
	for _, item := range targets {
		if _, ok := blocked[item]; !ok {
			out = append(out, item)
		}
	}
	return out, nil
}
