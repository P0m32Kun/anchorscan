//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestCLIEndToEndSmoke(t *testing.T) {
	root := repoRoot(t)
	ensureLab(t, root)
	waitForTCP(t, "127.0.0.1:8080", 30*time.Second)
	waitForTCP(t, "127.0.0.1:6379", 30*time.Second)

	paths := resolveToolPaths(t)
	binary := buildBinary(t, root)
	work := t.TempDir()
	configPath := writeConfig(t, work, paths)
	dbPath := filepath.Join(work, "scan.db")
	jsonPath := filepath.Join(work, "scan.json")
	htmlPath := filepath.Join(work, "scan.html")

	out := runCommand(t, root, 5*time.Minute, binary,
		"scan",
		"--config", configPath,
		"--target", "127.0.0.1",
		"--ports", "8080,6379",
		"--profile", "slow",
		"--db", dbPath,
		"--json", jsonPath,
		"--html", htmlPath,
	)

	if !strings.Contains(out, "run_id=") {
		t.Fatalf("expected run_id in output, got %q", out)
	}
	assertFileExists(t, jsonPath)
	assertFileExists(t, htmlPath)

	reportData := readReport(t, jsonPath)
	assertPortsPresent(t, reportData, 6379, 8080)

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open returned error: %v", err)
	}
	defer scanStore.Close()

	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatalf("unexpected runs: %#v", runs)
	}
}

func TestWebEndToEndSmoke(t *testing.T) {
	root := repoRoot(t)
	ensureLab(t, root)
	waitForTCP(t, "127.0.0.1:8080", 30*time.Second)
	waitForTCP(t, "127.0.0.1:6379", 30*time.Second)

	paths := resolveToolPaths(t)
	binary := buildBinary(t, root)
	work := t.TempDir()
	configPath := writeConfig(t, work, paths)
	dbPath := filepath.Join(work, "scan.db")
	listen := freeAddress(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, binary,
		"web",
		"--config", configPath,
		"--db", dbPath,
		"--listen", listen,
	)
	cmd.Dir = root
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("web start returned error: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})

	baseURL := "http://" + listen
	waitForHTTP(t, baseURL+"/", 30*time.Second)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}

	form := url.Values{
		"target":  {"127.0.0.1"},
		"ports":   {"8080,6379"},
		"profile": {"slow"},
	}
	resp, err := client.PostForm(baseURL+"/scan", form)
	if err != nil {
		t.Fatalf("PostForm returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected redirect, got %d body=%s stderr=%s", resp.StatusCode, string(body), stderr.String())
	}

	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, "/runs/") {
		t.Fatalf("expected run redirect, got %q", location)
	}
	runID := strings.TrimPrefix(location, "/runs/")

	waitForRunComplete(t, dbPath, runID, 5*time.Minute)

	reportResp, err := client.Get(baseURL + "/reports/" + runID)
	if err != nil {
		t.Fatalf("GET report returned error: %v", err)
	}
	defer reportResp.Body.Close()
	body, err := io.ReadAll(reportResp.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if reportResp.StatusCode != http.StatusOK {
		t.Fatalf("expected report 200, got %d body=%s", reportResp.StatusCode, string(body))
	}
	html := string(body)
	if !strings.Contains(strings.ToLower(html), "redis") || !strings.Contains(strings.ToLower(html), "tomcat") {
		t.Fatalf("expected redis and tomcat in report page, got %q", html)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected repo root at %s: %v", root, err)
	}
	return root
}

func ensureLab(t *testing.T, root string) {
	t.Helper()
	if labRunning(root, "anchorscan-lab-tomcat") && labRunning(root, "anchorscan-lab-redis") {
		return
	}
	startExistingLabContainers(t, root)
	if labRunning(root, "anchorscan-lab-tomcat") && labRunning(root, "anchorscan-lab-redis") {
		return
	}
	runCommand(t, root, 2*time.Minute, "docker", "compose", "-f", "docker-compose.lab.yml", "up", "-d")
}

func startExistingLabContainers(t *testing.T, root string) {
	t.Helper()
	names := existingLabContainers(t, root)
	if len(names) == 0 {
		return
	}
	args := append([]string{"start"}, names...)
	runCommand(t, root, 30*time.Second, "docker", args...)
}

func existingLabContainers(t *testing.T, root string) []string {
	t.Helper()
	out := runCommand(t, root, 30*time.Second, "docker", "ps", "-a", "--format", "{{.Names}}")
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		switch strings.TrimSpace(line) {
		case "anchorscan-lab-tomcat", "anchorscan-lab-redis":
			names = append(names, strings.TrimSpace(line))
		}
	}
	return names
}

func labRunning(root string, name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", name)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

type toolPaths struct {
	rustscan string
	nmap     string
	httpx    string
	nuclei   string
}

func resolveToolPaths(t *testing.T) toolPaths {
	t.Helper()
	return toolPaths{
		rustscan: mustFindTool(t, "ANCHORSCAN_RUSTSCAN", "rustscan", true),
		nmap:     mustFindTool(t, "ANCHORSCAN_NMAP", "nmap", true),
		httpx:    mustFindTool(t, "ANCHORSCAN_HTTPX", "httpx", false),
		nuclei:   mustFindTool(t, "ANCHORSCAN_NUCLEI", "nuclei", false),
	}
}

func mustFindTool(t *testing.T, envName string, fallback string, required bool) string {
	t.Helper()
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	path, err := exec.LookPath(fallback)
	if err == nil {
		return path
	}
	if required {
		t.Fatalf("missing required tool %s: %v", fallback, err)
	}
	return ""
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "anchorscan")
	runCommand(t, root, 2*time.Minute, "go", "build", "-o", path, "./cmd/anchorscan")
	return path
}

func writeConfig(t *testing.T, dir string, paths toolPaths) string {
	t.Helper()
	configPath := filepath.Join(dir, "config.yaml")
	content := fmt.Sprintf(`tools:
  rustscan: %s
  nmap: %s
  httpx: %s
  nuclei: %s

scan:
  ports: 8080,6379
  profile: slow

profiles:
  slow:
    host_workers: 1
    rustscan_args: ["--ulimit", "5000"]
    nmap_args: ["-T2", "--max-retries", "2"]
    httpx_args: ["-rate-limit", "20", "-threads", "5"]
    nuclei_args: ["-rate-limit", "10", "-c", "5"]
`, yamlString(paths.rustscan), yamlString(paths.nmap), yamlString(paths.httpx), yamlString(paths.nuclei))
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return configPath
}

func yamlString(value string) string {
	return strconvQuote(value)
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}

func runCommand(t *testing.T, dir string, timeout time.Duration, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s %s timed out: %s", name, strings.Join(args, " "), string(out))
	}
	if err != nil {
		t.Fatalf("%s %s returned error: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func waitForTCP(t *testing.T, address string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for tcp %s", address)
}

func waitForHTTP(t *testing.T, target string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(target)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for http %s", target)
}

func freeAddress(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func readReport(t *testing.T, path string) report.ScanReport {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var got report.ScanReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	return got
}

func assertPortsPresent(t *testing.T, data report.ScanReport, ports ...int) {
	t.Helper()
	var got []int
	for _, host := range data.Hosts {
		for _, port := range host.Ports {
			got = append(got, port.Port)
		}
	}
	sort.Ints(got)
	sort.Ints(ports)
	for _, want := range ports {
		if !containsInt(got, want) {
			t.Fatalf("expected port %d in %#v", want, got)
		}
	}
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func waitForRunComplete(t *testing.T, dbPath string, runID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		scanStore, err := store.Open(dbPath)
		if err == nil {
			run, getErr := scanStore.GetScanRun(runID)
			_ = scanStore.Close()
			if getErr == nil {
				if run.Status == "completed" {
					return
				}
				if run.Status == "failed" || run.Status == "canceled" {
					t.Fatalf("run %s finished with status %s error=%s", runID, run.Status, run.Error)
				}
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("timeout waiting for run %s", runID)
}
