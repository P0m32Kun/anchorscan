package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

var aliveNmapXML = []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`)

func TestScanOptionsIncludesTask2MetadataFields(t *testing.T) {
	type fieldCheck struct {
		name string
		typ  reflect.Type
	}

	for _, check := range []fieldCheck{
		{name: "ProfileName", typ: reflect.TypeOf("")},
		{name: "HostWorkers", typ: reflect.TypeOf(0)},
		{name: "ExtraArgs", typ: reflect.TypeOf(ToolExtraArgs{})},
		{name: "ProjectID", typ: reflect.TypeOf("")},
		{name: "ConfigSnapshot", typ: reflect.TypeOf("")},
	} {
		field, ok := reflect.TypeOf(ScanOptions{}).FieldByName(check.name)
		if !ok {
			t.Fatalf("expected ScanOptions.%s", check.name)
		}
		if field.Type != check.typ {
			t.Fatalf("expected ScanOptions.%s type %v, got %v", check.name, check.typ, field.Type)
		}
	}
}

func TestRunScanStoresFingerprintAndWritesJSONReport(t *testing.T) {
	runner := &sequenceRunner{
		outputs: [][]byte{
			aliveNmapXML,
			[]byte("192.168.1.10 -> [8080]\n"),
			[]byte(`<nmaprun><host><address addr="192.168.1.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="8080"><state state="open"/><service name="http" product="Apache Tomcat" version="9.0.65"/></port></ports></host></nmaprun>`),
			[]byte(`{"url":"http://192.168.1.10:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
			[]byte("{\"template-id\":\"tomcat-default-login\",\"matcher-name\":\"basic-auth\",\"extractor-results\":[\"admin:admin\"],\"curl-command\":\"curl -u admin:admin http://192.168.1.10:8080/manager/html\",\"info\":{\"name\":\"Tomcat Default Login\",\"severity\":\"high\"},\"matched-at\":\"http://192.168.1.10:8080\"}\n"),
		},
	}

	dbPath := filepath.Join(t.TempDir(), "scan.db")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	opts := ScanOptions{
		RunID:   "run-1",
		Targets: []string{"192.168.1.10"},
		Ports:   "8080",
		Tools: ToolPaths{
			Rustscan: "/opt/rustscan",
			Nmap:     "/opt/nmap",
			Httpx:    "/opt/httpx",
			Nuclei:   "/opt/nuclei",
		},
		JSONReportPath: reportPath,
		NSERules: map[string][]string{
			"http": {"http-tomcat-manager"},
		},
		TagRules: []TagRule{
			{
				Name:       "tomcat",
				Service:    []string{"http"},
				Product:    []string{"apache tomcat"},
				Tech:       []string{"tomcat"},
				NucleiTags: []string{"tomcat"},
				Target:     "url",
			},
		},
	}

	if err := RunScan(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	rows, err := scanStore.ListFingerprints("run-1")
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].URL != "http://192.168.1.10:8080" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	findings, err := scanStore.ListFindings("run-1")
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("unexpected findings: %#v", findings)
	}
	for _, finding := range findings {
		if finding.Source != "nuclei" {
			continue
		}
		if !strings.Contains(finding.Output, "curl-command") || !strings.Contains(finding.Output, "admin:admin") {
			t.Fatalf("expected rich nuclei evidence output, got %#v", finding)
		}
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report not written: %v", err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var decoded report.ScanReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(decoded.Hosts) != 1 || len(decoded.Hosts[0].Ports[0].Findings) != 1 {
		t.Fatalf("unexpected report: %#v", decoded)
	}
}

func TestRunScanWritesAuditArtifacts(t *testing.T) {
	dir := t.TempDir()
	scanStore := newScanStore(t)
	runner := runnerFunc(func(_ context.Context, binary string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case binary == "nmap" && strings.Contains(joined, "-sn"):
			return []byte(`<nmaprun><host><status state="up"/><address addr="127.0.0.1"/></host></nmaprun>`), nil
		case binary == "rustscan":
			return []byte("127.0.0.1 -> [80]\n"), nil
		case binary == "nmap" && strings.Contains(joined, "-sV"):
			return []byte(`<nmaprun><host><address addr="127.0.0.1"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx"/></port></ports></host></nmaprun>`), nil
		case binary == "httpx":
			return []byte("{\"url\":\"http://127.0.0.1:80\",\"status-code\":200,\"title\":\"ok\",\"tech\":[\"nginx\"]}\n"), nil
		default:
			return []byte(""), nil
		}
	})

	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID:          "run-artifacts",
		Targets:        []string{"127.0.0.1"},
		Ports:          "80",
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Httpx: "httpx"},
		ProfileName:    "normal",
		HostWorkers:    1,
		ArtifactRoot:   dir,
		JSONReportPath: filepath.Join(dir, "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}

	artifactDir := filepath.Join(dir, "run-artifacts")
	entries, err := os.ReadDir(artifactDir)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	joinedNames := strings.Join(names, "\n")
	for _, want := range []string{"nmap-alive", "rustscan-127.0.0.1-ports", "nmap-service-127.0.0.1", "httpx-127.0.0.1-80"} {
		if !strings.Contains(joinedNames, want) {
			t.Fatalf("missing artifact %q in files:\n%s", want, joinedNames)
		}
	}

	run, err := scanStore.GetScanRun("run-artifacts")
	if err != nil {
		t.Fatalf("GetScanRun returned error: %v", err)
	}
	if run.ArtifactDir != artifactDir {
		t.Fatalf("artifact dir mismatch: got %q want %q", run.ArtifactDir, artifactDir)
	}
}

type sequenceRunner struct {
	outputs [][]byte
	sleeps  []time.Duration
	index   int
}

func (s *sequenceRunner) Run(_ context.Context, _ string, _ []string) ([]byte, error) {
	if s.index < len(s.sleeps) {
		time.Sleep(s.sleeps[s.index])
	}
	out := s.outputs[s.index]
	s.index++
	return out, nil
}

type runnerFunc func(ctx context.Context, binary string, args []string) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, binary string, args []string) ([]byte, error) {
	return f(ctx, binary, args)
}

func newScanStore(t *testing.T) *store.Store {
	t.Helper()
	scanStore, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return scanStore
}

type cancelRunner struct{}

func (cancelRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	return nil, ctx.Err()
}

type cancelAfterFirstTargetRunner struct {
	cancel func()
	calls  int
}

func (r *cancelAfterFirstTargetRunner) Run(ctx context.Context, _ string, _ []string) ([]byte, error) {
	r.calls++
	if r.calls == 1 {
		r.cancel()
		<-ctx.Done()
	}
	return nil, ctx.Err()
}

type killedAfterCancelRunner struct {
	cancel func()
	calls  int
}

func (r *killedAfterCancelRunner) Run(ctx context.Context, binary string, _ []string) ([]byte, error) {
	r.calls++
	switch {
	case binary == "/opt/rustscan":
		return []byte("192.168.1.10 -> [22]\n"), nil
	case binary == "/opt/nmap":
		r.cancel()
		<-ctx.Done()
		return nil, fmt.Errorf("signal: killed")
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type recordingSequenceRunner struct {
	outputs  [][]byte
	errors   []error
	commands [][]string
	index    int
}

func (r *recordingSequenceRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{binary}, args...))
	out := r.outputs[r.index]
	err := error(nil)
	if len(r.errors) > r.index {
		err = r.errors[r.index]
	}
	r.index++
	return out, err
}

func (r *recordingSequenceRunner) hasArg(binary string, arg string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, got := range cmd[1:] {
			if got == arg {
				return true
			}
		}
	}
	return false
}

func (r *recordingSequenceRunner) hasArgs(binary string, args ...string) bool {
	for _, cmd := range r.commands {
		if len(cmd) == 0 || cmd[0] != binary {
			continue
		}
		for _, arg := range args {
			found := false
			for _, got := range cmd[1:] {
				if got == arg {
					found = true
					break
				}
			}
			if !found {
				goto next
			}
		}
		return true
	next:
	}
	return false
}

func containsLogSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

func TestToolContextLeavesZeroTimeoutWithoutDeadline(t *testing.T) {
	ctx, cancel := toolContext(context.Background(), 0)
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("zero timeout added a deadline")
	}
	ctx, cancel = toolContext(context.Background(), time.Second)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("non-zero timeout did not add a deadline")
	}
}

func TestRunScanClassifiesToolDeadlineAsFailure(t *testing.T) {
	scanStore := newScanStore(t)
	runner := runnerFunc(func(ctx context.Context, _ string, _ []string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("tool context has no deadline")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-timeout", Targets: []string{"192.0.2.10"}, Ports: "80",
		Tools: ToolPaths{Rustscan: "rustscan"}, Timeouts: ToolTimeouts{Rustscan: time.Millisecond},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunScan error = %v, want deadline exceeded", err)
	}
	run, err := scanStore.GetScanRun("run-timeout")
	if err != nil || run.Status != "failed" {
		t.Fatalf("run = %#v, %v", run, err)
	}
}

func TestRunScanClassifiesOptionalToolDeadlineAsCompletedWithErrors(t *testing.T) {
	scanStore := newScanStore(t)
	runner := runnerFunc(func(ctx context.Context, binary string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case binary == "nmap" && strings.Contains(joined, "-sn"):
			return []byte(aliveSweepXML("192.0.2.10")), nil
		case binary == "rustscan":
			return []byte("192.0.2.10 -> [22]\n"), nil
		case binary == "nmap" && strings.Contains(joined, "-sV"):
			return []byte(`<nmaprun><host><address addr="192.0.2.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`), nil
		case binary == "nmap" && strings.Contains(joined, "--script"):
			<-ctx.Done()
			return nil, ctx.Err()
		default:
			return nil, fmt.Errorf("unexpected command %s %s", binary, joined)
		}
	})
	err := RunScan(context.Background(), runner, scanStore, ScanOptions{
		RunID: "run-optional-timeout", Targets: []string{"192.0.2.10"}, Ports: "22",
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap"},
		Timeouts:       ToolTimeouts{NSE: time.Millisecond},
		NSERules:       map[string][]string{"ssh": {"ssh-hostkey"}},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	run, err := scanStore.GetScanRun("run-optional-timeout")
	if err != nil || run.Status != "completed_with_errors" {
		t.Fatalf("run = %#v, %v", run, err)
	}
	checks, err := scanStore.ListDetectionChecks("run-optional-timeout")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range checks {
		if check.Engine == "nse" && check.Status == "failed" && check.ReasonCode == "command_failed" {
			return
		}
	}
	t.Fatalf("expected failed NSE timeout check, got %#v", checks)
}

func TestRunScanClassifiesCanceledOptionalToolAsCanceled(t *testing.T) {
	scanStore := newScanStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := runnerFunc(func(toolCtx context.Context, binary string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case binary == "nmap" && strings.Contains(joined, "-sn"):
			return []byte(aliveSweepXML("192.0.2.11")), nil
		case binary == "rustscan":
			return []byte("192.0.2.11 -> [22]\n"), nil
		case binary == "nmap" && strings.Contains(joined, "-sV"):
			return []byte(`<nmaprun><host><address addr="192.0.2.11" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`), nil
		case binary == "nmap" && strings.Contains(joined, "--script"):
			cancel()
			<-toolCtx.Done()
			return nil, toolCtx.Err()
		default:
			return nil, fmt.Errorf("unexpected command %s %s", binary, joined)
		}
	})
	err := RunScan(ctx, runner, scanStore, ScanOptions{
		RunID: "run-optional-canceled", Targets: []string{"192.0.2.11"}, Ports: "22",
		Tools:          ToolPaths{Rustscan: "rustscan", Nmap: "nmap"},
		NSERules:       map[string][]string{"ssh": {"ssh-hostkey"}},
		JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunScan error = %v, want canceled", err)
	}
	run, err := scanStore.GetScanRun("run-optional-canceled")
	if err != nil || run.Status != "canceled" {
		t.Fatalf("run = %#v, %v", run, err)
	}
	checks, err := scanStore.ListDetectionChecks("run-optional-canceled")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range checks {
		if check.Engine == "nse" && check.Status == "canceled" && check.ReasonCode == "run_canceled" {
			return
		}
	}
	t.Fatalf("expected canceled NSE check, got %#v", checks)
}

type blockingRunner struct {
	mu        sync.Mutex
	active    int
	maxActive int
}

func (r *blockingRunner) Run(_ context.Context, binary string, _ []string) ([]byte, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	switch binary {
	case "/opt/rustscan":
		return []byte("127.0.0.1 -> [22]\n"), nil
	case "/opt/nmap":
		return []byte(`<nmaprun><host><address addr="127.0.0.1" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH"/></port></ports></host></nmaprun>`), nil
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type postAliveConcurrencyRunner struct {
	mu            sync.Mutex
	targets       []string
	wantActive    int
	release       chan struct{}
	released      bool
	aliveCalls    int
	rustscanCalls int
	active        int
	maxActive     int
}

func newPostAliveConcurrencyRunner(targets []string, wantActive int) *postAliveConcurrencyRunner {
	return &postAliveConcurrencyRunner{
		targets:    targets,
		wantActive: wantActive,
		release:    make(chan struct{}),
	}
}

func (r *postAliveConcurrencyRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	if binary == "/opt/nmap" && len(args) > 0 && args[0] == "-sn" {
		r.mu.Lock()
		r.aliveCalls++
		r.mu.Unlock()
		return []byte(aliveSweepXML(r.targets...)), nil
	}
	if binary != "/opt/rustscan" {
		return nil, fmt.Errorf("unexpected command %s %v", binary, args)
	}

	r.mu.Lock()
	r.rustscanCalls++
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	if r.maxActive >= r.wantActive && !r.released {
		close(r.release)
		r.released = true
	}
	r.mu.Unlock()

	select {
	case <-r.release:
	case <-time.After(time.Second):
	}

	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return []byte("127.0.0.1 -> []\n"), nil
}

type failFirstRunner struct {
	mu      sync.Mutex
	outputs [][]byte
	index   int
	failed  bool
}

func (r *failFirstRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if binary == "/opt/nmap" && len(args) > 0 && args[0] == "-sn" {
		return []byte(aliveSweepXML("192.168.1.10", "192.168.1.11")), nil
	}
	if !r.failed {
		r.failed = true
		return nil, fmt.Errorf("boom")
	}
	out := r.outputs[r.index]
	r.index++
	return out, nil
}

type failRunner struct {
	err error
}

func (r failRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	if binary == "/opt/nmap" && len(args) > 0 && args[0] == "-sn" {
		return []byte(aliveSweepXML("192.168.1.10", "192.168.1.11")), nil
	}
	return nil, r.err
}

func aliveSweepXML(targets ...string) string {
	var b strings.Builder
	b.WriteString("<nmaprun>")
	for _, target := range targets {
		b.WriteString(`<host><status state="up"/><address addr="`)
		b.WriteString(target)
		b.WriteString(`" addrtype="ipv4"/></host>`)
	}
	b.WriteString("</nmaprun>")
	return b.String()
}

type emptyPortRunner struct {
	nmapCalls int
}

func (r *emptyPortRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	switch binary {
	case "/opt/nmap":
		if len(args) > 0 && args[0] == "-sn" {
			return aliveNmapXML, nil
		}
		r.nmapCalls++
		return nil, fmt.Errorf("nmap should not run without open ports")
	case "/opt/rustscan":
		return []byte("172.22.0.7 -> []\n"), nil
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type downHostRunner struct {
	rustscanCalls int
}

func (r *downHostRunner) Run(_ context.Context, binary string, _ []string) ([]byte, error) {
	switch binary {
	case "/opt/nmap":
		return []byte(`<nmaprun><host><status state="down"/></host></nmaprun>`), nil
	case "/opt/rustscan":
		r.rustscanCalls++
		return nil, fmt.Errorf("rustscan should not run for down host")
	default:
		return nil, fmt.Errorf("unexpected binary %s", binary)
	}
}

type aliveSweepRunner struct {
	commands [][]string
}

func (r *aliveSweepRunner) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	r.commands = append(r.commands, append([]string{binary}, args...))
	switch {
	case binary == "/opt/nmap":
		return []byte(`<nmaprun><host><status state="up"/><address addr="172.22.0.1" addrtype="ipv4"/></host><host><status state="up"/><address addr="172.22.0.2" addrtype="ipv4"/></host></nmaprun>`), nil
	case binary == "/opt/rustscan" && len(args) >= 2 && args[1] == "172.22.0.1":
		return []byte("172.22.0.1 -> []\n"), nil
	case binary == "/opt/rustscan" && len(args) >= 2 && args[1] == "172.22.0.2":
		return []byte("172.22.0.2 -> []\n"), nil
	default:
		return nil, fmt.Errorf("unexpected command %s %v", binary, args)
	}
}

func containsEvent(events []store.ScanEvent, level string, stage string, target string) bool {
	for _, event := range events {
		if event.Level == level && event.Stage == stage && strings.Contains(event.Message, target) {
			return true
		}
	}
	return false
}
