package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func SaveWithBackup(path string, cfg Config, now time.Time) (string, error) {
	backup := path + ".bak." + now.Format("20060102-150405")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return "", err
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return "", err
	}
	return backup, nil
}
