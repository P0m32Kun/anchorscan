package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestExecuteReportWritesHTMLFromStoredRun(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "scan.db")
	htmlPath := filepath.Join(dir, "report.html")
	jsonPath := filepath.Join(dir, "report.json")
	writeFile(t, configPath, "tools:\n  rustscan: /opt/rustscan\n  nmap: /opt/nmap\n  httpx: /opt/httpx\n  nuclei: /opt/nuclei\n")

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := scanStore.SaveFingerprint("run-1", sampleFingerprint()); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}

	err = run([]string{
		"report",
		"--config", configPath,
		"--db", dbPath,
		"--run-id", "run-1",
		"--json", jsonPath,
		"--html", htmlPath,
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	for _, path := range []string{jsonPath, htmlPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
	}
}

func TestExecuteImportNmapWritesRunAndReports(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "sample.xml")
	dbPath := filepath.Join(dir, "scan.db")
	jsonPath := filepath.Join(dir, "import.json")
	htmlPath := filepath.Join(dir, "import.html")
	writeFile(t, xmlPath, `<nmaprun>
  <host>
    <address addr="10.0.0.53"/>
    <ports>
      <port protocol="tcp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
        <cpe>cpe:/a:isc:bind:9.18</cpe>
        <script id="dns-version" output="9.18.0"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
    </ports>
  </host>
</nmaprun>`)

	var stdout bytes.Buffer
	err := run([]string{
		"import-nmap",
		"--xml", xmlPath,
		"--db", dbPath,
		"--json", jsonPath,
		"--html", htmlPath,
	}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "run_id=") {
		t.Fatalf("expected run_id in stdout: %q", stdout.String())
	}
	for _, path := range []string{jsonPath, htmlPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 1 {
		t.Fatalf("expected one run, got %d err=%v", len(runs), err)
	}
	fps, err := scanStore.ListFingerprints(runs[0].RunID)
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 2 {
		t.Fatalf("expected two fingerprints (tcp+udp), got %d", len(fps))
	}
	protos := map[string]bool{}
	for _, fp := range fps {
		protos[fp.Protocol] = true
	}
	if !protos["tcp"] || !protos["udp"] {
		t.Fatalf("expected both tcp and udp, got %v", protos)
	}
}

func TestExecuteImportNmapRejectsEmptyXML(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "empty.xml")
	dbPath := filepath.Join(dir, "scan.db")
	writeFile(t, xmlPath, "")

	err := run([]string{
		"import-nmap",
		"--xml", xmlPath,
		"--db", dbPath,
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "empty XML file") {
		t.Fatalf("expected empty XML error, got: %v", err)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}

func TestExecuteImportNmapRejectsNonNmaprun(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "foo.xml")
	dbPath := filepath.Join(dir, "scan.db")
	writeFile(t, xmlPath, `<foo><bar/></foo>`)

	err := run([]string{
		"import-nmap",
		"--xml", xmlPath,
		"--db", dbPath,
	}, &bytes.Buffer{}, &bytes.Buffer{}, cliDeps{})
	if err == nil || !strings.Contains(err.Error(), "root element is not nmaprun") {
		t.Fatalf("expected non-nmaprun error, got: %v", err)
	}

	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	runs, err := scanStore.ListScanRuns(100)
	if err != nil || len(runs) != 0 {
		t.Fatalf("expected no run on failure, got %d err=%v", len(runs), err)
	}
}

func TestExecuteImportNmapHelpShowsFlags(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"import-nmap", "--help"}, &stdout, &bytes.Buffer{}, cliDeps{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Usage: anchorscan import-nmap",
		"--xml",
		"--db",
		"--run-id",
		"--project",
		"--json",
		"--html",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in help output %q", want, output)
		}
	}
}
