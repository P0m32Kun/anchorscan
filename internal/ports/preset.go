package ports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PresetPath returns the on-disk path of a named port preset file
// (e.g. spec "highrisk" -> "<dir>/ports-highrisk.txt") relative to dir.
func PresetPath(spec, dir string) string {
	return filepath.Join(dir, fmt.Sprintf("ports-%s.txt", spec))
}

// SavePresetWithBackup writes a port preset file with a timestamped backup and
// an atomic rename, mirroring config.SaveWithBackup. The content is stored as a
// single trimmed CSV line followed by a newline.
func SavePresetWithBackup(spec, dir, content string, now time.Time) (string, error) {
	path := PresetPath(spec, dir)
	backup := path + ".bak." + now.Format("20060102-150405")
	if data, err := os.ReadFile(path); err == nil {
		if err := os.WriteFile(backup, data, 0o644); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return backup, nil
}
