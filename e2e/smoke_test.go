//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestCLIEndToEndMultiIPSpecifiedPorts(t *testing.T) {
	root := repoRoot(t)
	lab := ensureLab(t, root)
	waitForTCP(t, net.JoinHostPort(lab.tomcatIP, "8080"), 30*time.Second)
	waitForTCP(t, net.JoinHostPort(lab.redisIP, "6379"), 30*time.Second)

	paths := resolveToolPaths(t)
	binary := buildBinary(t, root)
	work := t.TempDir()
	configPath := writeConfig(t, root, work, paths)
	dbPath := filepath.Join(work, "scan.db")
	jsonPath := filepath.Join(work, "scan.json")

	targetValue := lab.tomcatIP + "," + lab.redisIP
	out := runCommand(t, root, 8*time.Minute, binary,
		"scan",
		"--config", configPath,
		"--target", targetValue,
		"--ports", "8080,6379",
		"--profile", "slow",
		"--db", dbPath,
		"--json", jsonPath,
	)

	if !strings.Contains(out, "run_id=") {
		t.Fatalf("expected run_id in output, got %q", out)
	}
	assertFileExists(t, jsonPath)

	reportData := readReport(t, jsonPath)
	assertHostPorts(t, reportData, map[string][]int{
		lab.tomcatIP: {8080},
		lab.redisIP:  {6379},
	})

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open returned error: %v", err)
	}

	runs, err := scanStore.ListScanRuns(10)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %#v", runs)
	}
	if runs[0].Status != "completed" && runs[0].Status != "completed_with_errors" {
		t.Fatalf("unexpected run status %#v", runs[0])
	}
	if runs[0].Target != targetValue {
		t.Fatalf("expected stored targets %q, got %q", targetValue, runs[0].Target)
	}
	if runs[0].Ports != "8080,6379" {
		t.Fatalf("expected stored ports 8080,6379, got %q", runs[0].Ports)
	}
}

func TestRealToolLabRecordsCoverageAcrossServiceFamilies(t *testing.T) {
	root := repoRoot(t)
	lab := ensureLab(t, root)
	for _, address := range []string{
		net.JoinHostPort(lab.tomcatIP, "8080"), net.JoinHostPort(lab.redisIP, "6379"),
		net.JoinHostPort(lab.mariadbIP, "3306"), net.JoinHostPort(lab.sshIP, "2222"),
		net.JoinHostPort(lab.smbIP, "445"), net.JoinHostPort(lab.unknownIP, "9099"),
	} {
		waitForTCP(t, address, 30*time.Second)
	}

	paths := resolveToolPaths(t)
	binary := buildBinary(t, root)
	work := t.TempDir()
	configPath := writeConfig(t, root, work, paths)
	dbPath := filepath.Join(work, "scan.db")
	jsonPath := filepath.Join(work, "coverage.json")
	artifactRoot := filepath.Join(work, "artifacts")
	targets := strings.Join([]string{lab.tomcatIP, lab.redisIP, lab.mariadbIP, lab.sshIP, lab.smbIP, lab.unknownIP}, ",")
	runCommand(t, root, 12*time.Minute, binary, "scan", "--config", configPath, "--target", targets,
		"--ports", "8080,6379,3306,2222,445,9099", "--profile", "slow", "--db", dbPath, "--json", jsonPath, "--artifacts", artifactRoot)

	data := readReport(t, jsonPath)
	if data.DetectionCoverage == nil || len(data.DetectionChecks) == 0 {
		t.Fatalf("PRODUCT: expected detection coverage and checks, got %#v", data)
	}
	assertHostPorts(t, data, map[string][]int{
		lab.tomcatIP: {8080}, lab.redisIP: {6379}, lab.mariadbIP: {3306},
		lab.sshIP: {2222}, lab.smbIP: {445}, lab.unknownIP: {9099},
	})

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := scanStore.ListScanRuns(1)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %#v, err=%v", runs, err)
	}
	if runs[0].Status != "completed" && runs[0].Status != "completed_with_errors" {
		t.Fatalf("unexpected run status %#v", runs[0])
	}
	checks, err := scanStore.ListDetectionChecks(runs[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDetectionCheck(checks, lab.tomcatIP, "nuclei", "completed") {
		t.Fatalf("RULE: expected completed Tomcat nuclei check, got %#v", checks)
	}
	if !hasEngineStatus(checks, "nse", "completed") {
		t.Fatalf("RULE: expected a completed real NSE check, got %#v", checks)
	}
	if !hasDetectionCheck(checks, lab.unknownIP, "nse", "skipped") || !hasDetectionCheck(checks, lab.unknownIP, "nuclei", "skipped") {
		t.Fatalf("RULE: expected unknown service rule skips, got %#v", checks)
	}
	fingerprints, err := scanStore.ListFingerprints(runs[0].RunID)
	if err != nil || !hasWebFingerprint(fingerprints, lab.tomcatIP) {
		t.Fatalf("PRODUCT: expected httpx-enriched Tomcat fingerprint, got %#v err=%v", fingerprints, err)
	}
	httpxArtifact, err := os.ReadFile(filepath.Join(artifactRoot, runs[0].RunID, "httpx-"+lab.tomcatIP+"-8080.jsonl"))
	var httpxResult struct {
		StatusCode int  `json:"status_code"`
		Failed     bool `json:"failed"`
	}
	decodeErr := json.Unmarshal(httpxArtifact, &httpxResult)
	if err != nil || decodeErr != nil || httpxResult.Failed || httpxResult.StatusCode <= 0 {
		t.Fatalf("TOOL: expected successful httpx artifact, got %q read_err=%v decode_err=%v", httpxArtifact, err, decodeErr)
	}
}

func TestRealToolLabPreservesFactsAfterOptionalFailure(t *testing.T) {
	root := repoRoot(t)
	lab := ensureLab(t, root)
	waitForTCP(t, net.JoinHostPort(lab.tomcatIP, "8080"), 30*time.Second)
	paths := resolveToolPaths(t)
	paths.nuclei = "/usr/bin/false"
	binary := buildBinary(t, root)
	work := t.TempDir()
	configPath := writeConfig(t, root, work, paths)
	dbPath := filepath.Join(work, "scan.db")
	jsonPath := filepath.Join(work, "partial.json")
	artifactRoot := filepath.Join(work, "artifacts")
	runCommand(t, root, 8*time.Minute, binary, "scan", "--config", configPath, "--target", lab.tomcatIP,
		"--ports", "8080", "--profile", "slow", "--db", dbPath, "--json", jsonPath, "--artifacts", artifactRoot)

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := scanStore.ListScanRuns(1)
	if err != nil || len(runs) != 1 || runs[0].Status != "completed_with_errors" {
		t.Fatalf("runs = %#v, err=%v", runs, err)
	}
	fingerprints, err := scanStore.ListFingerprints(runs[0].RunID)
	if err != nil || !hasWebFingerprint(fingerprints, lab.tomcatIP) {
		t.Fatalf("expected persisted fingerprint after optional failure, got %#v err=%v", fingerprints, err)
	}
	checks, err := scanStore.ListDetectionChecks(runs[0].RunID)
	if err != nil || !hasDetectionCheck(checks, lab.tomcatIP, "nuclei", "failed") {
		t.Fatalf("expected persisted failed nuclei check, got %#v err=%v", checks, err)
	}
	if data := readReport(t, jsonPath); data.DetectionCoverage == nil || len(data.Hosts) == 0 {
		t.Fatalf("expected report to retain partial facts, got %#v", data)
	}
	entries, err := os.ReadDir(filepath.Join(artifactRoot, runs[0].RunID))
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected artifacts before optional failure, got %#v err=%v", entries, err)
	}
}

func TestWebProjectScanAppliesExclusionsAndDeleteCleanup(t *testing.T) {
	root := repoRoot(t)
	lab := ensureLab(t, root)
	waitForTCP(t, net.JoinHostPort(lab.tomcatIP, "8080"), 30*time.Second)
	waitForTCP(t, net.JoinHostPort(lab.redisIP, "6379"), 30*time.Second)

	paths := resolveToolPaths(t)
	binary := buildBinary(t, root)
	work := t.TempDir()
	configPath := writeConfig(t, root, work, paths)
	dbPath := filepath.Join(work, "scan.db")
	listen := freeAddress(t)

	baseURL, stopWeb := startWebServer(t, root, binary, configPath, dbPath, listen)
	defer stopWeb()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}

	projectTargets := lab.tomcatIP + ",\n" + lab.redisIP
	createProjectResp, err := postMultipartForm(client, baseURL+"/projects", map[string]string{
		"name":        "Docker Lab",
		"description": "E2E project",
	})
	if err != nil {
		t.Fatalf("PostForm create project returned error: %v", err)
	}
	defer createProjectResp.Body.Close()
	if createProjectResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(createProjectResp.Body)
		t.Fatalf("expected create project redirect, got %d body=%s", createProjectResp.StatusCode, string(body))
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open returned error: %v", err)
	}

	projects, err := scanStore.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %#v", projects)
	}
	project := projects[0]
	zones, err := scanStore.ListProjectZones(project.ID)
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	if len(zones) == 0 {
		t.Fatalf("expected default project zones")
	}

	scanResp, err := client.PostForm(baseURL+"/scan", url.Values{
		"project_id":      {project.ID},
		"zone_id":         {zones[0].ZoneID},
		"target":          {projectTargets},
		"exclude_targets": {lab.redisIP},
		"ports":           {"8080,6379"},
		"exclude_ports":   {"6379"},
		"profile":         {"slow"},
		"access_point":    {"Docker lab switch"},
		"tester_ip":       {"172.22.0.250"},
	})
	if err != nil {
		t.Fatalf("PostForm scan returned error: %v", err)
	}
	defer scanResp.Body.Close()
	if scanResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(scanResp.Body)
		t.Fatalf("expected scan redirect, got %d body=%s", scanResp.StatusCode, string(body))
	}
	location := scanResp.Header.Get("Location")
	if !strings.HasPrefix(location, "/runs/") {
		t.Fatalf("expected run redirect, got %q", location)
	}
	runID := strings.TrimPrefix(location, "/runs/")

	waitForRunComplete(t, dbPath, runID, 8*time.Minute)

	run, err := scanStore.GetScanRun(runID)
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if run.Target != lab.tomcatIP {
		t.Fatalf("expected only non-excluded target %q, got %q", lab.tomcatIP, run.Target)
	}
	if run.Ports != "8080" {
		t.Fatalf("expected only non-excluded port 8080, got %q", run.Ports)
	}

	projectDir := filepath.Join(work, "projects", project.ID)
	reportPath := filepath.Join(projectDir, "runs", runID, "report.json")
	scanReport := readReport(t, reportPath)
	assertHostPorts(t, scanReport, map[string][]int{
		lab.tomcatIP: {8080},
	})
	if hostPorts(scanReport, lab.redisIP) != nil {
		t.Fatalf("expected excluded redis target %s to be absent from report: %#v", lab.redisIP, scanReport.Hosts)
	}

	deleteResp, err := client.PostForm(baseURL+"/projects/"+project.ID, url.Values{
		"_method": {"delete"},
	})
	if err != nil {
		t.Fatalf("PostForm delete project returned error: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(deleteResp.Body)
		t.Fatalf("expected delete redirect, got %d body=%s", deleteResp.StatusCode, string(body))
	}

	if _, err := scanStore.GetProject(project.ID); err == nil {
		t.Fatalf("expected project %s to be deleted", project.ID)
	}
	if _, err := scanStore.GetScanRun(runID); err == nil {
		t.Fatalf("expected run %s to be deleted with project", runID)
	}
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Fatalf("expected project dir %s to be removed, stat err=%v", projectDir, err)
	}
}

type labTargets struct {
	tomcatIP  string
	redisIP   string
	mariadbIP string
	sshIP     string
	smbIP     string
	unknownIP string
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

func labDir() string {
	if d := os.Getenv("SHARED_LAB_DIR"); d != "" {
		return d
	}
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return filepath.Join(usr.HomeDir, "DEV", "lab")
}

func ensureLab(t *testing.T, root string) labTargets {
	t.Helper()
	ld := labDir()
	for _, name := range labContainerNames {
		if !labRunning(root, name) {
			startExistingLabContainers(t, root)
			break
		}
	}
	for _, name := range labContainerNames {
		if !labRunning(root, name) {
			runCommand(t, ld, 2*time.Minute, "docker", "compose", "-f", filepath.Join(ld, "docker-compose.yml"), "up", "-d", "--build")
			break
		}
	}
	return labTargets{
		tomcatIP:  containerIP(t, root, "lab-tomcat"),
		redisIP:   containerIP(t, root, "lab-redis"),
		mariadbIP: containerIP(t, root, "lab-mariadb"),
		sshIP:     containerIP(t, root, "lab-ssh"),
		smbIP:     containerIP(t, root, "lab-samba"),
		unknownIP: containerIP(t, root, "lab-unknown"),
	}
}

var labContainerNames = []string{"lab-tomcat", "lab-redis", "lab-mariadb", "lab-ssh", "lab-samba", "lab-unknown"}

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
		case "lab-tomcat", "lab-redis", "lab-mariadb", "lab-ssh", "lab-samba", "lab-unknown":
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

func containerIP(t *testing.T, root string, name string) string {
	t.Helper()
	out := runCommand(t, root, 30*time.Second, "docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", name)
	ip := strings.TrimSpace(out)
	if ip == "" {
		t.Fatalf("container %s has empty IP", name)
	}
	return ip
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
		httpx:    mustFindTool(t, "ANCHORSCAN_HTTPX", "httpx", true),
		nuclei:   mustFindTool(t, "ANCHORSCAN_NUCLEI", "nuclei", true),
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
		t.Fatalf("TOOL: missing required tool %s: %v", fallback, err)
	}
	return ""
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "anchorscan")
	runCommand(t, root, 2*time.Minute, "go", "build", "-o", path, "./cmd/anchorscan")
	return path
}

func writeConfig(t *testing.T, root, dir string, paths toolPaths) string {
	t.Helper()
	for _, name := range []string{"nse.yaml", "service-tags.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, "config", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(dir, "config.yaml")
	template := filepath.Join(labDir(), "fixtures", "lab-tomcat.yaml")
	content := fmt.Sprintf(`tools:
  rustscan: %q
  nmap: %q
  httpx: %q
  nuclei: %q

scan:
  ports: 8080,6379
  profile: slow

profiles:
  slow:
    host_workers: 1
    rustscan_args: ["--ulimit", "5000"]
    nmap_args: ["-T2", "--max-retries", "2"]
    httpx_args: ["-silent"]
    nuclei_args: ["-t", %q]
`, paths.rustscan, paths.nmap, paths.httpx, paths.nuclei, template)
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return configPath
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
	t.Fatalf("ENV: timeout waiting for tcp %s", address)
}

func startWebServer(t *testing.T, root string, binary string, configPath string, dbPath string, listen string) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

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
		cancel()
		t.Fatalf("web start returned error: %v", err)
	}
	waitForHTTP(t, "http://"+listen+"/", 30*time.Second)
	return "http://" + listen, func() {
		cancel()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("web stdout:\n%s", stdout.String())
			t.Logf("web stderr:\n%s", stderr.String())
		}
	}
}

func postMultipartForm(client *http.Client, target string, fields map[string]string) (*http.Response, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return client.Do(req)
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

func assertHostPorts(t *testing.T, data report.ScanReport, expected map[string][]int) {
	t.Helper()
	if len(data.Hosts) != len(expected) {
		t.Fatalf("expected %d hosts, got %#v", len(expected), data.Hosts)
	}
	for ip, wantPorts := range expected {
		gotPorts := hostPorts(data, ip)
		if gotPorts == nil {
			t.Fatalf("expected host %s in report, got %#v", ip, data.Hosts)
		}
		sort.Ints(gotPorts)
		sort.Ints(wantPorts)
		if len(gotPorts) != len(wantPorts) {
			t.Fatalf("host %s ports mismatch: got=%v want=%v", ip, gotPorts, wantPorts)
		}
		for i := range gotPorts {
			if gotPorts[i] != wantPorts[i] {
				t.Fatalf("host %s ports mismatch: got=%v want=%v", ip, gotPorts, wantPorts)
			}
		}
	}
}

func hostPorts(data report.ScanReport, ip string) []int {
	for _, host := range data.Hosts {
		if host.IP != ip {
			continue
		}
		ports := make([]int, 0, len(host.Ports))
		for _, port := range host.Ports {
			ports = append(ports, port.Port)
		}
		return ports
	}
	return nil
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

func hasDetectionCheck(checks []store.DetectionCheck, ip, engine, status string) bool {
	for _, check := range checks {
		if check.IP == ip && check.Engine == engine && check.Status == status {
			return true
		}
	}
	return false
}

func hasEngineStatus(checks []store.DetectionCheck, engine, status string) bool {
	for _, check := range checks {
		if check.Engine == engine && check.Status == status {
			return true
		}
	}
	return false
}

func hasWebFingerprint(fingerprints []fingerprint.ServiceFingerprint, ip string) bool {
	for _, fingerprint := range fingerprints {
		if fingerprint.IP == ip && fingerprint.IsWeb && fingerprint.URL != "" {
			return true
		}
	}
	return false
}
