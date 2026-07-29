package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestNormalizeToolOutput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text unchanged",
			in:   "hello\nworld\n",
			want: "hello\nworld",
		},
		{
			name: "ansi escape preserved",
			in:   "\x1b[31mred\x1b[0m\x1b[1mbold\x1b[0m",
			want: "\x1b[31mred\x1b[0m\x1b[1mbold\x1b[0m",
		},
		{
			name: "carriage return keeps final state",
			in:   "progress 0%\rprogress 50%\rprogress 100%",
			want: "progress 100%",
		},
		{
			name: "ansi and cr combined",
			in:   "\x1b[2K\rdone",
			want: "done",
		},
		{
			name: "literal ansi residue preserved",
			in:   "[[34mINF[0m] running",
			want: "[[34mINF[0m] running",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeToolOutput(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeToolOutput(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEmitToolNormalizesOutput(t *testing.T) {
	st := newToolRunStore(t)
	emitTool(ToolRunOptions{RunID: "run-output"}, st, "info", "nuclei", "%s", "[[34mINF[0m] running")

	events, err := st.ListScanEvents("run-output", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Message != "[[34mINF[0m] running" {
		t.Fatalf("events = %#v", events)
	}
}

type toolRunnerFunc func(binary string, args []string) ([]byte, error)

func (f toolRunnerFunc) Run(_ context.Context, binary string, args []string) ([]byte, error) {
	return f(binary, args)
}

func newToolRunStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "scans.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestRunToolRustscanSavesOpenPorts(t *testing.T) {
	st := newToolRunStore(t)
	jsonPath := filepath.Join(t.TempDir(), "report.json")
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte("[80,443]"), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-rustscan", Tool: "rustscan", Target: "192.0.2.10", Ports: "80,443", Tools: ToolPaths{Rustscan: "rustscan"}, JSONReportPath: jsonPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	fps, err := st.ListFingerprints("run-rustscan")
	if err != nil {
		t.Fatal(err)
	}
	if len(fps) != 2 || fps[0].Port != 80 || fps[1].Port != 443 {
		t.Fatalf("fingerprints = %#v", fps)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatal(err)
	}
}

func TestRunToolNmapServiceSavesFingerprints(t *testing.T) {
	st := newToolRunStore(t)
	xml := `<nmaprun><host><address addr="192.0.2.10"/><ports><port protocol="tcp" portid="22"><state state="open"/><service name="ssh" product="OpenSSH" version="9.6"/></port></ports></host></nmaprun>`
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) { return []byte(xml), nil })

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-nmap", Tool: "nmap", Mode: "service", Target: "192.0.2.10", Ports: "22", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fps, err := st.ListFingerprints("run-nmap")
	if err != nil {
		t.Fatal(err)
	}
	if len(fps) != 1 || fps[0].Service != "ssh" || fps[0].Product != "OpenSSH" {
		t.Fatalf("fingerprints = %#v", fps)
	}
}

func TestRunToolNmapServiceSavesManualReviewFindings(t *testing.T) {
	st := newToolRunStore(t)
	xml := `<nmaprun><host><address addr="192.0.2.10"/><ports><port protocol="tcp" portid="3389"><state state="open"/><service name="ms-wbt-server" product="Microsoft Terminal Services"/></port></ports></host></nmaprun>`
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) { return []byte(xml), nil })

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-nmap-bluekeep", Tool: "nmap", Mode: "service", Target: "192.0.2.10", Ports: "3389", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	findings, err := st.ListFindings("run-nmap-bluekeep")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Source != "manual-review" || findings[0].ID != "manual-review:CVE-2019-0708" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestRunToolNmapAliveSavesInfoFinding(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-alive", Tool: "nmap", Mode: "alive", Target: "192.0.2.10", Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	findings, err := st.ListFindings("run-alive")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ID != "host-alive" || !strings.Contains(findings[0].Summary, "alive") {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestRunToolHttpxSavesWebFingerprint(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte(`{"url":"http://192.0.2.10:8080","status-code":200,"title":"Lab","tech":["nginx"]}` + "\n"), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-httpx", Tool: "httpx", URL: "http://192.0.2.10:8080", Tools: ToolPaths{Httpx: "httpx"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fps, err := st.ListFingerprints("run-httpx")
	if err != nil {
		t.Fatal(err)
	}
	if len(fps) != 1 || !fps[0].IsWeb || fps[0].Port != 8080 || fps[0].URL != "http://192.0.2.10:8080" {
		t.Fatalf("fingerprints = %#v", fps)
	}
}

func TestRunToolNucleiSavesFindings(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) {
		return []byte(`{"template-id":"redis-default-logins","ip":"192.0.2.10","port":"6379","info":{"name":"Redis Default Login","severity":"high"},"matched-at":"192.0.2.10:6379"}` + "\n"), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-nuclei", Tool: "nuclei", URL: "http://192.0.2.10:8080", Tags: []string{"tomcat"}, Tools: ToolPaths{Nuclei: "nuclei"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}

	findings, err := st.ListFindings("run-nuclei")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Source != "nuclei" || findings[0].ID != "redis-default-logins" || findings[0].Severity != "high" || findings[0].IP != "192.0.2.10" || findings[0].Port != 6379 || findings[0].Target != "192.0.2.10:6379" {
		t.Fatalf("findings = %#v", findings)
	}
}

func TestRunToolNucleiTemplateUsesExplicitTemplate(t *testing.T) {
	st := newToolRunStore(t)
	var gotArgs []string
	runner := toolRunnerFunc(func(_ string, args []string) ([]byte, error) {
		gotArgs = append([]string(nil), args...)
		return nil, nil
	})

	if err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-nuclei-template", Tool: "nuclei", URL: "http://192.0.2.10:8080", Template: "cves/demo.yaml",
		Tools: ToolPaths{Nuclei: "nuclei"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(gotArgs, " "); got != "-target http://192.0.2.10:8080 -t cves/demo.yaml -jsonl" {
		t.Fatalf("nuclei args = %q", got)
	}
}

func TestRunToolRecordsRawArgsAuditAndWarning(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, args []string) ([]byte, error) {
		return []byte(`{"template-id":"demo","ip":"192.0.2.10","port":"8080","info":{"name":"demo","severity":"info"},"matched-at":"http://192.0.2.10:8080"}` + "\n"), nil
	})

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-nuclei-raw", Tool: "nuclei", URL: "http://192.0.2.10:8080", Tags: []string{"demo"},
		Tools: ToolPaths{Nuclei: "nuclei"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
		ExtraArgs: ToolExtraArgs{Nuclei: []string{"-rate-limit", "1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	run, err := st.GetScanRun("run-nuclei-raw")
	if err != nil {
		t.Fatal(err)
	}
	if run.Target != "http://192.0.2.10:8080" {
		t.Fatalf("Target = %q, want raw target not args", run.Target)
	}
	if !strings.Contains(run.ConfigSnapshot, `"extra_args"`) || !strings.Contains(run.ConfigSnapshot, `"-rate-limit"`) {
		t.Fatalf("ConfigSnapshot missing extra_args audit: %s", run.ConfigSnapshot)
	}
	events, err := st.ListScanEvents("run-nuclei-raw", 100)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, ev := range events {
		if strings.Contains(ev.Message, "raw tool arguments supplied") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing raw-args warning in events: %#v", events)
	}
}

func TestRunToolNativeArgsDoNotLeakIntoTargetField(t *testing.T) {
	st := newToolRunStore(t)
	runner := toolRunnerFunc(func(_ string, _ []string) ([]byte, error) { return []byte("ok\n"), nil })

	err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-native", Tool: "nmap", UseNativeArgs: true, NativeArgs: []string{"--version"},
		Tools: ToolPaths{Nmap: "nmap"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.GetScanRun("run-native")
	if err != nil {
		t.Fatal(err)
	}
	if run.Target != "" {
		t.Fatalf("native Target = %q, want empty to avoid leaking raw args", run.Target)
	}
	if !strings.Contains(run.ConfigSnapshot, `"native_args"`) || !strings.Contains(run.ConfigSnapshot, `"--version"`) {
		t.Fatalf("ConfigSnapshot missing native_args audit: %s", run.ConfigSnapshot)
	}
	findings, err := st.ListFindings("run-native")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || strings.Contains(findings[0].Target, "--version") {
		t.Fatalf("finding target leaks native args: %#v", findings)
	}
}

func TestRunToolAppliesConfiguredToolTimeout(t *testing.T) {
	for _, test := range []struct {
		tool     string
		timeouts ToolTimeouts
	}{
		{tool: "rustscan", timeouts: ToolTimeouts{Rustscan: time.Millisecond}},
		{tool: "nmap", timeouts: ToolTimeouts{Nmap: time.Millisecond}},
		{tool: "httpx", timeouts: ToolTimeouts{Httpx: time.Millisecond}},
		{tool: "nuclei", timeouts: ToolTimeouts{Nuclei: time.Millisecond}},
	} {
		t.Run(test.tool, func(t *testing.T) {
			st := newToolRunStore(t)
			runner := runnerFunc(func(ctx context.Context, _ string, _ []string) ([]byte, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("tool context has no deadline")
				}
				<-ctx.Done()
				return nil, ctx.Err()
			})
			err := RunTool(context.Background(), runner, st, ToolRunOptions{
				RunID: "run-timeout-" + test.tool, Tool: test.tool, UseNativeArgs: true, NativeArgs: []string{"--version"},
				Tools:    ToolPaths{Rustscan: "rustscan", Nmap: "nmap", Httpx: "httpx", Nuclei: "nuclei"},
				Timeouts: test.timeouts, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
			})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("RunTool error = %v, want deadline exceeded", err)
			}
		})
	}
}

func TestRunToolZeroTimeoutReusesContext(t *testing.T) {
	st := newToolRunStore(t)
	runner := runnerFunc(func(ctx context.Context, _ string, _ []string) ([]byte, error) {
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("zero timeout added a deadline")
		}
		return []byte("ok"), nil
	})
	if err := RunTool(context.Background(), runner, st, ToolRunOptions{
		RunID: "run-unlimited", Tool: "rustscan", UseNativeArgs: true, NativeArgs: []string{"--version"},
		Tools: ToolPaths{Rustscan: "rustscan"}, JSONReportPath: filepath.Join(t.TempDir(), "report.json"),
	}); err != nil {
		t.Fatal(err)
	}
}
