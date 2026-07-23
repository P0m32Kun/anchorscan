package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// projectReportHTML renders the single-file formal project report. It projects
// confirmed and not_observed verifications and embeds their evidence as data URIs
// so the output is fully offline-readable. Missing required metadata blocks the export.
func (s *server) projectReportHTML(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	deliverable, project, err := s.buildProjectDeliverable(w, r, projectID)
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.html"`, safeReportFilename(project)))
	if err := report.RenderProjectHTML(w, deliverable); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// buildProjectDeliverable loads the project, validates report metadata, and
// projects included verifications + evidence into the shared deliverable model
// consumed by both the HTML and DOCX exporters. It writes HTTP errors and
// returns a non-nil error when the response is already handled.
func (s *server) buildProjectDeliverable(w http.ResponseWriter, r *http.Request, projectID string) (report.ProjectDeliverable, store.Project, error) {
	project, err := s.store.GetProject(projectID)
	if err != nil {
		http.NotFound(w, r)
		return report.ProjectDeliverable{}, store.Project{}, err
	}
	if missing := projectReportMissingMetadata(project); len(missing) > 0 {
		http.Error(w, "报告元数据不完整，缺失："+strings.Join(missing, "、"), http.StatusBadRequest)
		return report.ProjectDeliverable{}, store.Project{}, fmt.Errorf("missing metadata")
	}
	zones, err := s.store.ListProjectZones(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return report.ProjectDeliverable{}, store.Project{}, err
	}
	verifications, err := s.store.ListProjectVerifications(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return report.ProjectDeliverable{}, store.Project{}, err
	}
	runs, err := s.store.ListProjectScanRuns(projectID, 10000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return report.ProjectDeliverable{}, store.Project{}, err
	}

	reportZones := make([]report.ProjectZone, 0, len(zones))
	for _, z := range zones {
		reportZones = append(reportZones, report.ProjectZone{ZoneID: z.ZoneID, Name: z.Name, SortOrder: z.SortOrder})
	}

	deliverableVerifications := make([]report.DeliverableVerification, 0, len(verifications))
	for _, v := range verifications {
		if v.Outcome != "confirmed" && v.Outcome != "not_observed" {
			continue
		}
		assets, err := s.store.ListVerificationAssets(v.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return report.ProjectDeliverable{}, store.Project{}, err
		}
		evidenceRows, err := s.store.ListVerificationEvidence(v.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return report.ProjectDeliverable{}, store.Project{}, err
		}
		if len(evidenceRows) == 0 {
			err := fmt.Errorf("纳入报告的验证项“%s”缺少证据", v.Title)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return report.ProjectDeliverable{}, store.Project{}, err
		}
		delAssets := make([]report.DeliverableAsset, 0, len(assets))
		for _, a := range assets {
			delAssets = append(delAssets, report.DeliverableAsset{IP: a.IP, Port: a.Port, Display: assetDisplay(a.IP, a.Port)})
		}
		delEvidence := make([]report.DeliverableEvidence, 0, len(evidenceRows))
		for _, e := range evidenceRows {
			dataURI, err := s.evidenceDataURI(projectID, e)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return report.ProjectDeliverable{}, store.Project{}, err
			}
			delEvidence = append(delEvidence, report.DeliverableEvidence{DataURI: dataURI, FilePath: s.store.EvidenceFilePath(e, projectID), MediaType: e.MediaType, Caption: e.Caption, Width: e.Width, Height: e.Height})
		}
		deliverableVerifications = append(deliverableVerifications, report.DeliverableVerification{
			ID:               v.ID,
			VulnerabilityKey: v.VulnerabilityKey,
			ZoneID:           v.ZoneID,
			Title:            v.Title,
			Severity:         v.Severity,
			Outcome:          v.Outcome,
			Description:      v.Description,
			Remediation:      v.Remediation,
			Assets:           delAssets,
			Evidence:         delEvidence,
			Position:         v.Position,
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

	metadata := report.ProjectMetadata{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		ClientUnit:  project.ClientUnit,
		ReportTitle: reportTitle(project),
		TestObject:  project.TestObject,
		StartDate:   project.StartDate,
		EndDate:     project.EndDate,
		Testers:     project.Testers,
		CreatedAt:   project.CreatedAt,
	}
	deliverable := report.BuildProjectDeliverable(metadata, reportZones, reportRuns, deliverableVerifications, s.opts.Now())
	if err := report.ValidateProjectDeliverable(deliverable); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return report.ProjectDeliverable{}, store.Project{}, err
	}
	return deliverable, project, nil
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

func (s *server) evidenceDataURI(projectID string, evidence store.VerificationEvidence) (string, error) {
	absPath := s.store.EvidenceFilePath(evidence, projectID)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	mediaType := evidence.MediaType
	if mediaType == "" {
		mediaType = "image/png"
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func projectReportMissingMetadata(project store.Project) []string {
	var missing []string
	if strings.TrimSpace(project.ClientUnit) == "" {
		missing = append(missing, "被测单位")
	}
	if strings.TrimSpace(project.TestObject) == "" {
		missing = append(missing, "测试对象")
	}
	if strings.TrimSpace(project.StartDate) == "" {
		missing = append(missing, "测试开始日期")
	}
	if strings.TrimSpace(project.EndDate) == "" {
		missing = append(missing, "测试结束日期")
	}
	if strings.TrimSpace(project.Testers) == "" {
		missing = append(missing, "测试人员")
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

func safeReportFilename(project store.Project) string {
	name := strings.TrimSpace(project.ReportTitle)
	if name == "" {
		name = strings.TrimSpace(project.Name)
	}
	if name == "" {
		name = "project-report"
	}
	name = strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_").Replace(name)
	return name
}
