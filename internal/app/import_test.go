package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

const importFixtureXML = `<nmaprun>
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
    <hostscript>
      <script id="ssh-hostkey" output="2048 aa:bb"/>
    </hostscript>
  </host>
  <postscripts>
    <script id="http-title" output="Welcome"/>
  </postscripts>
</nmaprun>`

func writeImportFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.xml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func TestImportNmapCreatesCompletedRunWithTCPAndUDP(t *testing.T) {
	scanStore := newScanStore(t)
	xmlPath := writeImportFixture(t, importFixtureXML)

	runID, err := ImportNmap(context.Background(), scanStore, ImportNmapOptions{
		XMLPath: xmlPath,
		Now:     func() time.Time { return time.Unix(1700000000, 0) },
	})
	if err != nil {
		t.Fatalf("ImportNmap returned error: %v", err)
	}

	run, err := scanStore.GetScanRun(runID)
	if err != nil || run.Status != "completed" {
		t.Fatalf("expected completed run, got %#v err=%v", run, err)
	}

	fps, err := scanStore.ListFingerprints(runID)
	if err != nil {
		t.Fatalf("ListFingerprints returned error: %v", err)
	}
	if len(fps) != 2 {
		t.Fatalf("expected two fingerprints (tcp+udp), got %d", len(fps))
	}
	protos := map[string]bool{}
	for _, fp := range fps {
		protos[fp.Protocol] = true
		if fp.CPE != "cpe:/a:isc:bind:9.18" && fp.Protocol == "tcp" {
			t.Fatalf("CPE not persisted on tcp: %q", fp.CPE)
		}
	}
	if !protos["tcp"] || !protos["udp"] {
		t.Fatalf("expected both tcp and udp, got %v", protos)
	}

	findings, err := scanStore.ListFindings(runID)
	if err != nil {
		t.Fatalf("ListFindings returned error: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("expected three findings (port+hostscript+postscript), got %d", len(findings))
	}
}

func TestImportNmapWritesReports(t *testing.T) {
	scanStore := newScanStore(t)
	xmlPath := writeImportFixture(t, importFixtureXML)
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "reports", "import.json")
	htmlPath := filepath.Join(dir, "reports", "import.html")

	_, err := ImportNmap(context.Background(), scanStore, ImportNmapOptions{
		XMLPath:  xmlPath,
		JSONPath: jsonPath,
		HTMLPath: htmlPath,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
	})
	if err != nil {
		t.Fatalf("ImportNmap returned error: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("JSON report not written: %v", err)
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Fatalf("HTML report not written: %v", err)
	}
}

func TestImportNmapRejectsEmptyXMLWithoutRun(t *testing.T) {
	scanStore := newScanStore(t)
	xmlPath := writeImportFixture(t, "")

	before := countRuns(t, scanStore)
	_, err := ImportNmap(context.Background(), scanStore, ImportNmapOptions{
		XMLPath: xmlPath,
	})
	if err == nil || !strings.Contains(err.Error(), "empty XML file") {
		t.Fatalf("expected empty XML error, got: %v", err)
	}
	if after := countRuns(t, scanStore); after != before {
		t.Fatalf("no run should be created on failure, before=%d after=%d", before, after)
	}
}

func TestImportNmapRejectsNonNmaprunWithoutRun(t *testing.T) {
	scanStore := newScanStore(t)
	xmlPath := writeImportFixture(t, `<foo><bar/></foo>`)

	before := countRuns(t, scanStore)
	_, err := ImportNmap(context.Background(), scanStore, ImportNmapOptions{
		XMLPath: xmlPath,
	})
	if err == nil || !strings.Contains(err.Error(), "root element is not nmaprun") {
		t.Fatalf("expected non-nmaprun error, got: %v", err)
	}
	if after := countRuns(t, scanStore); after != before {
		t.Fatalf("no run should be created on failure, before=%d after=%d", before, after)
	}
}

func TestImportNmapRejectsInvalidXMLWithoutRun(t *testing.T) {
	scanStore := newScanStore(t)
	xmlPath := writeImportFixture(t, `<nmaprun><host><address addr="1.2.3.4"`)

	before := countRuns(t, scanStore)
	_, err := ImportNmap(context.Background(), scanStore, ImportNmapOptions{
		XMLPath: xmlPath,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid Nmap XML") {
		t.Fatalf("expected invalid XML error, got: %v", err)
	}
	if after := countRuns(t, scanStore); after != before {
		t.Fatalf("no run should be created on failure, before=%d after=%d", before, after)
	}
}

func TestImportNmapRequiresXMLPath(t *testing.T) {
	scanStore := newScanStore(t)
	_, err := ImportNmap(context.Background(), scanStore, ImportNmapOptions{})
	if err == nil || !strings.Contains(err.Error(), "--xml") {
		t.Fatalf("expected --xml required error, got: %v", err)
	}
}

func countRuns(t *testing.T, scanStore *store.Store) int {
	t.Helper()
	rows, err := scanStore.ListScanRuns(100)
	if err != nil {
		t.Fatalf("ListScanRuns returned error: %v", err)
	}
	return len(rows)
}
