package store

import (
	"encoding/json"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

// BuildProjectReportInput gathers all project-level facts into the single
// read model seam used by report.BuildProjectReport.
func (s *Store) BuildProjectReportInput(projectID string, catalog *knowledgebase.Catalog) (report.ProjectReportInput, error) {
	project, err := s.GetProject(projectID)
	if err != nil {
		return report.ProjectReportInput{}, err
	}
	zones, err := s.ListProjectZones(projectID)
	if err != nil {
		return report.ProjectReportInput{}, err
	}
	runs, err := s.ListProjectScanRuns(projectID, 1000)
	if err != nil {
		return report.ProjectReportInput{}, err
	}
	findings, err := s.ListProjectFindings(projectID)
	if err != nil {
		return report.ProjectReportInput{}, err
	}
	fingerprints, err := s.ListProjectFingerprints(projectID)
	if err != nil {
		return report.ProjectReportInput{}, err
	}
	checks, err := s.ListProjectDetectionChecks(projectID)
	if err != nil {
		return report.ProjectReportInput{}, err
	}

	inputZones := make([]report.ProjectZone, len(zones))
	for i, z := range zones {
		inputZones[i] = report.ProjectZone{ZoneID: z.ZoneID, Name: z.Name, SortOrder: z.SortOrder}
	}
	inputRuns := make([]report.ProjectRun, len(runs))
	for i, r := range runs {
		excludeTargets, excludePorts := projectRunExclusions(r.ConfigSnapshot)
		inputRuns[i] = report.ProjectRun{
			RunID: r.RunID, ZoneID: r.ZoneID, Status: r.Status, IncludeInReport: r.IncludeInReport,
			Label: r.Label, AccessPoint: r.AccessPoint, TesterIP: r.TesterIP, Target: r.Target,
			ExcludeTargets: excludeTargets, Ports: r.Ports, ExcludePorts: excludePorts,
			Profile: r.Profile, Notes: r.Notes,
		}
	}

	return report.ProjectReportInput{
		Project: report.ProjectMetadata{
			ID: project.ID, Name: project.Name, Description: project.Description,
			ClientUnit: project.ClientUnit, ReportTitle: project.ReportTitle,
			TestObject: project.TestObject, StartDate: project.StartDate,
			EndDate: project.EndDate, Testers: project.Testers, CreatedAt: project.CreatedAt,
		},
		Zones:           inputZones,
		Runs:            inputRuns,
		Findings:        findings,
		Fingerprints:    fingerprints,
		DetectionChecks: checks,
		Catalog:         catalog,
	}, nil
}

func projectRunExclusions(snapshot string) (string, string) {
	var values struct {
		ExcludeTargets string `json:"exclude_targets"`
		ExcludePorts   string `json:"exclude_ports"`
	}
	if json.Unmarshal([]byte(snapshot), &values) != nil {
		return "", ""
	}
	return strings.TrimSpace(values.ExcludeTargets), strings.TrimSpace(values.ExcludePorts)
}

// ListProjectFindings returns all findings from the project's included
// completed runs, together with the run and zone they came from. It is the
// only store seam used to build the project-level read model.
func (s *Store) ListProjectFindings(projectID string) ([]report.ProjectFinding, error) {
	rows, err := s.db.Query(`
		SELECT r.run_id, r.zone_id, f.ip, f.port, f.protocol, f.scope, f.source, f.finding_id, f.severity, f.summary, f.target, f.output
		FROM findings f
		JOIN scan_runs r ON f.run_id = r.run_id
		WHERE r.project_id = ? AND r.include_in_report = 1 AND r.status IN ('completed', 'completed_with_errors')
		ORDER BY r.zone_id, f.ip, f.port, f.protocol, f.finding_id`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []report.ProjectFinding
	for rows.Next() {
		var pf report.ProjectFinding
		if err := rows.Scan(
			&pf.RunID, &pf.ZoneID,
			&pf.IP, &pf.Port, &pf.Protocol, &pf.Scope, &pf.Source, &pf.ID, &pf.Severity, &pf.Summary, &pf.Target, &pf.Output,
		); err != nil {
			return nil, err
		}
		findings = append(findings, pf)
	}
	return findings, rows.Err()
}

// ListProjectFingerprints returns all fingerprints from the project's included
// completed runs, together with the run and zone they came from.
func (s *Store) ListProjectFingerprints(projectID string) ([]report.ProjectFingerprint, error) {
	rows, err := s.db.Query(`
		SELECT r.run_id, r.zone_id, f.ip, f.port, f.protocol, f.service, f.product, f.version, f.extrainfo, f.tunnel, f.cpe, f.normalized, f.is_web, f.url
		FROM fingerprints f
		JOIN scan_runs r ON f.run_id = r.run_id
		WHERE r.project_id = ? AND r.include_in_report = 1 AND r.status IN ('completed', 'completed_with_errors')
		ORDER BY r.zone_id, f.ip, f.port, f.protocol`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fingerprints []report.ProjectFingerprint
	for rows.Next() {
		var fp report.ProjectFingerprint
		var isWeb int
		if err := rows.Scan(
			&fp.RunID, &fp.ZoneID,
			&fp.IP, &fp.Port, &fp.Protocol, &fp.Service, &fp.Product, &fp.Version, &fp.ExtraInfo, &fp.Tunnel, &fp.CPE, &fp.Normalized, &isWeb, &fp.URL,
		); err != nil {
			return nil, err
		}
		fp.IsWeb = isWeb == 1
		fingerprints = append(fingerprints, fp)
	}
	return fingerprints, rows.Err()
}

// ListProjectDetectionChecks returns all detection checks from the project's
// included completed runs, together with the run and zone they came from.
func (s *Store) ListProjectDetectionChecks(projectID string) ([]report.ProjectDetectionCheck, error) {
	rows, err := s.db.Query(`
		SELECT r.run_id, r.zone_id, c.ip, c.port, c.protocol, c.engine, c.status, c.reason_code, c.detail, c.started_at, c.finished_at
		FROM detection_checks c
		JOIN scan_runs r ON c.run_id = r.run_id
		WHERE r.project_id = ? AND r.include_in_report = 1 AND r.status IN ('completed', 'completed_with_errors')
		ORDER BY r.zone_id, c.ip, c.port, c.protocol, c.engine`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []report.ProjectDetectionCheck
	for rows.Next() {
		var pc report.ProjectDetectionCheck
		var startedAt, finishedAt string
		if err := rows.Scan(
			&pc.RunID, &pc.ZoneID,
			&pc.IP, &pc.Port, &pc.Protocol, &pc.Engine, &pc.Status, &pc.ReasonCode, &pc.Detail, &startedAt, &finishedAt,
		); err != nil {
			return nil, err
		}
		started, err := parseTime(startedAt)
		if err != nil {
			return nil, err
		}
		finished, err := parseTime(finishedAt)
		if err != nil {
			return nil, err
		}
		pc.StartedAt = report.DetectionCheckTime(started)
		pc.FinishedAt = report.DetectionCheckTime(finished)
		checks = append(checks, pc)
	}
	return checks, rows.Err()
}
