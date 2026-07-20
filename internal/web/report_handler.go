package web

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

func (s *server) reportDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/reports/")
	if strings.HasSuffix(path, "/commands/batch") {
		s.reportBatchCommand(w, r, strings.TrimSuffix(path, "/commands/batch"))
		return
	}
	if strings.HasSuffix(path, "/commands") {
		s.reportCommand(w, r, strings.TrimSuffix(path, "/commands"))
		return
	}
	format := ""
	exportFormat := ""
	assetExport := ""
	runID := path
	if strings.HasSuffix(path, "/export") {
		exportFormat = strings.TrimSpace(r.URL.Query().Get("format"))
		runID = strings.TrimSuffix(path, "/export")
	}
	if strings.HasSuffix(path, "/assets.txt") {
		assetExport = "txt"
		runID = strings.TrimSuffix(path, "/assets.txt")
	}
	if strings.HasSuffix(path, "/assets.csv") {
		assetExport = "csv"
		runID = strings.TrimSuffix(path, "/assets.csv")
	}
	if assetExport == "" && strings.HasSuffix(path, ".json") {
		format = "json"
		runID = strings.TrimSuffix(path, ".json")
	}
	if assetExport == "" && strings.HasSuffix(path, ".html") {
		format = "html"
		runID = strings.TrimSuffix(path, ".html")
	}

	run, err := s.store.GetScanRun(runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fps, err := s.store.ListFingerprints(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	findings, err := s.store.ListFindings(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var detectionChecks []report.DetectionCheck
	if run.Status != "running" {
		checks, err := s.store.ListDetectionChecks(runID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, check := range checks {
			detectionChecks = append(detectionChecks, report.DetectionCheck{
				IP: check.IP, Port: check.Port, Protocol: check.Protocol, Engine: check.Engine, CheckID: check.CheckID,
				Status: check.Status, Verdict: check.Verdict, ReasonCode: check.ReasonCode, Detail: check.Detail,
				StartedAt: report.DetectionCheckTime(check.StartedAt), FinishedAt: report.DetectionCheckTime(check.FinishedAt),
			})
		}
	}
	filters := reportFiltersFromValues(r.URL.Query())
	filteredFingerprints := filterFingerprints(fps, filters)
	filteredFindings := filterFindings(findings, fps, filters)
	filteredChecks := filterDetectionChecks(detectionChecks, filteredFingerprints)
	filteredBuilt := report.Build(filteredFingerprints, filteredFindings)
	if run.Status != "running" {
		filteredBuilt = report.BuildWithScanDataAndDetectionChecks(filteredFingerprints, filteredFindings, report.ScanData{}, filteredChecks)
	}
	if format == "html" || exportFormat == "html" {
		report.EnrichFindingsWithVulnerabilityDetails(&filteredBuilt, s.catalog)
	}
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(filteredBuilt)
	case "html":
		tmp := filepath.Join(os.TempDir(), "anchorscan-report-"+runID+".html")
		if err := report.WriteHTML(tmp, filteredBuilt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer os.Remove(tmp)
		http.ServeFile(w, r, tmp)
	default:
		if exportFormat != "" {
			filename := "anchorscan-" + runID
			switch exportFormat {
			case "json":
				w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.json"`)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(filteredBuilt)
				return
			case "html":
				tmp := filepath.Join(os.TempDir(), "anchorscan-export-"+runID+".html")
				if err := report.WriteHTML(tmp, filteredBuilt); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer os.Remove(tmp)
				w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.html"`)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				http.ServeFile(w, r, tmp)
				return
			case "csv":
				data, err := exportFindingsCSV(filteredFindings, filteredFingerprints)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				_, _ = io.WriteString(w, data)
				return
			default:
				http.Error(w, "unknown export format", http.StatusBadRequest)
				return
			}
		}
		if assetExport == "txt" {
			w.Header().Set("Content-Disposition", `attachment; filename="`+runID+`-assets.txt"`)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, exportAssetsTXT(filteredFingerprints, r.URL.Query().Get("kind")))
			return
		}
		if assetExport == "csv" {
			data, err := exportAssetsCSV(filteredFingerprints)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Disposition", `attachment; filename="`+runID+`-assets.csv"`)
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			_, _ = io.WriteString(w, data)
			return
		}
		render(w, "templates/report.html", buildReportViewModel(reportViewInput{
			Run:               run,
			Fingerprints:      filteredFingerprints,
			Findings:          filteredFindings,
			DetectionChecks:   filteredBuilt.DetectionChecks,
			DetectionCoverage: filteredBuilt.DetectionCoverage,
			Query:             r.URL.Query(),
			Catalog:           s.catalog,
			CommandTools:      s.commandTools(filteredFindings),
		}))
	}
}

type riskSummary struct {
	Total    int
	Critical int
	High     int
	Medium   int
}

func summarizeRisk(findings []report.Finding) riskSummary {
	summary := riskSummary{Total: len(findings)}
	for _, finding := range findings {
		switch strings.ToLower(finding.Severity) {
		case "critical":
			summary.Critical++
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		}
	}
	return summary
}

type commandToolView struct {
	Name  string
	Label string
}

type commandToolsView struct {
	Tools  []commandToolView
	Reason string
}

func (s *server) commandTools(findings []report.Finding) map[string]commandToolsView {
	counts := map[string]int{}
	for _, finding := range findings {
		counts[report.FindingKey(finding)]++
	}
	available := map[string]commandToolsView{}
	for _, finding := range findings {
		key := report.FindingKey(finding)
		if counts[key] != 1 {
			continue
		}
		view := commandToolsView{}
		for _, tool := range []commandToolView{{Name: "nuclei", Label: "Nuclei"}, {Name: "nmap", Label: "Nmap NSE"}, {Name: "msf", Label: "MSF"}} {
			_, err := s.buildCommand(tool.Name, finding)
			if err == nil {
				view.Tools = append(view.Tools, tool)
			}
		}
		if len(view.Tools) == 0 {
			view.Reason = s.commandUnavailableReason(finding)
		}
		available[key] = view
	}
	return available
}

func (s *server) commandUnavailableReason(finding report.Finding) string {
	match := s.catalog.Match(report.ObservationFromFinding(finding))
	if match.Status != knowledgebase.MatchMatched {
		return "知识库未匹配或匹配不唯一"
	}
	for _, diagnostic := range s.catalog.Diagnostics() {
		if diagnostic.EntryID == match.Entry.ID && strings.Contains(diagnostic.Reason, "命令无效") {
			return "知识库命令格式无效"
		}
	}
	if match.Entry.Commands.Nuclei == "" && match.Entry.Commands.NmapNSE == "" && match.Entry.Commands.Metasploit == "" {
		return "知识库未提供检测命令"
	}
	return "目标不可绑定"
}

func (s *server) reportCommand(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tool := strings.TrimSpace(r.FormValue("tool"))
	if tool != "nuclei" && tool != "nmap" && tool != "msf" {
		http.Error(w, "unsupported tool", http.StatusBadRequest)
		return
	}
	findings, err := s.store.ListFindings(runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fingerprints, err := s.store.ListFingerprints(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	findings = filterFindings(findings, fingerprints, reportFiltersFromValues(r.URL.Query()))
	key := strings.TrimSpace(r.FormValue("finding_key"))
	var matches []report.Finding
	for _, finding := range findings {
		if report.FindingKey(finding) == key {
			matches = append(matches, finding)
		}
	}
	if len(matches) != 1 {
		http.Error(w, "finding unavailable or ambiguous", http.StatusBadRequest)
		return
	}
	command, err := s.buildCommand(tool, matches[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(command)
}

func (s *server) reportBatchCommand(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid batch command request", http.StatusBadRequest)
		return
	}
	tool := strings.TrimSpace(r.FormValue("tool"))
	if tool != "nuclei" && tool != "nmap" && tool != "msf" {
		http.Error(w, "invalid batch command request", http.StatusBadRequest)
		return
	}
	findings, err := s.store.ListFindings(runID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fingerprints, err := s.store.ListFingerprints(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filtered := filterFindings(findings, fingerprints, reportFiltersFromValues(r.URL.Query()))
	if tool == "msf" {
		s.reportBatchMSFCommand(w, filtered, strings.TrimSpace(r.FormValue("group_key")))
		return
	}
	if tool == "nmap" {
		s.reportBatchNmapCommand(w, runID, filtered, strings.TrimSpace(r.FormValue("group_key")))
		return
	}
	batch, err := report.BuildBatchNucleiCommand(filtered, s.catalog, strings.TrimSpace(r.FormValue("group_key")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	contents := strings.Join(batch.Targets, "\n") + "\n"
	sum := sha256.Sum256([]byte(contents))
	dir := filepath.Dir(managedReportPath(s.opts.DBPath, "", runID))
	if run, getErr := s.store.GetScanRun(runID); getErr == nil {
		dir = filepath.Dir(managedReportPath(s.opts.DBPath, run.ProjectID, runID))
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	targetFile := filepath.Join(dir, "nuclei-targets-"+fmt.Sprintf("%x", sum[:])+".txt")
	if err := writeBatchTargetFile(targetFile, []byte(contents)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	args := append([]string(nil), batch.Args...)
	args[len(args)-2], args[len(args)-1] = "-l", targetFile
	toolArgs := displayCommandArgs(args[1:])
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"full_command": displayCommandArgs(args), "tool_args": toolArgs, "target_file": targetFile})
}

func (s *server) reportBatchMSFCommand(w http.ResponseWriter, findings []report.Finding, groupKey string) {
	commands, err := report.BuildBatchMSFCommands(findings, s.catalog, groupKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"commands": commands, "warning": "MSF 命令在外部环境逐条执行，不使用服务端目标文件"})
}

func (s *server) reportBatchNmapCommand(w http.ResponseWriter, runID string, findings []report.Finding, groupKey string) {
	batches, err := report.BuildBatchNmapCommands(findings, s.catalog, groupKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	run, err := s.store.GetScanRun(runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dir, err := filepath.Abs(filepath.Dir(managedReportPath(s.opts.DBPath, run.ProjectID, runID)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	result := make([]map[string]string, 0, len(batches))
	for _, batch := range batches {
		contents := strings.Join(batch.Targets, "\n") + "\n"
		sum := sha256.Sum256([]byte(contents))
		path := filepath.Join(dir, "nmap-targets-"+strconv.Itoa(batch.Port)+"-"+fmt.Sprintf("%x", sum[:])+".txt")
		if err := writeBatchTargetFile(path, []byte(contents)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		args := append([]string(nil), batch.Args[:len(batch.Args)-1]...)
		args = append(args, "-iL", path)
		result = append(result, map[string]string{"full_command": displayCommandArgs(args), "tool_args": displayCommandArgs(args[1:]), "target_file": path})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"commands": result, "warning": "Nmap 按实际端口分组，避免主机与端口组合扩大范围"})
}

func writeBatchTargetFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		existing, readErr := os.ReadFile(path)
		if readErr != nil || string(existing) != string(contents) {
			return fmt.Errorf("目标文件内容不一致")
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, err = file.Write(contents)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func displayCommandArgs(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"'\\") {
			arg = strconv.Quote(arg)
		}
		parts[i] = arg
	}
	return strings.Join(parts, " ")
}

func (s *server) buildCommand(tool string, finding report.Finding) (report.DetectionCommand, error) {
	switch tool {
	case "nuclei":
		return report.BuildNucleiCommand(finding, s.catalog)
	case "nmap":
		return report.BuildNmapCommand(finding, s.catalog)
	case "msf":
		return report.BuildMSFCommand(finding, s.catalog)
	}
	return report.DetectionCommand{}, fmt.Errorf("unsupported tool")
}

func reportFiltersFromValues(values url.Values) reportFilters {
	return reportFilters{
		IP:         values.Get("ip"),
		Port:       values.Get("port"),
		Service:    values.Get("service"),
		Keyword:    values.Get("q"),
		Severity:   values.Get("severity"),
		Severities: parseSeverityFilters(values),
		Source:     values.Get("source"),
	}
}
