package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteToolsCheckReportsConfiguredTools(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, filepath.Join(dir, "rustscan"), "")
	writeFile(t, filepath.Join(dir, "nmap"), "")
	writeFile(t, filepath.Join(dir, "httpx"), "")
	writeFile(t, filepath.Join(dir, "nuclei"), "")
	writeFile(t, configPath, "tools:\n  rustscan: "+filepath.Join(dir, "rustscan")+"\n  nmap: "+filepath.Join(dir, "nmap")+"\n  httpx: "+filepath.Join(dir, "httpx")+"\n  nuclei: "+filepath.Join(dir, "nuclei")+"\n")

	var stdout bytes.Buffer
	err := run([]string{"tools", "check", "--config", configPath}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	for _, name := range []string{"rustscan: ok", "nmap: ok", "httpx: ok", "nuclei: ok"} {
		if !strings.Contains(output, name) {
			t.Fatalf("expected %q in output %q", name, output)
		}
	}
}

func TestExecuteDoctorPrintsOptionalWarningsAndVersions(t *testing.T) {
	dir := t.TempDir()
	toolPath := filepath.Join(dir, "tool")
	writeFile(t, toolPath, "#!/bin/sh\necho scanner 1.2.3\n")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: \"\"\n  nuclei: \"\"\nscan:\n  ports: 22\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "nse.yaml"), "ssh: [ssh-hostkey]\n")

	var stdout bytes.Buffer
	err := run([]string{"doctor", "--config", configPath, "--db", filepath.Join(dir, "scan.db"), "--reports", dir}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("warnings must not fail doctor: %v\n%s", err, stdout.String())
	}
	for _, want := range []string{
		"rustscan: ok scanner 1.2.3",
		"nmap: ok scanner 1.2.3",
		"httpx: warning",
		"nuclei: warning",
		"rdpscan: warning",
		"nse rules: ok 1 rules",
		"tag rules: ok 1 rules",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in %q", want, stdout.String())
		}
	}
}

func TestExecuteDoctorPrintsChecks(t *testing.T) {
	dir := t.TempDir()
	toolPath := filepath.Join(dir, "tool")
	writeFile(t, toolPath, "#!/bin/sh\necho scanner 1.2.3\n")
	configPath := filepath.Join(dir, "config.yaml")
	writeFile(t, configPath, "tools:\n  rustscan: "+toolPath+"\n  nmap: "+toolPath+"\n  httpx: "+toolPath+"\n  nuclei: "+toolPath+"\nscan:\n  ports: 22\n  profile: normal\nprofiles:\n  normal:\n    host_workers: 1\n")
	writeFile(t, filepath.Join(dir, "nse.yaml"), "ssh: [ssh-hostkey]\n")
	writeFile(t, filepath.Join(dir, "service-tags.yaml"), "- name: ssh\n  service: [ssh]\n  nuclei_tags: [ssh]\n")

	var stdout bytes.Buffer
	err := run([]string{"doctor", "--config", configPath, "--db", filepath.Join(dir, "scan.db"), "--reports", dir}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	for _, want := range []string{
		"config: ok",
		"rustscan: ok",
		"nmap: ok",
		"ports: ok",
		"nse rules: ok",
		"tag rules: ok",
		"database: ok",
		"reports: ok",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected %q in %q", want, stdout.String())
		}
	}
}

func TestExecuteWebHelpShowsListen(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"web", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "--listen") {
		t.Fatalf("expected --listen in %q", stdout.String())
	}
}

func TestExecuteCancelPostsToServer(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/runs/run-1/cancel" {
			called = true
			w.WriteHeader(http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := run([]string{"cancel", "--run-id", "run-1", "--server", server.URL}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !called {
		t.Fatal("expected cancel request")
	}
}
