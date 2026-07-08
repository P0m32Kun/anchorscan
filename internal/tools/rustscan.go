package tools

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var openPortPattern = regexp.MustCompile(`\b(\d+)\b`)
var greppablePattern = regexp.MustCompile(`\[(.*?)\]`)

func DiscoverPorts(ctx context.Context, runner Runner, binaryPath string, target string, ports string, extraArgs []string) ([]int, error) {
	args := []string{"-a", target}
	if strings.Contains(ports, "-") && !strings.Contains(ports, ",") {
		args = append(args, "--range", ports)
	} else {
		args = append(args, "--ports", ports)
	}
	args = append(args, "-g", "--no-banner")
	args = append(args, extraArgs...)

	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return nil, withOutputError(err, out)
	}

	matches := extractPortMatches(string(out))
	seen := map[int]struct{}{}
	var found []int
	for _, match := range matches {
		port, err := strconv.Atoi(match)
		if err != nil {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		found = append(found, port)
	}

	sort.Ints(found)
	return found, nil
}

func extractPortMatches(output string) []string {
	if match := greppablePattern.FindStringSubmatch(output); len(match) == 2 {
		parts := strings.Split(match[1], ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			value := strings.TrimSpace(part)
			if value != "" {
				out = append(out, value)
			}
		}
		return out
	}
	return openPortPattern.FindAllString(output, -1)
}
