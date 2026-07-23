package report_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func projectReportInput(t *testing.T, s *store.Store, projectID string, catalog *knowledgebase.Catalog) report.ProjectReportInput {
	t.Helper()
	project, err := s.GetProject(projectID)
	if err != nil {
		t.Fatalf("GetProject returned error: %v", err)
	}
	zones, err := s.ListProjectZones(projectID)
	if err != nil {
		t.Fatalf("ListProjectZones returned error: %v", err)
	}
	runs, err := s.ListProjectScanRuns(projectID, 1000)
	if err != nil {
		t.Fatalf("ListProjectScanRuns returned error: %v", err)
	}
	findings, err := s.ListProjectFindings(projectID)
	if err != nil {
		t.Fatalf("ListProjectFindings returned error: %v", err)
	}
	fps, err := s.ListProjectFingerprints(projectID)
	if err != nil {
		t.Fatalf("ListProjectFingerprints returned error: %v", err)
	}
	checks, err := s.ListProjectDetectionChecks(projectID)
	if err != nil {
		t.Fatalf("ListProjectDetectionChecks returned error: %v", err)
	}

	inputZones := make([]report.ProjectZone, len(zones))
	for i, z := range zones {
		inputZones[i] = report.ProjectZone{ZoneID: z.ZoneID, Name: z.Name, SortOrder: z.SortOrder}
	}
	inputRuns := make([]report.ProjectRun, len(runs))
	for i, r := range runs {
		inputRuns[i] = report.ProjectRun{
			RunID: r.RunID, ZoneID: r.ZoneID, Status: r.Status, IncludeInReport: r.IncludeInReport,
			Label: r.Label, AccessPoint: r.AccessPoint, TesterIP: r.TesterIP, Target: r.Target, Ports: r.Ports, Profile: r.Profile, Notes: r.Notes,
		}
	}

	return report.ProjectReportInput{
		Project: report.ProjectMetadata{
			ID: project.ID, Name: project.Name, Description: project.Description,
			ClientUnit: project.ClientUnit, ReportTitle: project.ReportTitle, TestObject: project.TestObject,
			StartDate: project.StartDate, EndDate: project.EndDate, Testers: project.Testers,
		},
		Zones:           inputZones,
		Runs:            inputRuns,
		Findings:        findings,
		Fingerprints:    fps,
		DetectionChecks: checks,
		Catalog:         catalog,
	}
}

func writeCatalogFixture(t *testing.T, dir string) string {
	t.Helper()
	content := `<!-- anchorscan-catalog version: 1 -->

### Redis 默认登录（高危）

<!-- anchorscan-entry
id: redis-default-login
aliases:
match:
  nuclei:
    - redis-default-logins
-->

#### 漏洞描述
Redis 服务未启用认证。

#### 验证命令

##### Nuclei
` + "```" + `
nuclei -t redis-default-logins -u {{host}}:{{port}}
` + "```" + `

##### Nmap NSE
` + "```" + `
nmap -p {{port}} --script redis-info {{host}}
` + "```" + `

##### MSF
` + "```" + `
use auxiliary/scanner/redis/redis_login
set RHOSTS {{host}}
set RPORT {{port}}
run
` + "```" + `

#### 修复建议
启用 Redis 认证。

---
`
	path := filepath.Join(dir, "kb.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func writeAmbiguousCatalogFixture(t *testing.T, dir string) string {
	t.Helper()
	path := writeCatalogFixture(t, dir)
	extra := `
### Redis 默认登录副本（中危）

<!-- anchorscan-entry
id: redis-ambiguous
aliases:
match:
  nuclei:
    - redis-default-logins
-->

#### 漏洞描述
Redis 服务未启用认证（重复条目，用于歧义测试）。

#### 验证命令

##### Nuclei
` + "```" + `
nuclei -t redis-default-logins -u {{host}}:{{port}}
` + "```" + `

##### Nmap NSE
` + "```" + `
nmap -p {{port}} --script redis-info {{host}}
` + "```" + `

##### MSF
` + "```" + `
use auxiliary/scanner/redis/redis_login
set RHOSTS {{host}}
set RPORT {{port}}
run
` + "```" + `

#### 修复建议
启用 Redis 认证。

---
`
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile returned error: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(extra); err != nil {
		t.Fatalf("WriteString returned error: %v", err)
	}
	return path
}

func loadCatalog(t *testing.T, dir string) *knowledgebase.Catalog {
	t.Helper()
	path := writeCatalogFixture(t, dir)
	catalog := knowledgebase.Load("", path)
	if catalog.Status() != knowledgebase.StatusReady {
		t.Fatalf("catalog not ready: status=%s diagnostics=%v", catalog.Status(), catalog.Diagnostics())
	}
	return catalog
}

func loadAmbiguousCatalog(t *testing.T, dir string) *knowledgebase.Catalog {
	t.Helper()
	path := writeAmbiguousCatalogFixture(t, dir)
	catalog := knowledgebase.Load("", path)
	if catalog.Status() != knowledgebase.StatusReady {
		t.Fatalf("catalog not ready: status=%s diagnostics=%v", catalog.Status(), catalog.Diagnostics())
	}
	return catalog
}

func TestProjectReportPositiveCandidatesAggregateAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	for _, run := range []store.ScanRun{
		{RunID: "run-I-1", ProjectID: "p1", ZoneID: "I", Target: "10.0.1.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now},
		{RunID: "run-I-2", ProjectID: "p1", ZoneID: "I", Target: "10.0.1.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now},
		{RunID: "run-II-1", ProjectID: "p1", ZoneID: "II", Target: "10.0.2.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now},
	} {
		if err := s.SaveScanRun(run); err != nil {
			t.Fatalf("SaveScanRun returned error: %v", err)
		}
	}

	// Same vulnerability and asset appears in two I-zone runs; should be deduplicated.
	s.SaveFinding("run-I-1", report.Finding{IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.1:6379"})
	s.SaveFinding("run-I-1", report.Finding{IP: "10.0.0.2", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.2:6379"})
	s.SaveFinding("run-I-2", report.Finding{IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.1:6379"})

	// A different zone adds a distinct asset for the same vulnerability.
	s.SaveFinding("run-II-1", report.Finding{IP: "10.0.0.10", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.10:6379"})

	// Unmatched finding must be preserved as a pending candidate.
	s.SaveFinding("run-I-1", report.Finding{IP: "10.0.0.3", Port: 80, Protocol: "tcp", Source: "nuclei", ID: "unknown-id", Severity: "medium", Summary: "Unknown Issue", Target: "10.0.0.3:80"})

	catalog := loadCatalog(t, dir)
	input := projectReportInput(t, s, "p1", catalog)
	result, err := report.BuildProjectReport(input)
	if err != nil {
		t.Fatalf("BuildProjectReport returned error: %v", err)
	}
	if result.Project.ID != "p1" {
		t.Fatalf("unexpected project id: %s", result.Project.ID)
	}
	if len(result.Zones) != 3 {
		t.Fatalf("expected 3 zones, got %d", len(result.Zones))
	}

	// Zone I should have one matched and one pending candidate.
	zoneI := result.Zones[0]
	if zoneI.Zone.ZoneID != "I" || zoneI.Zone.Name != "I区" {
		t.Fatalf("unexpected zone I: %#v", zoneI.Zone)
	}
	if len(zoneI.PositiveCandidates) != 2 {
		t.Fatalf("expected 2 positive candidates in zone I, got %d", len(zoneI.PositiveCandidates))
	}

	matched := zoneI.PositiveCandidates[0]
	if matched.IsPending {
		t.Fatalf("expected first candidate to be matched")
	}
	if matched.GroupKey != "redis-default-login" {
		t.Fatalf("unexpected group key: %s", matched.GroupKey)
	}
	if matched.Title != "Redis 默认登录" {
		t.Fatalf("unexpected title: %s", matched.Title)
	}
	if matched.Severity != knowledgebase.SeverityHigh {
		t.Fatalf("unexpected severity: %s", matched.Severity)
	}
	if len(matched.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(matched.Assets))
	}
	if matched.Assets[0].IP != "10.0.0.1" || matched.Assets[0].Port != 6379 {
		t.Fatalf("unexpected first asset: %#v", matched.Assets[0])
	}
	if matched.Assets[1].IP != "10.0.0.2" || matched.Assets[1].Port != 6379 {
		t.Fatalf("unexpected second asset: %#v", matched.Assets[1])
	}
	if len(matched.SourceRuns) != 2 || matched.SourceRuns[0] != "run-I-1" || matched.SourceRuns[1] != "run-I-2" {
		t.Fatalf("unexpected source runs: %v", matched.SourceRuns)
	}

	pending := zoneI.PositiveCandidates[1]
	if !pending.IsPending {
		t.Fatalf("expected second candidate to be pending")
	}
	if pending.Title != "Unknown Issue" {
		t.Fatalf("unexpected pending title: %s", pending.Title)
	}
	if pending.Severity != knowledgebase.SeverityMedium {
		t.Fatalf("unexpected pending severity: %s", pending.Severity)
	}
	if len(pending.Assets) != 1 || pending.Assets[0].IP != "10.0.0.3" || pending.Assets[0].Port != 80 {
		t.Fatalf("unexpected pending assets: %#v", pending.Assets)
	}

	// Zone II should have the matched candidate with one asset.
	zoneII := result.Zones[1]
	if zoneII.Zone.ZoneID != "II" {
		t.Fatalf("unexpected zone II: %#v", zoneII.Zone)
	}
	if len(zoneII.PositiveCandidates) != 1 {
		t.Fatalf("expected 1 positive candidate in zone II, got %d", len(zoneII.PositiveCandidates))
	}
	if zoneII.PositiveCandidates[0].Assets[0].IP != "10.0.0.10" {
		t.Fatalf("unexpected zone II asset: %#v", zoneII.PositiveCandidates[0].Assets[0])
	}

	// Zone III has no included runs and no candidates.
	zoneIII := result.Zones[2]
	if len(zoneIII.Runs) != 0 || len(zoneIII.PositiveCandidates) != 0 {
		t.Fatalf("expected zone III to be empty, got runs=%d candidates=%d", len(zoneIII.Runs), len(zoneIII.PositiveCandidates))
	}
}

func TestProjectReportInfoFindingsExcluded(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "run1", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	s.SaveFinding("run1", report.Finding{IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.1:6379"})
	s.SaveFinding("run1", report.Finding{IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-info", Severity: "info", Summary: "Redis Info", Target: "10.0.0.1:6379"})

	catalog := loadCatalog(t, dir)
	input := projectReportInput(t, s, "p1", catalog)
	result, _ := report.BuildProjectReport(input)
	if len(result.Zones[0].PositiveCandidates) != 1 {
		t.Fatalf("expected 1 positive candidate, got %d", len(result.Zones[0].PositiveCandidates))
	}
}

func TestProjectReportAmbiguousAndUnmatchedBecomePending(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "run1", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	// Both catalog entries match this nuclei ID, so it is ambiguous and must not be treated as matched.
	s.SaveFinding("run1", report.Finding{IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.1:6379"})

	catalog := loadAmbiguousCatalog(t, dir)
	input := projectReportInput(t, s, "p1", catalog)
	result, _ := report.BuildProjectReport(input)
	if len(result.Zones[0].PositiveCandidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Zones[0].PositiveCandidates))
	}
	candidate := result.Zones[0].PositiveCandidates[0]
	if !candidate.IsPending {
		t.Fatalf("expected ambiguous finding to become pending")
	}
	if candidate.Description != "" {
		t.Fatalf("pending candidate must not carry a guessed description: %s", candidate.Description)
	}
	if candidate.Remediation != "" {
		t.Fatalf("pending candidate must not carry a guessed remediation: %s", candidate.Remediation)
	}
}

func TestProjectReportNegativeCandidates(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "run1", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "80", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := s.SaveFingerprint("run1", fingerprint.ServiceFingerprint{IP: "10.0.0.1", Port: 80, Protocol: "tcp", Service: "http", Product: "nginx", Normalized: "http"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{RunID: "run1", IP: "10.0.0.1", Port: 80, Protocol: "tcp", Engine: "nse", Status: "completed", StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("UpsertDetectionCheck returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{RunID: "run1", IP: "10.0.0.1", Port: 80, Protocol: "tcp", Engine: "nuclei", Status: "completed", StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("UpsertDetectionCheck returned error: %v", err)
	}

	catalog := loadCatalog(t, dir)
	input := projectReportInput(t, s, "p1", catalog)
	result, _ := report.BuildProjectReport(input)
	if len(result.Zones[0].NegativeCandidates) != 1 {
		t.Fatalf("expected 1 negative candidate, got %d", len(result.Zones[0].NegativeCandidates))
	}
	nc := result.Zones[0].NegativeCandidates[0]
	if nc.Asset.IP != "10.0.0.1" || nc.Asset.Port != 80 {
		t.Fatalf("unexpected negative candidate asset: %#v", nc.Asset)
	}
	if len(result.Zones[0].IncompleteChecks) != 0 {
		t.Fatalf("expected no incomplete checks, got %d", len(result.Zones[0].IncompleteChecks))
	}
}

func TestProjectReportIncompleteChecks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "run1", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "80,3389", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := s.SaveFingerprint("run1", fingerprint.ServiceFingerprint{IP: "10.0.0.1", Port: 80, Protocol: "tcp", Service: "http", Product: "nginx", Normalized: "http"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := s.SaveFingerprint("run1", fingerprint.ServiceFingerprint{IP: "10.0.0.2", Port: 3389, Protocol: "tcp", Service: "rdp", Product: "ms-wbt-server", Normalized: "rdp"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{RunID: "run1", IP: "10.0.0.1", Port: 80, Protocol: "tcp", Engine: "nse", Status: "completed", StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("UpsertDetectionCheck returned error: %v", err)
	}
	if err := s.UpsertDetectionCheck(store.DetectionCheck{RunID: "run1", IP: "10.0.0.1", Port: 80, Protocol: "tcp", Engine: "nuclei", Status: "failed", StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("UpsertDetectionCheck returned error: %v", err)
	}
	// 10.0.0.2 has no detection checks at all (legacy data), so it must be incomplete.

	catalog := loadCatalog(t, dir)
	input := projectReportInput(t, s, "p1", catalog)
	result, _ := report.BuildProjectReport(input)
	if len(result.Zones[0].NegativeCandidates) != 0 {
		t.Fatalf("expected no negative candidates, got %d", len(result.Zones[0].NegativeCandidates))
	}
	if len(result.Zones[0].IncompleteChecks) != 2 {
		t.Fatalf("expected 2 incomplete checks, got %d", len(result.Zones[0].IncompleteChecks))
	}

	var httpCheck, rdpCheck *report.ProjectIncompleteCheck
	for i := range result.Zones[0].IncompleteChecks {
		c := &result.Zones[0].IncompleteChecks[i]
		if c.Asset.Port == 80 {
			httpCheck = c
		} else if c.Asset.Port == 3389 {
			rdpCheck = c
		}
	}
	if httpCheck == nil || rdpCheck == nil {
		t.Fatalf("expected checks for ports 80 and 3389: %#v", result.Zones[0].IncompleteChecks)
	}
	if len(httpCheck.Engines) != 1 || httpCheck.Engines[0].Engine != "nuclei" || httpCheck.Engines[0].Status != "failed" {
		t.Fatalf("unexpected httpCheck engines: %#v", httpCheck.Engines)
	}
	if len(rdpCheck.Engines) != 2 {
		t.Fatalf("expected 2 missing engines for rdp, got %d", len(rdpCheck.Engines))
	}
	engineNames := []string{rdpCheck.Engines[0].Engine, rdpCheck.Engines[1].Engine}
	sort.Strings(engineNames)
	if engineNames[0] != "nse" || engineNames[1] != "nuclei" {
		t.Fatalf("unexpected rdp engines: %v", engineNames)
	}
}

func TestProjectReportClassifiesPerEngineCoverage(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "scan.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "run1", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.1", Ports: "111,112,3389", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	for _, fp := range []fingerprint.ServiceFingerprint{
		{IP: "10.0.0.1", Port: 111, Protocol: "tcp", Service: "rpcbind", Product: "rpcbind", Normalized: "rpcbind"},
		{IP: "10.0.0.1", Port: 112, Protocol: "tcp", Service: "unknown", Normalized: "unknown"},
		{IP: "10.0.0.1", Port: 3389, Protocol: "tcp", Service: "rdp", Normalized: "rdp"},
	} {
		if err := s.SaveFingerprint("run1", fp); err != nil {
			t.Fatalf("SaveFingerprint returned error: %v", err)
		}
	}
	for _, check := range []store.DetectionCheck{
		{RunID: "run1", IP: "10.0.0.1", Port: 111, Protocol: "tcp", Engine: "nse", Status: "skipped", ReasonCode: "no_matching_rule", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 111, Protocol: "tcp", Engine: "nuclei", Status: "completed", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 111, Protocol: "tcp", Engine: "rdpscan", Status: "skipped", ReasonCode: "no_matching_rule", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 112, Protocol: "tcp", Engine: "nse", Status: "skipped", ReasonCode: "no_matching_rule", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 112, Protocol: "tcp", Engine: "nuclei", Status: "skipped", ReasonCode: "no_matching_rule", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 112, Protocol: "tcp", Engine: "rdpscan", Status: "skipped", ReasonCode: "no_matching_rule", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 3389, Protocol: "tcp", Engine: "nse", Status: "completed", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 3389, Protocol: "tcp", Engine: "nuclei", Status: "canceled", StartedAt: now, FinishedAt: now},
		{RunID: "run1", IP: "10.0.0.1", Port: 3389, Protocol: "tcp", Engine: "rdpscan", Status: "interrupted", StartedAt: now, FinishedAt: now},
	} {
		if err := s.UpsertDetectionCheck(check); err != nil {
			t.Fatalf("UpsertDetectionCheck returned error: %v", err)
		}
	}

	result, _ := report.BuildProjectReport(projectReportInput(t, s, "p1", loadCatalog(t, dir)))
	if got := len(result.Zones[0].NegativeCandidates); got != 1 {
		t.Fatalf("one completed engine covers the service, expected 1 negative candidate, got %d", got)
	}
	if got := result.Zones[0].NegativeCandidates[0].Asset.Port; got != 111 {
		t.Fatalf("all engines without matching rules must not produce negative proof, got port %d", got)
	}
	if got := len(result.Zones[0].IncompleteChecks); got != 1 {
		t.Fatalf("expected only the endpoint with abnormal engine states to be incomplete, got %d", got)
	}
	incomplete := result.Zones[0].IncompleteChecks[0]
	if incomplete.Asset.Port != 3389 || len(incomplete.Engines) != 2 {
		t.Fatalf("unexpected incomplete check: %#v", incomplete)
	}
	statuses := []string{incomplete.Engines[0].Status, incomplete.Engines[1].Status}
	sort.Strings(statuses)
	if statuses[0] != "canceled" || statuses[1] != "interrupted" {
		t.Fatalf("canceled and interrupted engines must remain incomplete, got %v", statuses)
	}
}

func TestProjectReportExcludesNonIncludedRuns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "included", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "excluded", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: false, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	s.SaveFinding("included", report.Finding{IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.1:6379"})
	s.SaveFinding("excluded", report.Finding{IP: "10.0.0.2", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.2:6379"})

	catalog := loadCatalog(t, dir)
	input := projectReportInput(t, s, "p1", catalog)
	result, _ := report.BuildProjectReport(input)
	if len(result.Zones[0].PositiveCandidates) != 1 {
		t.Fatalf("expected 1 candidate from included run, got %d", len(result.Zones[0].PositiveCandidates))
	}
	if result.Zones[0].PositiveCandidates[0].Assets[0].IP != "10.0.0.1" {
		t.Fatalf("unexpected asset: %#v", result.Zones[0].PositiveCandidates[0].Assets[0])
	}
	if len(result.Zones[0].Runs) != 1 || result.Zones[0].Runs[0].RunID != "included" {
		t.Fatalf("expected only included run, got %#v", result.Zones[0].Runs)
	}
}

func TestProjectReportStoreQueriesOnlyReturnIncludedCompletedRuns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	for _, run := range []store.ScanRun{
		{RunID: "included", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now},
		{RunID: "not-included", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "completed", IncludeInReport: false, StartedAt: now, FinishedAt: now},
		{RunID: "running", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "running", IncludeInReport: true, StartedAt: now, FinishedAt: now},
		{RunID: "failed", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.0/24", Ports: "6379", Profile: "normal", Status: "failed", IncludeInReport: true, StartedAt: now, FinishedAt: now},
	} {
		if err := s.SaveScanRun(run); err != nil {
			t.Fatalf("SaveScanRun returned error: %v", err)
		}
		s.SaveFinding(run.RunID, report.Finding{IP: "10.0.0.1", Port: 6379, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "10.0.0.1:6379"})
	}

	findings, err := s.ListProjectFindings("p1")
	if err != nil {
		t.Fatalf("ListProjectFindings returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 project finding, got %d", len(findings))
	}
	if findings[0].RunID != "included" {
		t.Fatalf("expected finding from included run, got %s", findings[0].RunID)
	}
}

func TestProjectReportCandidateIncludesServiceAndTarget(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Unix(1, 0)
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	if err := s.SaveScanRun(store.ScanRun{RunID: "run-1", ProjectID: "p1", ZoneID: "I", Target: "10.0.0.1", Ports: "80", Profile: "normal", Status: "completed", IncludeInReport: true, StartedAt: now, FinishedAt: now}); err != nil {
		t.Fatalf("SaveScanRun returned error: %v", err)
	}
	if err := s.SaveFingerprint("run-1", fingerprint.ServiceFingerprint{IP: "10.0.0.1", Port: 80, Protocol: "tcp", Service: "http", IsWeb: true, URL: "http://10.0.0.1/"}); err != nil {
		t.Fatalf("SaveFingerprint returned error: %v", err)
	}
	if err := s.SaveFinding("run-1", report.Finding{IP: "10.0.0.1", Port: 80, Protocol: "tcp", Source: "nuclei", ID: "redis-default-logins", Severity: "high", Summary: "Redis Default Login", Target: "http://10.0.0.1/"}); err != nil {
		t.Fatalf("SaveFinding returned error: %v", err)
	}

	catalog := loadCatalog(t, dir)
	input := projectReportInput(t, s, "p1", catalog)
	result, err := report.BuildProjectReport(input)
	if err != nil {
		t.Fatalf("BuildProjectReport returned error: %v", err)
	}
	if len(result.Zones) == 0 || len(result.Zones[0].PositiveCandidates) == 0 {
		t.Fatalf("expected positive candidate")
	}
	cand := result.Zones[0].PositiveCandidates[0]
	if len(cand.Services) != 1 || cand.Services[0] != "http" {
		t.Fatalf("expected service http, got %v", cand.Services)
	}
	if len(cand.Assets) == 0 || cand.Assets[0].Target != "http://10.0.0.1/" {
		t.Fatalf("expected asset target, got %v", cand.Assets)
	}
	if len(cand.Sources) == 0 || cand.Sources[0].FindingID != "redis-default-logins" {
		t.Fatalf("expected source finding, got %v", cand.Sources)
	}
}
