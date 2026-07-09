package doctor

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

type Check struct {
	Name    string
	OK      bool
	Message string
}

type Options struct {
	ConfigPath string
	DBPath     string
	ReportDir  string
}

func Run(opts Options) []Check {
	cfg, err := config.Load(opts.ConfigPath)
	checks := []Check{{Name: "config", OK: err == nil, Message: message(err, "ok")}}
	if err != nil {
		return checks
	}

	checks = append(checks,
		executableCheck("rustscan", cfg.Tools.Rustscan),
		executableCheck("nmap", cfg.Tools.Nmap),
		executableCheck("httpx", cfg.Tools.Httpx),
		executableCheck("nuclei", cfg.Tools.Nuclei),
	)

	if _, err := ports.Resolve(cfg.Scan.Ports, filepath.Dir(opts.ConfigPath)); err != nil {
		checks = append(checks, Check{Name: "ports", OK: false, Message: message(err, "ok")})
	} else {
		checks = append(checks, Check{Name: "ports", OK: true, Message: "ok"})
	}

	if _, err := config.LoadNSERules(sidecarPath(opts.ConfigPath, "nse.yaml")); err == nil || errors.Is(err, os.ErrNotExist) {
		checks = append(checks, Check{Name: "nse rules", OK: true, Message: "ok"})
	} else {
		checks = append(checks, Check{Name: "nse rules", OK: false, Message: message(err, "ok")})
	}
	if _, err := config.LoadTagRules(sidecarPath(opts.ConfigPath, "service-tags.yaml")); err == nil || errors.Is(err, os.ErrNotExist) {
		checks = append(checks, Check{Name: "tag rules", OK: true, Message: "ok"})
	} else {
		checks = append(checks, Check{Name: "tag rules", OK: false, Message: message(err, "ok")})
	}

	checks = append(checks,
		databaseCheck(opts.DBPath),
		writableDirCheck("reports", opts.ReportDir),
	)
	return checks
}

func HasFailures(checks []Check) bool {
	for _, check := range checks {
		if !check.OK {
			return true
		}
	}
	return false
}

func executableCheck(name string, path string) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{Name: name, OK: false, Message: message(err, "ok")}
	}
	if info.IsDir() {
		return Check{Name: name, OK: false, Message: "path is a directory"}
	}
	if info.Mode()&0o111 == 0 {
		return Check{Name: name, OK: false, Message: "not executable"}
	}
	return Check{Name: name, OK: true, Message: "ok"}
}

func writableParentCheck(name string, path string) Check {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		parent = "."
	}
	if err := writableDirWritable(parent); err != nil {
		return Check{Name: name, OK: false, Message: err.Error()}
	}
	return Check{Name: name, OK: true, Message: "ok"}
}

func writableDirCheck(name string, path string) Check {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writableParentCheck(name, path)
		}
		return Check{Name: name, OK: false, Message: message(err, "ok")}
	}
	if !info.IsDir() {
		return Check{Name: name, OK: false, Message: "path is not a directory"}
	}
	if err := writableDirWritable(path); err != nil {
		return Check{Name: name, OK: false, Message: message(err, "ok")}
	}
	return Check{Name: name, OK: true, Message: "ok"}
}

func databaseCheck(path string) Check {
	if check := writableParentCheck("database", path); !check.OK {
		return check
	}
	scanStore, err := store.Open(path)
	if err != nil {
		return Check{Name: "database", OK: false, Message: err.Error()}
	}
	_ = scanStore.Close()
	return Check{Name: "database", OK: true, Message: "ok"}
}

func message(err error, ok string) string {
	if err == nil {
		return ok
	}
	return err.Error()
}

func sidecarPath(configPath string, fileName string) string {
	return filepath.Join(filepath.Dir(configPath), fileName)
}

func writableDirWritable(path string) error {
	test := filepath.Join(path, ".anchorscan-doctor-write-test")
	if err := os.WriteFile(test, []byte(""), 0o600); err != nil {
		return err
	}
	return os.Remove(test)
}
