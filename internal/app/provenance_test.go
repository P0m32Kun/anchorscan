package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/vuln"
)

func TestBuildRunProvenanceRecordsVersionAndScope(t *testing.T) {
	p := BuildRunProvenance(ProvenanceOptions{
		Version:        "v2.0.0-test",
		ConfigSnapshot: `{"includes":["192.0.2.0/24"]}`,
		Scope:          `{"includes":["192.0.2.0/24"]}`,
		Tools:          config.ToolPaths{Rustscan: "/bin/rustscan", Nmap: "/bin/nmap"},
	}, time.Unix(1, 0), time.Unix(2, 0), nil)
	if p.Version != "v2.0.0-test" {
		t.Fatalf("version = %q, want v2.0.0-test", p.Version)
	}
	if p.Scope != `{"includes":["192.0.2.0/24"]}` {
		t.Fatalf("scope = %q", p.Scope)
	}
	if !p.StartedAt.Equal(time.Unix(1, 0)) || !p.FinishedAt.Equal(time.Unix(2, 0)) {
		t.Fatalf("time boundaries wrong")
	}
	if p.ToolVersions["rustscan"] != "/bin/rustscan" {
		t.Fatalf("tool versions = %v", p.ToolVersions)
	}
}

func TestBuildRunProvenanceHashesRuleFiles(t *testing.T) {
	dir := t.TempDir()
	nsePath := filepath.Join(dir, "nse.yaml")
	tagPath := filepath.Join(dir, "service-tags.yaml")
	if err := os.WriteFile(nsePath, []byte("nse: rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tagPath, []byte("tags: rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := BuildRunProvenance(ProvenanceOptions{
		RulePaths: []string{nsePath, tagPath},
		NSERules:  map[string][]string{"ssh": {"ssh-brute"}},
		TagRules: []vuln.TagRule{
			{Name: "ssh", Service: []string{"ssh"}, NucleiTags: []string{"ssh"}},
		},
	}, time.Now(), time.Now(), nil)

	if len(p.RuleHashes) != 2 {
		t.Fatalf("rule hashes = %v", p.RuleHashes)
	}
	if _, ok := p.RuleHashes[nsePath]; !ok {
		t.Fatalf("missing nse hash")
	}
	if _, ok := p.RuleHashes[tagPath]; !ok {
		t.Fatalf("missing tag hash")
	}
	if len(p.NSEScripts) != 1 || len(p.Tags) != 1 {
		t.Fatalf("expected nse/tag records, got %v %v", p.NSEScripts, p.Tags)
	}
}

func TestHashArtifactFilesHashesReportJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "report.json"), []byte(`{"scan_meta":{"tool":"anchorscan"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatal(err)
	}

	hashes, err := HashArtifactFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 2 {
		t.Fatalf("hashes = %v", hashes)
	}
	if _, ok := hashes["report.json"]; !ok {
		t.Fatalf("missing report.json hash")
	}
}

func TestRedactedToolConfigSnapshotMasksNativeArgs(t *testing.T) {
	s := redactedToolConfigSnapshot(ToolRunOptions{
		Tool:          "nmap",
		UseNativeArgs: true,
		NativeArgs:    []string{"-sn", "192.0.2.10"},
		ExtraArgs:     config.ToolArgs{Nmap: []string{"--min-rate", "50"}},
	})
	if strings.Contains(s, "192.0.2.10") || strings.Contains(s, "-sn") || strings.Contains(s, "50") {
		t.Fatalf("redacted snapshot leaked args: %s", s)
	}
	if !strings.Contains(s, "REDACTED") {
		t.Fatalf("expected REDACTED placeholder: %s", s)
	}
}

func TestReportProvenanceOmitsHashesAndConfig(t *testing.T) {
	p := BuildRunProvenance(ProvenanceOptions{
		Version:        "v2.0.0-test",
		ConfigSnapshot: `{"secret":"token"}`,
		RulePaths:      []string{"/etc/rules.yaml"},
	}, time.Unix(1, 0), time.Unix(2, 0), map[string]string{"report.json": "abc"})
	rp := ReportProvenance(p, []string{"nmap", "nuclei"})
	if rp.Version != "v2.0.0-test" {
		t.Fatalf("version lost")
	}
	if len(rp.Engines) != 2 {
		t.Fatalf("engines = %v", rp.Engines)
	}
	if rp.Scope == "" {
		t.Fatalf("scope missing")
	}
	// Ensure report provenance does not carry raw config or hashes.
	b, _ := json.Marshal(rp)
	if strings.Contains(string(b), "token") || strings.Contains(string(b), "abc") || strings.Contains(string(b), "rule_hashes") {
		t.Fatalf("report provenance leaked secrets: %s", string(b))
	}
}

func TestEnginesFromDetectionChecksDeduplicates(t *testing.T) {
	engines := EnginesFromDetectionChecks([]report.DetectionCheck{
		{Engine: "nuclei", Status: "completed"},
		{Engine: "nse", Status: "completed"},
		{Engine: "nuclei", Status: "completed"},
		{Engine: "rdpscan", Status: "failed"},
	})
	if len(engines) != 2 || engines[0] != "nse" || engines[1] != "nuclei" {
		t.Fatalf("engines = %v", engines)
	}
}

func TestRunProvenanceStoredAndStableAfterRuleChange(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer scanStore.Close()

	rulePath := filepath.Join(dir, "nse.yaml")
	if err := os.WriteFile(rulePath, []byte("rules: v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	_, _ = h.Write([]byte("rules: v1\n"))
	wantHash := hex.EncodeToString(h.Sum(nil))

	p := BuildRunProvenance(ProvenanceOptions{
		Version:   "v2.0.0-test",
		RulePaths: []string{rulePath},
	}, time.Unix(1, 0), time.Unix(2, 0), nil)
	if err := SaveRunProvenance(scanStore, "run-1", p); err != nil {
		t.Fatal(err)
	}

	// Change rule file after the run is stored.
	if err := os.WriteFile(rulePath, []byte("rules: v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := scanStore.GetRunProvenance("run-1")
	if err != nil {
		t.Fatal(err)
	}
	var got RunProvenance
	if err := json.Unmarshal([]byte(manifest), &got); err != nil {
		t.Fatal(err)
	}
	if got.RuleHashes[rulePath] != wantHash {
		t.Fatalf("stored rule hash changed after file update: got %s want %s", got.RuleHashes[rulePath], wantHash)
	}
}

func TestLegacyDatabaseMigratedAndRunStillReadable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	// Create a pre-migration 13 database: schema_migrations up to 12 and the
	// scan_runs table without run_provenance.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL);
INSERT INTO schema_migrations (version, name, applied_at) VALUES (1, 'legacy', ''), (2, 'legacy', ''), (3, 'legacy', ''), (4, 'legacy', ''), (5, 'legacy', ''), (6, 'legacy', ''), (7, 'legacy', ''), (8, 'legacy', ''), (9, 'legacy', ''), (10, 'legacy', ''), (11, 'legacy', ''), (12, 'legacy', '');
CREATE TABLE scan_runs (
  run_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  zone_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  label TEXT NOT NULL,
  access_point TEXT NOT NULL,
  tester_ip TEXT NOT NULL,
  notes TEXT NOT NULL,
  include_in_report INTEGER NOT NULL,
  target TEXT NOT NULL,
  ports TEXT NOT NULL,
  profile TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  error TEXT NOT NULL,
  config_snapshot TEXT NOT NULL,
  artifact_dir TEXT NOT NULL
);
INSERT INTO scan_runs (run_id, project_id, zone_id, kind, label, access_point, tester_ip, notes, include_in_report, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir) VALUES ('run-1', '', '', '', '', '', '', '', 0, '', '', '', 'completed', '', '', '', '', '');
`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	run, err := s.GetScanRun("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if run.RunID != "run-1" {
		t.Fatalf("legacy run unreadable: %#v", run)
	}
	manifest, err := s.GetRunProvenance("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		t.Fatalf("expected empty manifest for legacy run, got %q", manifest)
	}
}

func TestToolRunHashesOnlyReportJSON(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	scanStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer scanStore.Close()

	reportPath := filepath.Join(dir, "reports", "tool-nmap-1.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write an unrelated file in the shared reports directory to prove it is not hashed.
	if err := os.WriteFile(filepath.Join(dir, "reports", "other.json"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := runnerFunc(func(ctx context.Context, binary string, args []string) ([]byte, error) {
		return []byte(`<nmaprun><host><status state="up"/></host></nmaprun>`), nil
	})
	opts := ToolRunOptions{
		RunID:          "tool-nmap-1",
		Tool:           "nmap",
		Mode:           "alive",
		Target:         "192.0.2.10",
		JSONReportPath: reportPath,
		Tools:          ToolPaths{Nmap: "/bin/nmap"},
		Timeouts:       ToolTimeouts{},
	}
	if err := RunTool(context.Background(), runner, scanStore, opts); err != nil {
		t.Fatalf("RunTool returned error: %v", err)
	}

	manifest, err := scanStore.GetRunProvenance("tool-nmap-1")
	if err != nil {
		t.Fatal(err)
	}
	var p RunProvenance
	if err := json.Unmarshal([]byte(manifest), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.ArtifactHashes) != 1 || p.ArtifactHashes["tool-nmap-1.json"] == "" {
		t.Fatalf("tool run should hash only its own report, got %v", p.ArtifactHashes)
	}
}
