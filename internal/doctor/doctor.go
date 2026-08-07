package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusFail    Status = "fail"
)

type Check struct {
	Name    string
	Status  Status
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
	checks := []Check{checkForError("config", err, "ok")}
	if err != nil {
		return checks
	}

	checks = append(checks,
		// fathom is the sole port/fingerprint engine inside the scan pipeline
		// (spec v2.0): a missing path is a blocking deployment defect, surfaced
		// here as a fail (the actual scan-time block lives in preflight).
		toolCheck("fathom", cfg.Tools.Fathom, false),
		toolCheck("nmap", cfg.Tools.Nmap, false),
		toolCheck("httpx", cfg.Tools.Httpx, true),
		toolCheck("nuclei", cfg.Tools.Nuclei, true),
		rdpscanCheck(cfg.Tools.Rdpscan),
	)

	if _, err := ports.Resolve(cfg.Scan.Ports); err != nil {
		checks = append(checks, failCheck("ports", err.Error()))
	} else {
		checks = append(checks, okCheck("ports", "ok"))
	}

	nseRules, err := config.LoadNSERulesForConfig(opts.ConfigPath)
	checks = append(checks, ruleCheck("nse rules", len(nseRules), err, true))
	tagRules, err := config.LoadTagRulesForConfig(opts.ConfigPath)
	checks = append(checks, ruleCheck("tag rules", len(tagRules), err, strings.TrimSpace(cfg.Tools.Nuclei) != ""))

	checks = append(checks,
		databaseCheck(opts.DBPath),
		writableDirCheck("reports", opts.ReportDir),
		docxtplCheck(opts.DocxRenderProject),
	)
	return checks
}

func HasFailures(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

func toolCheck(name, path string, optional bool) Check {
	if strings.TrimSpace(path) == "" {
		if optional {
			return warningCheck(name, "not configured (optional)")
		}
		return failCheck(name, "path is empty")
	}
	check := executableCheck(name, path)
	if check.Status == StatusFail {
		return check
	}
	version, err := toolVersion(path)
	if err != nil {
		return warningCheck(name, "executable but version unavailable: "+err.Error())
	}
	return okCheck(name, version)
}

// ToolVersion returns the first line of the tool's --version or -version output.
func ToolVersion(path string) (string, error) {
	return toolVersion(path)
}

func toolVersion(path string) (string, error) {
	var lastErr error
	for _, flag := range []string{"--version", "-version"} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		out, err := exec.CommandContext(ctx, path, flag).CombinedOutput()
		cancel()
		line := versionLine(string(out))
		if err == nil && line != "" {
			return line, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = errors.New("empty version output")
		}
	}
	if lastErr == nil {
		lastErr = errors.New("version unavailable")
	}
	return "", lastErr
}

// versionLine extracts the meaningful version line from a tool's --version
// output. Tools like httpx/nuclei print an ASCII banner or ANSI-colored
// "[INF] ..." log lines before the actual version, so the first line is not
// always useful. It prefers the first line that mentions a version, then
// strips ANSI escape codes and a "[INF]" log prefix; falls back to the first
// non-empty line otherwise.
func versionLine(output string) string {
	lines := strings.Split(output, "\n")
	for _, raw := range lines {
		line := stripANSI(strings.TrimSpace(raw))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "version") && (strings.Contains(lower, ":")) {
			// Strip a log-level prefix: "[INF] " (bracketed) or "INF "
			// (bare, after ANSI codes were removed from an unbracketed log line).
			line = strings.TrimPrefix(line, "[INF] ")
			line = strings.TrimPrefix(line, "INF ")
			return trimMessage(line)
		}
	}
	return firstLine(output)
}

// stripANSI removes ANSI escape sequences (color codes) from a line, e.g.
// "\x1b[34mINF\x1b[0m" → "INF".
func stripANSI(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); {
		if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '[' {
			j := i + 2
			for j < len(value) && !(value[j] >= 'A' && value[j] <= 'Z') && !(value[j] >= 'a' && value[j] <= 'z') {
				j++
			}
			if j < len(value) {
				j++ // consume the final letter
			}
			i = j
			continue
		}
		b.WriteByte(value[i])
		i++
	}
	return b.String()
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	line, _, _ := strings.Cut(value, "\n")
	return trimMessage(line)
}

func executableCheck(name, path string) Check {
	info, err := os.Stat(path)
	if err != nil {
		return failCheck(name, err.Error())
	}
	if info.IsDir() {
		return failCheck(name, "path is a directory")
	}
	if info.Mode()&0o111 == 0 {
		return failCheck(name, "not executable")
	}
	return okCheck(name, "ok")
}

func rdpscanCheck(path string) Check {
	if strings.TrimSpace(path) == "" {
		return warningCheck("rdpscan", rdpscanInstallHint())
	}
	check := executableCheck("rdpscan", path)
	if check.Status == StatusOK {
		check.Message = path
	}
	return check
}

func ruleCheck(name string, count int, err error, required bool) Check {
	if err != nil {
		if required {
			return failCheck(name, err.Error())
		}
		return warningCheck(name, err.Error())
	}
	return okCheck(name, fmt.Sprintf("%d rules", count))
}

// docxtplCheck is non-blocking because HTML export remains available.
func docxtplCheck(projectDir string) Check {
	const name = "docxtpl (docx export)"
	if projectDir == "" {
		return warningCheck(name, "not configured: DOCX export disabled, HTML export unaffected")
	}
	if _, err := os.Stat(projectDir); err != nil {
		return warningCheck(name, "tools/docx-render not found: DOCX export disabled, HTML export unaffected")
	}
	cmd := exec.Command("uv", "run", "--project", projectDir, "python", "-c", "import docxtpl")
	if out, err := cmd.CombinedOutput(); err != nil {
		return warningCheck(name, "docxtpl missing: run `uv sync --project "+projectDir+"`; DOCX export disabled, HTML export unaffected: "+trimMessage(string(out)))
	}
	return okCheck(name, "ok")
}

func trimMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 120 {
		return value[:120] + "..."
	}
	return value
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
		return failCheck(name, err.Error())
	}
	return okCheck(name, "ok")
}

func writableDirCheck(name string, path string) Check {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writableParentCheck(name, path)
		}
		return failCheck(name, err.Error())
	}
	if !info.IsDir() {
		return failCheck(name, "path is not a directory")
	}
	if err := writableDirWritable(path); err != nil {
		return failCheck(name, err.Error())
	}
	return okCheck(name, "ok")
}

func databaseCheck(path string) Check {
	if strings.TrimSpace(path) == "" {
		// 空路径直接 Open 会让 store.Open 在 cwd 创建 "?_pragma=..." 垃圾文件
		// （ISSUE-001）；无 DB 路径时数据库检查无意义，跳过。
		return okCheck("database", "not configured (no DB path provided)")
	}
	scanStore, err := store.Open(path)
	if err != nil {
		return failCheck("database", err.Error())
	}
	_ = scanStore.Close()
	return okCheck("database", "ok")
}

func checkForError(name string, err error, okMessage string) Check {
	if err != nil {
		return failCheck(name, err.Error())
	}
	return okCheck(name, okMessage)
}

func okCheck(name, message string) Check {
	return Check{Name: name, Status: StatusOK, Message: message}
}

func warningCheck(name, message string) Check {
	return Check{Name: name, Status: StatusWarning, Message: message}
}

func failCheck(name, message string) Check {
	return Check{Name: name, Status: StatusFail, Message: message}
}

func writableDirWritable(path string) error {
	test := filepath.Join(path, ".anchorscan-doctor-write-test")
	if err := os.WriteFile(test, []byte(""), 0o600); err != nil {
		return err
	}
	return os.Remove(test)
}
