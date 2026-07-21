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
	Title                string `json:"title"`
	ClientName           string `json:"client_name"`
	TestSubject          string `json:"test_subject"`
	ProjectCreatedDate   string `json:"project_created_date"`
	ProjectCreatedMonth  string `json:"project_created_month"`
	TestersText          string `json:"testers_text"`
	TestPeriod           string `json:"test_period"`
}

type docxSummaryRow struct {
	Number        int    `json:"number"`
	Title         string `json:"title"`
	AssetsText    string `json:"assets_text"`
	SeverityLabel string `json:"severity_label"`
}

type docxZone struct {
	Name         string            `json:"name"`
	Sessions     []docxSession     `json:"sessions"`
	Confirmed    []docxVerification `json:"confirmed"`
	NotObserved  []docxVerification `json:"not_observed"`
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
	Heading     string       `json:"heading,omitempty"`
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	AssetsText  string       `json:"assets_text,omitempty"`
	Remediation string       `json:"remediation,omitempty"`
	PortsText   string       `json:"ports_text,omitempty"`
	Evidence    []docxEvidence `json:"evidence"`
}

type docxEvidence struct {
	Path string `json:"path"`
}

type docxConclusion struct {
	NetworkZoneNamesText string `json:"network_zone_names_text"`
	Total                int    `json:"total"`
	High                 int    `json:"high"`
	Medium               int    `json:"medium"`
	Low                  int    `json:"low"`
	FocusText            string `json:"focus_text"`
}

// BuildDocxContext projects a deliverable into the sidecar's JSON contract.
// The cover date/month default to the generation time when the project did not
// record explicit dates. Sessions are left empty: the deliverable does not
// carry per-run access metadata, and the template renders an empty loop.
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
		reportCtx.ProjectCreatedDate = period
	} else {
		reportCtx.ProjectCreatedDate = now.Format("2006年1月2日")
	}
	reportCtx.ProjectCreatedMonth = chineseYearMonth(now)

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
		if len(z.Confirmed) == 0 && len(z.NotObserved) == 0 {
			continue
		}
		zone := docxZone{Name: z.Zone.Name}
		for _, v := range z.Confirmed {
			zone.Confirmed = append(zone.Confirmed, docxVerification{
				Heading:     v.Title + "（" + severityLabel(v.Severity) + "）",
				Description: v.Description,
				AssetsText:  joinAssets(v.Assets),
				Remediation: v.Remediation,
				Evidence:    evidencePaths(v.Evidence),
			})
		}
		for _, v := range z.NotObserved {
			zone.NotObserved = append(zone.NotObserved, docxVerification{
				Title:     v.Title,
				PortsText: joinAssets(v.Assets),
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
			High:                 deliverable.Stats.High,
			Medium:               deliverable.Stats.Medium,
			Low:                  deliverable.Stats.Low,
			FocusText:            focusText(deliverable),
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

func focusText(deliverable ProjectDeliverable) string {
	seen := map[string]struct{}{}
	var titles []string
	for _, z := range deliverable.Zones {
		for _, v := range z.Confirmed {
			title := strings.TrimSpace(v.Title)
			if title == "" {
				continue
			}
			if _, ok := seen[title]; ok {
				continue
			}
			seen[title] = struct{}{}
			titles = append(titles, title)
		}
	}
	return strings.Join(titles, "、")
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
