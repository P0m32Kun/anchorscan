package report

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// DeliverableEvidence is one screenshot embedded into the project report. For
// the HTML exporter DataURI carries the base64 payload; for the DOCX exporter
// FilePath points at the on-disk image the sidecar reads.
type DeliverableEvidence struct {
	DataURI   string
	FilePath  string
	MediaType string
	Caption   string
	Width     int
	Height    int
}

// DeliverableAsset is one IP:port covered by a verification.
type DeliverableAsset struct {
	IP      string
	Port    int
	Display string
}

// DeliverableVerification is one included verification projected for the formal
// report, carrying its assets and ordered evidence.
type DeliverableVerification struct {
	ID               string
	VulnerabilityKey string
	ZoneID           string
	Title            string
	Severity         string
	Outcome          string
	Description      string
	Remediation      string
	Assets           []DeliverableAsset
	AssetsText       string
	PortsText        string
	Evidence         []DeliverableEvidence
	Position         int
}

// DeliverableZoneInfo is the minimal zone identity needed by the report.
type DeliverableZoneInfo struct {
	ZoneID    string
	Name      string
	SortOrder int
}

// DeliverableSession is one included run's on-site context.
type DeliverableSession struct {
	Label       string
	AccessPoint string
	TesterIP    string
	Targets     string
	Exclusions  string
	Notes       string
}

// DeliverableZone groups the confirmed and not-observed verifications for one
// network zone.
type DeliverableZone struct {
	Zone        DeliverableZoneInfo
	Sessions    []DeliverableSession
	Confirmed   []DeliverableVerification
	NotObserved []DeliverableVerification
}

// DeliverableSummaryRow is one row of table 3-1.
type DeliverableSummaryRow struct {
	Number   int
	Title    string
	Assets   string
	Severity string
}

// DeliverableStats holds the conclusion statistics counted from confirmed
// verifications actually present in the report.
type DeliverableStats struct {
	Total    int
	Critical int
	High     int
	Medium   int
	Low      int
}

// ProjectDeliverable is the formal, offline report model shared by the HTML and
// DOCX exporters. It is built only from included verifications and their
// evidence; raw findings and candidates never reach it.
type ProjectDeliverable struct {
	Project     ProjectMetadata
	Zones       []DeliverableZone
	Summary     []DeliverableSummaryRow
	Stats       DeliverableStats
	ZoneNames   string
	FocusText   string
	GeneratedAt time.Time
}

// BuildProjectDeliverable projects included verifications into the formal report
// model: confirmed verifications drive table 3-1 and the per-zone detail
// sections, not_observed verifications drive the per-zone negative sections,
// and inconclusive verifications are excluded from both the summary and the
// statistics. The builder is pure: evidence must already be loaded as data URIs
// by the caller.
func BuildProjectDeliverable(project ProjectMetadata, zones []ProjectZone, runs []ProjectRun, verifications []DeliverableVerification, now time.Time) ProjectDeliverable {
	zoneIndex := make(map[string]int, len(zones))
	deliverableZones := make([]DeliverableZone, len(zones))
	for i, z := range zones {
		zoneIndex[z.ZoneID] = i
		deliverableZones[i] = DeliverableZone{Zone: DeliverableZoneInfo{ZoneID: z.ZoneID, Name: z.Name, SortOrder: z.SortOrder}}
	}
	for _, run := range runs {
		idx, ok := zoneIndex[run.ZoneID]
		if !ok || !run.IncludeInReport || (run.Status != "completed" && run.Status != "completed_with_errors") {
			continue
		}
		label := strings.TrimSpace(run.Label)
		if label == "" {
			label = run.RunID
		}
		deliverableZones[idx].Sessions = append(deliverableZones[idx].Sessions, DeliverableSession{
			Label: label, AccessPoint: run.AccessPoint, TesterIP: run.TesterIP, Targets: run.Target,
			Exclusions: formatExclusions(run.ExcludeTargets, run.ExcludePorts), Notes: run.Notes,
		})
	}

	for _, input := range verifications {
		v := input
		v.Assets = normalizeDeliverableAssets(v.Assets)
		v.AssetsText = joinAssets(v.Assets)
		v.PortsText = joinPorts(v.Assets)
		idx, ok := zoneIndex[v.ZoneID]
		if !ok {
			continue
		}
		switch v.Outcome {
		case "confirmed":
			deliverableZones[idx].Confirmed = append(deliverableZones[idx].Confirmed, v)
		case "not_observed":
			v.Title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v.Title), "未发现 "))
			deliverableZones[idx].NotObserved = append(deliverableZones[idx].NotObserved, v)
		}
	}

	type summaryGroup struct {
		verification DeliverableVerification
		assets       map[string]DeliverableAsset
	}
	summaryGroups := map[string]*summaryGroup{}
	var summaryOrder []string
	stats := DeliverableStats{}
	var zoneNames []string
	for i := range deliverableZones {
		zone := &deliverableZones[i]
		sortVerifications(zone.Confirmed)
		sortVerifications(zone.NotObserved)
		if len(zone.Sessions) > 0 || len(zone.Confirmed) > 0 || len(zone.NotObserved) > 0 {
			zoneNames = append(zoneNames, zone.Zone.Name)
		}
		for _, v := range zone.Confirmed {
			key := strings.TrimSpace(v.VulnerabilityKey)
			if key == "" {
				key = v.ID
			}
			group := summaryGroups[key]
			if group == nil {
				group = &summaryGroup{verification: v, assets: map[string]DeliverableAsset{}}
				summaryGroups[key] = group
				summaryOrder = append(summaryOrder, key)
			}
			for _, asset := range v.Assets {
				group.assets[asset.IP+"\x00"+strconv.Itoa(asset.Port)] = asset
			}
		}
	}

	summary := make([]DeliverableSummaryRow, 0, len(summaryOrder))
	for i, key := range summaryOrder {
		group := summaryGroups[key]
		assets := make([]DeliverableAsset, 0, len(group.assets))
		for _, asset := range group.assets {
			assets = append(assets, asset)
		}
		sort.Slice(assets, func(i, j int) bool {
			if assets[i].IP != assets[j].IP {
				return assets[i].IP < assets[j].IP
			}
			return assets[i].Port < assets[j].Port
		})
		summary = append(summary, DeliverableSummaryRow{
			Number:   i + 1,
			Title:    strings.TrimSpace(group.verification.Title),
			Assets:   joinAssets(assets),
			Severity: severityLabel(group.verification.Severity),
		})
		stats.Total++
		switch strings.ToLower(group.verification.Severity) {
		case "critical":
			stats.Critical++
		case "high":
			stats.High++
		case "medium":
			stats.Medium++
		case "low":
			stats.Low++
		}
	}

	focusTitles := make([]string, 0, len(summary))
	for _, row := range summary {
		focusTitles = append(focusTitles, row.Title)
	}
	return ProjectDeliverable{
		Project:     project,
		Zones:       deliverableZones,
		Summary:     summary,
		Stats:       stats,
		ZoneNames:   strings.Join(zoneNames, "、"),
		FocusText:   strings.Join(focusTitles, `\`),
		GeneratedAt: now,
	}
}

func sortVerifications(items []DeliverableVerification) {
	sort.SliceStable(items, func(i, j int) bool {
		if severityRank(items[i].Severity) != severityRank(items[j].Severity) {
			return severityRank(items[i].Severity) < severityRank(items[j].Severity)
		}
		if items[i].Position != items[j].Position {
			return items[i].Position < items[j].Position
		}
		return items[i].Title < items[j].Title
	})
}

func severityRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func normalizeDeliverableAssets(assets []DeliverableAsset) []DeliverableAsset {
	byKey := make(map[string]DeliverableAsset, len(assets))
	for _, asset := range assets {
		key := asset.IP + "\x00" + strconv.Itoa(asset.Port)
		if _, exists := byKey[key]; !exists {
			byKey[key] = asset
		}
	}
	out := make([]DeliverableAsset, 0, len(byKey))
	for _, asset := range byKey {
		out = append(out, asset)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IP != out[j].IP {
			return out[i].IP < out[j].IP
		}
		return out[i].Port < out[j].Port
	})
	return out
}

func joinPorts(assets []DeliverableAsset) string {
	seen := map[int]struct{}{}
	ports := make([]int, 0, len(assets))
	for _, asset := range assets {
		if asset.Port <= 0 {
			continue
		}
		if _, exists := seen[asset.Port]; exists {
			continue
		}
		seen[asset.Port] = struct{}{}
		ports = append(ports, asset.Port)
	}
	sort.Ints(ports)
	parts := make([]string, len(ports))
	for i, port := range ports {
		parts[i] = strconv.Itoa(port)
	}
	return strings.Join(parts, "、")
}

func formatExclusions(targets, ports string) string {
	var parts []string
	if targets = strings.TrimSpace(targets); targets != "" {
		parts = append(parts, "目标："+targets)
	}
	if ports = strings.TrimSpace(ports); ports != "" {
		parts = append(parts, "端口："+ports)
	}
	return strings.Join(parts, "；")
}

func joinAssets(assets []DeliverableAsset) string {
	parts := make([]string, 0, len(assets))
	for _, a := range assets {
		if display := strings.TrimSpace(a.Display); display != "" {
			parts = append(parts, display)
		}
	}
	return strings.Join(parts, ", ")
}

func severityLabel(value string) string {
	switch strings.ToLower(value) {
	case "critical":
		return "严重"
	case "high":
		return "高危"
	case "medium":
		return "中危"
	case "low":
		return "低危"
	default:
		return strings.TrimSpace(value)
	}
}
