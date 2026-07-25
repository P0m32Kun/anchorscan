package app

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

var (
	ErrProjectReportNotFound = errors.New("project report not found")
	ErrInvalidProjectReport  = errors.New("invalid project report")
)

// BuildProjectDeliverable reads, assembles, and validates the shared HTML and
// DOCX project-report model. Evidence is loaded eagerly for both adapters.
func BuildProjectDeliverable(scanStore *store.Store, projectID string, now time.Time) (report.ProjectDeliverable, error) {
	project, err := scanStore.GetProject(projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return report.ProjectDeliverable{}, fmt.Errorf("%w: %s", ErrProjectReportNotFound, projectID)
		}
		return report.ProjectDeliverable{}, err
	}
	if missing := projectReportMissingMetadata(project); len(missing) > 0 {
		return report.ProjectDeliverable{}, invalidProjectReportError("报告元数据不完整，缺失：" + strings.Join(missing, "、"))
	}
	zones, err := scanStore.ListProjectZones(projectID)
	if err != nil {
		return report.ProjectDeliverable{}, err
	}
	verifications, err := scanStore.ListProjectVerifications(projectID)
	if err != nil {
		return report.ProjectDeliverable{}, err
	}
	runs, err := scanStore.ListProjectScanRuns(projectID, 10000)
	if err != nil {
		return report.ProjectDeliverable{}, err
	}

	reportZones := make([]report.ProjectZone, 0, len(zones))
	for _, zone := range zones {
		reportZones = append(reportZones, report.ProjectZone{ZoneID: zone.ZoneID, Name: zone.Name, SortOrder: zone.SortOrder})
	}

	deliverableVerifications := make([]report.DeliverableVerification, 0, len(verifications))
	for _, verification := range verifications {
		if verification.Outcome != "confirmed" && verification.Outcome != "not_observed" {
			continue
		}
		assets, err := scanStore.ListVerificationAssets(verification.ID)
		if err != nil {
			return report.ProjectDeliverable{}, err
		}
		evidenceRows, err := scanStore.ListVerificationEvidence(verification.ID)
		if err != nil {
			return report.ProjectDeliverable{}, err
		}
		if len(evidenceRows) == 0 {
			return report.ProjectDeliverable{}, invalidProjectReportError(fmt.Sprintf("纳入报告的验证项“%s”缺少证据", verification.Title))
		}
		zoneID, ok := projectReportVerificationZone(verification.ZoneID, zones, runs)
		if !ok {
			return report.ProjectDeliverable{}, invalidProjectReportError(fmt.Sprintf("纳入报告的验证项“%s”没有有效网络分区", verification.Title))
		}

		deliverableAssets := make([]report.DeliverableAsset, 0, len(assets))
		for _, asset := range assets {
			deliverableAssets = append(deliverableAssets, report.DeliverableAsset{IP: asset.IP, Port: asset.Port, Display: assetDisplay(asset.IP, asset.Port)})
		}
		deliverableEvidence := make([]report.DeliverableEvidence, 0, len(evidenceRows))
		for _, evidence := range evidenceRows {
			filePath := scanStore.EvidenceFilePath(evidence, projectID)
			dataURI, err := evidenceDataURI(filePath, evidence.MediaType)
			if err != nil {
				return report.ProjectDeliverable{}, err
			}
			deliverableEvidence = append(deliverableEvidence, report.DeliverableEvidence{DataURI: dataURI, FilePath: filePath, MediaType: evidence.MediaType, Caption: evidence.Caption, Width: evidence.Width, Height: evidence.Height})
		}
		deliverableVerifications = append(deliverableVerifications, report.DeliverableVerification{
			ID: verification.ID, VulnerabilityKey: verification.VulnerabilityKey, ZoneID: zoneID, Title: verification.Title,
			Severity: verification.Severity, Outcome: verification.Outcome, Description: verification.Description, Remediation: verification.Remediation,
			Assets: deliverableAssets, Evidence: deliverableEvidence, Position: verification.Position,
		})
	}

	reportRuns := make([]report.ProjectRun, 0, len(runs))
	for _, run := range runs {
		excludeTargets, excludePorts := reportRunExclusions(run.ConfigSnapshot)
		reportRuns = append(reportRuns, report.ProjectRun{
			RunID: run.RunID, ZoneID: run.ZoneID, Status: run.Status, IncludeInReport: run.IncludeInReport,
			Label: run.Label, AccessPoint: run.AccessPoint, TesterIP: run.TesterIP, Target: run.Target,
			ExcludeTargets: excludeTargets, Ports: run.Ports, ExcludePorts: excludePorts, Profile: run.Profile, Notes: run.Notes,
		})
	}

	deliverable := report.BuildProjectDeliverable(report.ProjectMetadata{
		ID: project.ID, Name: project.Name, Description: project.Description, ClientUnit: project.ClientUnit,
		ReportTitle: reportTitle(project), TestObject: project.TestObject, StartDate: project.StartDate,
		EndDate: project.EndDate, Testers: project.Testers, CreatedAt: project.CreatedAt,
	}, reportZones, reportRuns, deliverableVerifications, now)
	if err := report.ValidateProjectDeliverable(deliverable); err != nil {
		return report.ProjectDeliverable{}, fmt.Errorf("%w: %v", ErrInvalidProjectReport, err)
	}
	return deliverable, nil
}

func invalidProjectReportError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidProjectReport, message)
}

func projectReportVerificationZone(zoneID string, zones []store.ProjectZone, runs []store.ScanRun) (string, bool) {
	validZones := make(map[string]bool, len(zones))
	for _, zone := range zones {
		validZones[zone.ZoneID] = true
	}
	if validZones[zoneID] {
		return zoneID, true
	}
	activeZones := map[string]bool{}
	for _, run := range runs {
		if run.IncludeInReport && (run.Status == "completed" || run.Status == "completed_with_errors") && validZones[run.ZoneID] {
			activeZones[run.ZoneID] = true
		}
	}
	if len(activeZones) == 1 {
		for activeZone := range activeZones {
			return activeZone, true
		}
	}
	if len(zones) == 1 {
		return zones[0].ZoneID, true
	}
	return "", false
}

func reportRunExclusions(snapshot string) (string, string) {
	var values struct {
		ExcludeTargets string `json:"exclude_targets"`
		ExcludePorts   string `json:"exclude_ports"`
	}
	if json.Unmarshal([]byte(snapshot), &values) != nil {
		return "", ""
	}
	return strings.TrimSpace(values.ExcludeTargets), strings.TrimSpace(values.ExcludePorts)
}

func evidenceDataURI(filePath, mediaType string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	if mediaType == "" {
		mediaType = "image/png"
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func projectReportMissingMetadata(project store.Project) []string {
	var missing []string
	for _, field := range []struct {
		value string
		name  string
	}{
		{project.ClientUnit, "被测单位"},
		{project.TestObject, "测试对象"},
		{project.StartDate, "测试开始日期"},
		{project.EndDate, "测试结束日期"},
		{project.Testers, "测试人员"},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	return missing
}

func reportTitle(project store.Project) string {
	unit := strings.TrimSpace(project.ClientUnit)
	if unit == "" {
		unit = strings.TrimSpace(project.Name)
	}
	if unit == "" {
		return "安全渗透测试分析报告"
	}
	return unit + "安全渗透测试分析报告"
}

func assetDisplay(ip string, port int) string {
	if port == 0 {
		return ip
	}
	return fmt.Sprintf("%s:%d", ip, port)
}
