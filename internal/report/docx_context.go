package report

import (
	"strings"
	"time"
)

// DocxContext is the JSON-shaped contract consumed by the docxtpl sidecar and
// its version-managed template. It mirrors fixtures/project_report.json; every
// evidence entry carries an absolute FilePath the sidecar resolves.
type DocxContext struct {
	Report       docxReport       `json:"report"`
	SummaryRows  []docxSummaryRow `json:"summary_rows"`
	SummaryEmpty bool             `json:"summary_empty"`
	NetworkZones []docxZone       `json:"network_zones"`
	Conclusion   docxConclusion   `json:"conclusion"`
}

type docxReport struct {
	Title               string `json:"title"`
	ClientName          string `json:"client_name"`
	TestSubject         string `json:"test_subject"`
	ProjectCreatedDate  string `json:"project_created_date"`
	ProjectCreatedMonth string `json:"project_created_month"`
	TestersText         string `json:"testers_text"`
	TestPeriod          string `json:"test_period"`
}

type docxSummaryRow struct {
	Number        int    `json:"number"`
	Title         string `json:"title"`
	AssetsText    string `json:"assets_text"`
	SeverityLabel string `json:"severity_label"`
}

type docxZone struct {
	Name        string             `json:"name"`
	Sessions    []docxSession      `json:"sessions"`
	Confirmed   []docxVerification `json:"confirmed"`
	NotObserved []docxVerification `json:"not_observed"`
}

type docxSession struct {
	Label          string `json:"label"`
	AccessPoint    string `json:"access_point"`
	TesterIP       string `json:"tester_ip"`
	TargetsText    string `json:"targets_text"`
	ExclusionsText string `json:"exclusions_text"`
	Notes          string `json:"notes"`
}

type docxVerification struct {
	Heading     string         `json:"heading,omitempty"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	AssetsText  string         `json:"assets_text,omitempty"`
	Remediation string         `json:"remediation,omitempty"`
	PortsText   string         `json:"ports_text,omitempty"`
	Evidence    []docxEvidence `json:"evidence"`
}

type docxEvidence struct {
	Path string `json:"path"`
}

type docxConclusion struct {
	NetworkZoneNamesText string `json:"network_zone_names_text"`
	Total                int    `json:"total"`
	Critical             int    `json:"critical"`
	High                 int    `json:"high"`
	Medium               int    `json:"medium"`
	Low                  int    `json:"low"`
	FocusText            string `json:"focus_text"`
}

// BuildDocxContext projects a deliverable into the sidecar's JSON contract.
// The cover date/month default to the generation time when the project did not
// record an explicit creation timestamp.
func BuildDocxContext(deliverable ProjectDeliverable, now time.Time) DocxContext {
	reportCtx := docxReport{
		Title:       deliverable.Project.ReportTitle,
		ClientName:  deliverable.Project.ClientUnit,
		TestSubject: deliverable.Project.TestObject,
		TestersText: deliverable.Project.Testers,
	}
	period := strings.TrimSpace(deliverable.Project.StartDate)
	end := strings.TrimSpace(deliverable.Project.EndDate)
	if period != "" && end != "" {
		reportCtx.TestPeriod = period + " 至 " + end
	}
	createdAt := deliverable.Project.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	reportCtx.ProjectCreatedDate = createdAt.Format("2006年1月2日")
	reportCtx.ProjectCreatedMonth = chineseYearMonth(createdAt)

	summaryRows := make([]docxSummaryRow, 0, len(deliverable.Summary))
	for _, row := range deliverable.Summary {
		summaryRows = append(summaryRows, docxSummaryRow{
			Number:        row.Number,
			Title:         row.Title,
			AssetsText:    row.Assets,
			SeverityLabel: row.Severity,
		})
	}

	zones := make([]docxZone, 0, len(deliverable.Zones))
	for _, z := range deliverable.Zones {
		if len(z.Sessions) == 0 && len(z.Confirmed) == 0 && len(z.NotObserved) == 0 {
			continue
		}
		zone := docxZone{Name: z.Zone.Name}
		for _, session := range z.Sessions {
			zone.Sessions = append(zone.Sessions, docxSession{
				Label: session.Label, AccessPoint: session.AccessPoint, TesterIP: session.TesterIP, TargetsText: session.Targets, ExclusionsText: session.Exclusions, Notes: session.Notes,
			})
		}
		for _, v := range z.Confirmed {
			zone.Confirmed = append(zone.Confirmed, docxVerification{
				Heading:     v.Title + "（" + severityLabel(v.Severity) + "）",
				Description: v.Description,
				AssetsText:  v.AssetsText,
				Remediation: v.Remediation,
				Evidence:    evidencePaths(v.Evidence),
			})
		}
		for _, v := range z.NotObserved {
			zone.NotObserved = append(zone.NotObserved, docxVerification{
				Title:     v.Title,
				PortsText: v.PortsText,
				Evidence:  evidencePaths(v.Evidence),
			})
		}
		zones = append(zones, zone)
	}

	return DocxContext{
		Report:       reportCtx,
		SummaryRows:  summaryRows,
		SummaryEmpty: len(summaryRows) == 0,
		NetworkZones: zones,
		Conclusion: docxConclusion{
			NetworkZoneNamesText: deliverable.ZoneNames,
			Total:                deliverable.Stats.Total,
			Critical:             deliverable.Stats.Critical,
			High:                 deliverable.Stats.High,
			Medium:               deliverable.Stats.Medium,
			Low:                  deliverable.Stats.Low,
			FocusText:            deliverable.FocusText,
		},
	}
}

func evidencePaths(items []DeliverableEvidence) []docxEvidence {
	out := make([]docxEvidence, 0, len(items))
	for _, e := range items {
		if path := strings.TrimSpace(e.FilePath); path != "" {
			out = append(out, docxEvidence{Path: path})
		}
	}
	return out
}

func chineseYearMonth(t time.Time) string {
	return formatChineseYear(t.Year()) + "年" + chineseMonth(int(t.Month()))
}

func formatChineseYear(year int) string {
	digits := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if year == 0 {
		return digits[0]
	}
	var out []rune
	rest := year
	for rest > 0 {
		out = append([]rune(digits[rest%10]), out...)
		rest /= 10
	}
	return string(out)
}

func chineseMonth(month int) string {
	names := []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十", "十一", "十二"}
	if month < 1 || month > 12 {
		return ""
	}
	return names[month] + "月"
}
