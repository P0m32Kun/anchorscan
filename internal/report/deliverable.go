package report

import (
	"sort"
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
	ID          string
	ZoneID      string
	Title       string
	Severity    string
	Outcome     string
	Description string
	Remediation string
	Assets      []DeliverableAsset
	Evidence    []DeliverableEvidence
	Position    int
}

// DeliverableZoneInfo is the minimal zone identity needed by the report.
type DeliverableZoneInfo struct {
	ZoneID    string
	Name      string
	SortOrder int
}

// DeliverableZone groups the confirmed and not-observed verifications for one
// network zone.
type DeliverableZone struct {
	Zone        DeliverableZoneInfo
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
	GeneratedAt time.Time
}

// BuildProjectDeliverable projects included verifications into the formal report
// model: confirmed verifications drive table 3-1 and the per-zone detail
// sections, not_observed verifications drive the per-zone negative sections,
// and inconclusive verifications are excluded from both the summary and the
// statistics. The builder is pure: evidence must already be loaded as data URIs
// by the caller.
func BuildProjectDeliverable(project ProjectMetadata, zones []ProjectZone, verifications []DeliverableVerification, now time.Time) ProjectDeliverable {
	zoneIndex := make(map[string]int, len(zones))
	deliverableZones := make([]DeliverableZone, len(zones))
	for i, z := range zones {
		zoneIndex[z.ZoneID] = i
		deliverableZones[i] = DeliverableZone{Zone: DeliverableZoneInfo{ZoneID: z.ZoneID, Name: z.Name, SortOrder: z.SortOrder}}
	}

	for _, v := range verifications {
		idx, ok := zoneIndex[v.ZoneID]
		if !ok {
			continue
		}
		switch v.Outcome {
		case "confirmed":
			deliverableZones[idx].Confirmed = append(deliverableZones[idx].Confirmed, v)
		case "not_observed":
			deliverableZones[idx].NotObserved = append(deliverableZones[idx].NotObserved, v)
		}
	}

	summary := []DeliverableSummaryRow{}
	stats := DeliverableStats{}
	number := 0
	var zoneNames []string
	for i := range deliverableZones {
		zone := &deliverableZones[i]
		sortVerifications(zone.Confirmed)
		sortVerifications(zone.NotObserved)
		if len(zone.Confirmed) > 0 || len(zone.NotObserved) > 0 {
			zoneNames = append(zoneNames, zone.Zone.Name)
		}
		for _, v := range zone.Confirmed {
			number++
			summary = append(summary, DeliverableSummaryRow{
				Number:   number,
				Title:    strings.TrimSpace(v.Title),
				Assets:   joinAssets(v.Assets),
				Severity: severityLabel(v.Severity),
			})
			stats.Total++
			switch strings.ToLower(v.Severity) {
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
	}

	return ProjectDeliverable{
		Project:     project,
		Zones:       deliverableZones,
		Summary:     summary,
		Stats:       stats,
		ZoneNames:   strings.Join(zoneNames, "、"),
		GeneratedAt: now,
	}
}

// zoneIDOf is no longer needed: the zone id is carried on DeliverableVerification.ZoneID.

func sortVerifications(items []DeliverableVerification) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Position != items[j].Position {
			return items[i].Position < items[j].Position
		}
		return items[i].Title < items[j].Title
	})
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
