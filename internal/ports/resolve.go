package ports

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func Resolve(spec string, presetDir string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "top1000" {
		return spec, nil
	}
	if strings.Contains(spec, "-") {
		if strings.Contains(spec, ",") {
			return "", fmt.Errorf("invalid port: %s", spec)
		}
		return normalizeRange(spec)
	}

	parts := strings.Split(spec, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		port, err := parsePort(value)
		if err != nil {
			return "", err
		}
		normalized = append(normalized, strconv.Itoa(port))
	}
	return strings.Join(normalized, ","), nil
}

func normalizeRange(spec string) (string, error) {
	bounds := strings.SplitN(spec, "-", 2)
	if len(bounds) != 2 {
		return "", fmt.Errorf("invalid port: %s", spec)
	}
	start, err := parsePort(bounds[0])
	if err != nil {
		return "", err
	}
	end, err := parsePort(bounds[1])
	if err != nil {
		return "", err
	}
	if end < start {
		start, end = end, start
	}
	return fmt.Sprintf("%d-%d", start, end), nil
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %s", strings.TrimSpace(value))
	}
	return port, nil
}

func ResolveForConfig(spec string, configPath string) (string, error) {
	dir := filepath.Dir(configPath)
	resolved, err := Resolve(spec, dir)
	if err == nil {
		return resolved, nil
	}
	if dir != "config" {
		return Resolve(spec, "config")
	}
	return "", err
}
