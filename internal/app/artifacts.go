package app

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

func safeArtifactName(parts ...string) string {
	joined := strings.Join(parts, "-")
	joined = strings.TrimSpace(joined)
	var b strings.Builder
	lastDash := false
	for _, r := range joined {
		ok := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == ',' || r == '-'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" {
		return "artifact"
	}
	return name
}

func writeArtifact(dir, name string, data []byte) (string, error) {
	if dir == "" || len(data) == 0 {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := safeArtifactName(name)
	path := filepath.Join(dir, base)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, os.WriteFile(path, data, 0o644)
	}
	sum := sha1.Sum([]byte(base + string(data)))
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	path = filepath.Join(dir, stem+"-"+hex.EncodeToString(sum[:])[:8]+ext)
	return path, os.WriteFile(path, data, 0o644)
}

func joinIntParts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}
