package ports

import (
	"fmt"
	"path/filepath"
	"slices"
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

func ExcludeForConfig(portSpec string, excludeSpec string, configPath string) (string, error) {
	if strings.TrimSpace(portSpec) == "top1000" && strings.TrimSpace(excludeSpec) != "" {
		var err error
		portSpec, err = LoadPresetForConfig("top1000", configPath)
		if err != nil {
			return "", err
		}
	}
	return excludePorts(portSpec, excludeSpec)
}

func excludePorts(portSpec string, excludeSpec string) (string, error) {
	portSpec = strings.TrimSpace(portSpec)
	excludeSpec = strings.TrimSpace(excludeSpec)
	if portSpec == "" || excludeSpec == "" {
		return portSpec, nil
	}
	portsToUse, err := expandPortSpec(portSpec)
	if err != nil {
		return "", err
	}
	portsToDrop, err := expandPortSpec(excludeSpec)
	if err != nil {
		return "", err
	}
	blocked := map[int]struct{}{}
	for _, port := range portsToDrop {
		blocked[port] = struct{}{}
	}
	filtered := make([]int, 0, len(portsToUse))
	for _, port := range portsToUse {
		if _, ok := blocked[port]; !ok {
			filtered = append(filtered, port)
		}
	}
	return compressPorts(filtered), nil
}

func expandPortSpec(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var out []int
	seen := map[int]struct{}{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid port range: %s", part)
			}
			start, err := parsePortNumber(bounds[0])
			if err != nil {
				return nil, err
			}
			end, err := parsePortNumber(bounds[1])
			if err != nil {
				return nil, err
			}
			if end < start {
				start, end = end, start
			}
			for port := start; port <= end; port++ {
				if _, ok := seen[port]; !ok {
					seen[port] = struct{}{}
					out = append(out, port)
				}
			}
			continue
		}
		port, err := parsePortNumber(part)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[port]; !ok {
			seen[port] = struct{}{}
			out = append(out, port)
		}
	}
	slices.Sort(out)
	return out, nil
}

func parsePortNumber(value string) (int, error) {
	var port int
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &port)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %s", value)
	}
	return port, nil
}

func compressPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, fmt.Sprintf("%d", port))
	}
	return strings.Join(parts, ",")
}
