package doctor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	ConfigPath        string
	DBPath            string
	ReportDir         string
	DocxRenderProject string
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
		rdpscanCheck(cfg.Tools.Rdpscan),
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
		docxtplCheck(opts.DocxRenderProject),
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

func rdpscanCheck(path string) Check {
	check := executableCheck("rdpscan", path)
	if check.OK {
		check.Message = "ok: " + path
		return check
	}
	return Check{Name: "rdpscan", OK: true, Message: rdpscanInstallHint()}
}

// docxtplCheck reports whether the DOCX sidecar can import docxtpl. It is
// non-blocking: when unavailable it stays OK=true and explains that DOCX
// export will be disabled, leaving HTML export unaffected.
func docxtplCheck(projectDir string) Check {
	const name = "docxtpl (docx export)"
	if projectDir == "" {
		return Check{Name: name, OK: true, Message: "not configured: DOCX export disabled, HTML export unaffected"}
	}
	if _, err := os.Stat(projectDir); err != nil {
		return Check{Name: name, OK: true, Message: "tools/docx-render not found: DOCX export disabled, HTML export unaffected"}
	}
	cmd := exec.Command("uv", "run", "--project", projectDir, "python", "-c", "import docxtpl")
	if out, err := cmd.CombinedOutput(); err != nil {
		return Check{Name: name, OK: true, Message: "docxtpl missing: run `uv sync --project " + projectDir + "`; DOCX export disabled, HTML export unaffected: " + trimCmdOutput(out)}
	}
	return Check{Name: name, OK: true, Message: "ok"}
}

func trimCmdOutput(out []byte) string {
	s := string(out)
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	return s
}

func rdpscanInstallHint() string {
	repo := "https://github.com/robertdavidgraham/rdpscan"
	switch runtime.GOOS {
	case "windows":
		return "not installed (optional): BlueKeep (CVE-2019-0708) detection will be skipped. Building on Windows requires MSVC + OpenSSL; consider compiling in WSL or using the Docker-based BKScan alternative. Set tools.rdpscan in config after building."
	case "darwin":
		return "not installed (optional): BlueKeep (CVE-2019-0708) detection will be skipped. Build: git clone " + repo + " && cd rdpscan && make (Homebrew openssl may be needed; see README)."
	default:
		return "not installed (optional): BlueKeep (CVE-2019-0708) detection will be skipped. Build: git clone " + repo + " && cd rdpscan && make (requires libssl-dev)."
	}
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
