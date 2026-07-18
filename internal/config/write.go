package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func SaveWithBackup(path string, cfg Config, now time.Time) (string, error) {
	if _, err := cfg.Timeouts.Durations(); err != nil {
		return "", err
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return saveBytesWithBackup(path, out, now)
}

func SaveRawWithBackup(path string, raw string, now time.Time) (string, error) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return "", err
	}
	if _, err := cfg.Timeouts.Durations(); err != nil {
		return "", err
	}
	return saveBytesWithBackup(path, []byte(raw), now)
}

func saveBytesWithBackup(path string, out []byte, now time.Time) (string, error) {
	backup := path + ".bak." + now.Format("20060102-150405")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return backup, nil
}
