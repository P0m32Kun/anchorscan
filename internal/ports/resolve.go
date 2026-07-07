package ports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Resolve(spec string, presetDir string) (string, error) {
	switch spec {
	case "full":
		return "1-65535", nil
	case "top100", "top1000":
		name := fmt.Sprintf("ports-%s.txt", spec)
		data, err := os.ReadFile(filepath.Join(presetDir, name))
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return spec, nil
	}
}
