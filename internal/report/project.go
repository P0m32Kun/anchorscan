package report

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
)

// ProjectFinding is a single finding observed in an included project run,
// carrying the run and zone context needed for cross-run aggregation.
type ProjectFinding struct {
	RunID  string
	ZoneID string
	Finding
}

// ProjectFingerprint is a single service fingerprint observed in an included
// project run, carrying the run and zone context needed for aggregation.
type ProjectFingerprint struct {
	RunID  string
	ZoneID string
	fingerprint.ServiceFingerprint
}

// ProjectDetectionCheck is a single detection-check execution observed in an
// included project run, carrying the run and zone context needed for
// negative-candidate and incomplete-check classification.
type ProjectDetectionCheck struct {
	RunID  string
	ZoneID string
	DetectionCheck
}

// ProjectMetadata holds the report-level fields of a project.
type ProjectMetadata struct {
	ID          string
	Name        string
	Description string
	ClientUnit  string
	ReportTitle string
	TestObject  string
	StartDate   string
	EndDate     string
	Testers     string
	CreatedAt   time.Time
}

// ProjectZone describes a network zone inside a project.
type ProjectZone struct {
	ZoneID    string
	Name      string
	SortOrder int
}

// ProjectRun describes a run that may be included in a project report.
type ProjectRun struct {
	RunID           string
	ZoneID          string
	Status          string
	IncludeInReport bool
	Label           string
	AccessPoint     string
	TesterIP        string
	Target          string
	ExcludeTargets  string
	Ports           string
	ExcludePorts    string
	Profile         string
	Notes           string
}

// ProjectReportInput is the single seam used to build a project report: all
// cross-run facts are gathered once and passed in; the builder does not query
// stores or read report.json files.
type ProjectReportInput struct {
	Project         ProjectMetadata
	Zones           []ProjectZone
	Runs            []ProjectRun
	Findings        []ProjectFinding
	Fingerprints    []ProjectFingerprint
	DetectionChecks []ProjectDetectionCheck
	Catalog         *knowledgebase.Catalog
}

// ProjectReport is the pure read model for a project report. It contains the
// project metadata, zones, and aggregated vulnerability candidates.
type ProjectReport struct {
	Project ProjectMetadata
	Zones   []ProjectZoneReport
}

// ProjectZoneReport groups runs and candidates for one network zone.
type ProjectZoneReport struct {
	Zone               ProjectZone
	Runs               []ProjectRun
	PositiveCandidates []ProjectVulnerabilityCandidate
	NegativeCandidates []ProjectNegativeCandidate
	IncompleteChecks   []ProjectIncompleteCheck
}

// ProjectCandidateSource records the source finding behind one candidate asset.
type ProjectCandidateSource struct {
	RunID     string
	Source    string
	FindingID string
	IP        string
	Port      int
	Protocol  string
}

// ProjectVulnerabilityCandidate is a positive finding candidate aggregated
// across included runs. Matched candidates are keyed by knowledge-base entry
// ID; pending candidates are keyed by the pending delivery key and never
// guess a knowledge-base entry.
type ProjectVulnerabilityCandidate struct {
	GroupKey    string
	Title       string
	Severity    knowledgebase.Severity
	Description string
	Remediation string
	Assets      []ProjectAsset
	Services    []string
	SourceRuns  []string
	Sources     []ProjectCandidateSource
	IsPending   bool
	PendingKey  string
}

// ProjectAsset is a single IP:port observed in one or more included runs.
type ProjectAsset struct {
	IP       string
	Port     int
	Protocol string
	Target   string
	RunIDs   []string
}

// ProjectNegativeCandidate is a fingerprint endpoint whose NSE and nuclei
// checks both completed and has no non-info findings.
type ProjectNegativeCandidate struct {
	Asset       ProjectAsset
	Fingerprint fingerprint.ServiceFingerprint
	RunID       string
	ZoneID      string
}

// ProjectIncompleteCheck is a fingerprint endpoint that cannot be treated as a
// negative candidate because at least one required detection check is missing,
// failed, skipped, canceled or interrupted.
type ProjectIncompleteCheck struct {
	Asset       ProjectAsset
	Fingerprint fingerprint.ServiceFingerprint
	RunID       string
	ZoneID      string
	Engines     []ProjectEngineStatus
}

// ProjectEngineStatus describes the status of a single detection engine for an
// incomplete check.
type ProjectEngineStatus struct {
	Engine     string
	Status     string
	ReasonCode string
	Detail     string
}

// BuildProjectReport aggregates cross-run findings, fingerprints and detection
// checks into a zone-grouped project report. It is the only place where
// cross-run aggregation, knowledge-base matching, and deduplication happen.
func BuildProjectReport(input ProjectReportInput) (ProjectReport, error) {
	allowedRuns := map[string]ProjectRun{}
	for _, run := range input.Runs {
		if run.IncludeInReport && (run.Status == "completed" || run.Status == "completed_with_errors") {
			allowedRuns[run.RunID] = run
		}
	}

	zoneOrder := map[string]int{}
	zones := make([]ProjectZoneReport, len(input.Zones))
	for i, z := range input.Zones {
		zones[i] = ProjectZoneReport{Zone: z}
		zoneOrder[z.ZoneID] = i
	}
	for _, run := range allowedRuns {
		if i, ok := zoneOrder[run.ZoneID]; ok {
			zones[i].Runs = append(zones[i].Runs, run)
		}
	}
	for i := range zones {
		sort.Slice(zones[i].Runs, func(a, b int) bool {
			return zones[i].Runs[a].RunID < zones[i].Runs[b].RunID
		})
	}

	positiveByZone := buildPositiveCandidates(input, allowedRuns, zoneOrder)
	for i, candidates := range positiveByZone {
		zones[i].PositiveCandidates = candidates
	}

	negativeByZone, incompleteByZone := buildNegativeAndIncomplete(input, allowedRuns, zoneOrder)
	for i, candidates := range negativeByZone {
		zones[i].NegativeCandidates = candidates
	}
	for i, checks := range incompleteByZone {
		zones[i].IncompleteChecks = checks
	}

	return ProjectReport{Project: input.Project, Zones: zones}, nil
}

type posGroup struct {
	key         string
	title       string
	severity    knowledgebase.Severity
	description string
	remediation string
	assets      map[string]ProjectAsset
	runs        map[string]struct{}
	services    map[string]struct{}
	sources     []ProjectCandidateSource
	isPending   bool
	pendingKey  string
}

func buildPositiveCandidates(input ProjectReportInput, allowedRuns map[string]ProjectRun, zoneOrder map[string]int) [][]ProjectVulnerabilityCandidate {
	zoneGroups := map[string]map[string]*posGroup{}

	fpByRunPort := map[string]map[string]fingerprint.ServiceFingerprint{}
	for _, fp := range input.Fingerprints {
		if _, ok := allowedRuns[fp.RunID]; !ok {
			continue
		}
		if fpByRunPort[fp.RunID] == nil {
			fpByRunPort[fp.RunID] = map[string]fingerprint.ServiceFingerprint{}
		}
		fpByRunPort[fp.RunID][protocolPortKey(fp.IP, fp.Port, fp.Protocol)] = fp.ServiceFingerprint
	}

	for _, pf := range input.Findings {
		if _, ok := allowedRuns[pf.RunID]; !ok {
			continue
		}
		if strings.ToLower(pf.Severity) == "info" {
			continue
		}
		zoneIdx, ok := zoneOrder[pf.ZoneID]
		if !ok {
			continue
		}
		zoneID := input.Zones[zoneIdx].ZoneID

		obs := ObservationFromFinding(pf.Finding)
		var match knowledgebase.MatchResult
		if input.Catalog != nil {
			match = input.Catalog.Match(obs)
		}

		var g *posGroup
		if match.Status == knowledgebase.MatchMatched {
			key := match.Entry.ID
			if zoneGroups[zoneID] == nil {
				zoneGroups[zoneID] = map[string]*posGroup{}
			}
			g = zoneGroups[zoneID][key]
			if g == nil {
				g = &posGroup{
					key:         key,
					title:       match.Entry.Name,
					severity:    match.Entry.Severity,
					description: match.Entry.Description,
					remediation: match.Entry.Remediation,
					assets:      map[string]ProjectAsset{},
					runs:        map[string]struct{}{},
					services:    map[string]struct{}{},
				}
				zoneGroups[zoneID][key] = g
			}
		} else {
			key := pendingDeliveryKey(pf.Finding)
			if zoneGroups[zoneID] == nil {
				zoneGroups[zoneID] = map[string]*posGroup{}
			}
			g = zoneGroups[zoneID][key]
			severity := knowledgebase.Severity(strings.ToLower(pf.Severity))
			if g == nil {
				g = &posGroup{
					key:        key,
					title:      pendingDeliveryName(pf.Finding),
					severity:   severity,
					assets:     map[string]ProjectAsset{},
					runs:       map[string]struct{}{},
					services:   map[string]struct{}{},
					isPending:  true,
					pendingKey: key,
				}
				zoneGroups[zoneID][key] = g
			} else if deliverySeverityRank(severity) < deliverySeverityRank(g.severity) {
				g.severity = severity
			}
		}

		assetHost := normalizeProjectHost(pf.IP, pf.Target)
		assetKey := hostPortKey(assetHost, pf.Port)
		existing, ok := g.assets[assetKey]
		if !ok {
			g.assets[assetKey] = ProjectAsset{IP: assetHost, Port: pf.Port, Protocol: pf.Protocol, Target: pf.Target, RunIDs: []string{pf.RunID}}
		} else {
			existing.RunIDs = appendRunID(existing.RunIDs, pf.RunID)
			if existing.Target == "" {
				existing.Target = pf.Target
			}
			g.assets[assetKey] = existing
		}

		if fp, ok := fpByRunPort[pf.RunID][protocolPortKey(assetHost, pf.Port, pf.Protocol)]; ok {
			if service := strings.TrimSpace(fp.Service); service != "" {
				g.services[service] = struct{}{}
			}
			asset := g.assets[assetKey]
			if asset.Target == "" && fp.IsWeb && fp.URL != "" {
				asset.Target = fp.URL
			}
			if asset.Target == "" && (asset.Protocol == "http" || asset.Protocol == "https") {
				asset.Target = asset.Protocol + "://" + net.JoinHostPort(asset.IP, strconv.Itoa(asset.Port))
			}
			g.assets[assetKey] = asset
		}

		g.sources = append(g.sources, ProjectCandidateSource{
			RunID:     pf.RunID,
			Source:    pf.Source,
			FindingID: pf.ID,
			IP:        assetHost,
			Port:      pf.Port,
			Protocol:  pf.Protocol,
		})
		g.runs[pf.RunID] = struct{}{}
	}

	result := make([][]ProjectVulnerabilityCandidate, len(input.Zones))
	for zoneID, groups := range zoneGroups {
		idx := zoneOrder[zoneID]
		items := make([]ProjectVulnerabilityCandidate, 0, len(groups))
		for _, g := range groups {
			items = append(items, ProjectVulnerabilityCandidate{
				GroupKey:    g.key,
				Title:       g.title,
				Severity:    g.severity,
				Description: g.description,
				Remediation: g.remediation,
				Assets:      sortAssets(g.assets),
				Services:    sortStringsFromSet(g.services),
				SourceRuns:  sortRuns(g.runs),
				Sources:     sortCandidateSources(g.sources),
				IsPending:   g.isPending,
				PendingKey:  g.pendingKey,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			left, right := deliverySeverityRank(items[i].Severity), deliverySeverityRank(items[j].Severity)
			if left != right {
				return left < right
			}
			if items[i].Title != items[j].Title {
				return items[i].Title < items[j].Title
			}
			return items[i].GroupKey < items[j].GroupKey
		})
		result[idx] = items
	}
	return result
}

func buildNegativeAndIncomplete(input ProjectReportInput, allowedRuns map[string]ProjectRun, zoneOrder map[string]int) ([][]ProjectNegativeCandidate, [][]ProjectIncompleteCheck) {
	nonInfoByRunPort := map[string]map[string]struct{}{}
	for _, pf := range input.Findings {
		if _, ok := allowedRuns[pf.RunID]; !ok {
			continue
		}
		if strings.ToLower(pf.Severity) == "info" {
			continue
		}
		if nonInfoByRunPort[pf.RunID] == nil {
			nonInfoByRunPort[pf.RunID] = map[string]struct{}{}
		}
		key := hostPortKey(normalizeProjectHost(pf.IP, pf.Target), pf.Port)
		nonInfoByRunPort[pf.RunID][key] = struct{}{}
	}

	checksByRunPort := map[string]map[string]map[string]ProjectDetectionCheck{}
	for _, pc := range input.DetectionChecks {
		if _, ok := allowedRuns[pc.RunID]; !ok {
			continue
		}
		if checksByRunPort[pc.RunID] == nil {
			checksByRunPort[pc.RunID] = map[string]map[string]ProjectDetectionCheck{}
		}
		key := portKey(pc.IP, pc.Port, pc.Protocol)
		if checksByRunPort[pc.RunID][key] == nil {
			checksByRunPort[pc.RunID][key] = map[string]ProjectDetectionCheck{}
		}
		checksByRunPort[pc.RunID][key][pc.Engine] = pc
	}

	negative := make([][]ProjectNegativeCandidate, len(input.Zones))
	incomplete := make([][]ProjectIncompleteCheck, len(input.Zones))

	for _, fp := range input.Fingerprints {
		if _, ok := allowedRuns[fp.RunID]; !ok {
			continue
		}
		idx, ok := zoneOrder[fp.ZoneID]
		if !ok {
			continue
		}
		zoneID := input.Zones[idx].ZoneID

		assetHost := normalizeProjectHost(fp.IP, "")
		portKey := protocolPortKey(assetHost, fp.Port, fp.Protocol)
		anyKey := hostPortKey(assetHost, fp.Port)

		if _, hasFinding := nonInfoByRunPort[fp.RunID][anyKey]; hasFinding {
			continue
		}

		checks := checksByRunPort[fp.RunID][portKey]
		asset := ProjectAsset{IP: assetHost, Port: fp.Port, Protocol: fp.Protocol, RunIDs: []string{fp.RunID}}

		nse := checks["nse"]
		nuclei := checks["nuclei"]
		if nse.Status == "completed" && nuclei.Status == "completed" {
			negative[idx] = append(negative[idx], ProjectNegativeCandidate{
				Asset:       asset,
				Fingerprint: fp.ServiceFingerprint,
				RunID:       fp.RunID,
				ZoneID:      zoneID,
			})
			continue
		}

		engines := []ProjectEngineStatus{}
		for _, engine := range []string{"nse", "nuclei"} {
			check, ok := checks[engine]
			if !ok {
				engines = append(engines, ProjectEngineStatus{Engine: engine, Status: "missing"})
			} else if check.Status != "completed" {
				engines = append(engines, ProjectEngineStatus{Engine: engine, Status: check.Status, ReasonCode: check.ReasonCode, Detail: check.Detail})
			}
		}
		for engine, check := range checks {
			if engine == "nse" || engine == "nuclei" {
				continue
			}
			if check.Status != "completed" {
				engines = append(engines, ProjectEngineStatus{Engine: engine, Status: check.Status, ReasonCode: check.ReasonCode, Detail: check.Detail})
			}
		}
		if len(engines) > 0 {
			incomplete[idx] = append(incomplete[idx], ProjectIncompleteCheck{
				Asset:       asset,
				Fingerprint: fp.ServiceFingerprint,
				RunID:       fp.RunID,
				ZoneID:      zoneID,
				Engines:     engines,
			})
		}
	}

	for i := range negative {
		sort.Slice(negative[i], func(a, b int) bool {
			if negative[i][a].Asset.IP != negative[i][b].Asset.IP {
				return negative[i][a].Asset.IP < negative[i][b].Asset.IP
			}
			if negative[i][a].Asset.Port != negative[i][b].Asset.Port {
				return negative[i][a].Asset.Port < negative[i][b].Asset.Port
			}
			return negative[i][a].RunID < negative[i][b].RunID
		})
	}
	for i := range incomplete {
		sort.Slice(incomplete[i], func(a, b int) bool {
			if incomplete[i][a].Asset.IP != incomplete[i][b].Asset.IP {
				return incomplete[i][a].Asset.IP < incomplete[i][b].Asset.IP
			}
			if incomplete[i][a].Asset.Port != incomplete[i][b].Asset.Port {
				return incomplete[i][a].Asset.Port < incomplete[i][b].Asset.Port
			}
			return incomplete[i][a].RunID < incomplete[i][b].RunID
		})
	}
	return negative, incomplete
}

func sortAssets(assets map[string]ProjectAsset) []ProjectAsset {
	items := make([]ProjectAsset, 0, len(assets))
	for _, a := range assets {
		sort.Strings(a.RunIDs)
		items = append(items, a)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IP != items[j].IP {
			return items[i].IP < items[j].IP
		}
		return items[i].Port < items[j].Port
	})
	return items
}

func sortRuns(runs map[string]struct{}) []string {
	items := make([]string, 0, len(runs))
	for r := range runs {
		items = append(items, r)
	}
	sort.Strings(items)
	return items
}

func sortStringsFromSet(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for v := range values {
		items = append(items, v)
	}
	sort.Strings(items)
	return items
}

func sortCandidateSources(sources []ProjectCandidateSource) []ProjectCandidateSource {
	out := append([]ProjectCandidateSource(nil), sources...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].RunID != out[j].RunID {
			return out[i].RunID < out[j].RunID
		}
		if out[i].IP != out[j].IP {
			return out[i].IP < out[j].IP
		}
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].FindingID < out[j].FindingID
	})
	return out
}

func appendRunID(ids []string, id string) []string {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func normalizeProjectHost(ip, target string) string {
	host := strings.TrimSpace(ip)
	if host == "" {
		if h, _, err := net.SplitHostPort(strings.TrimSpace(target)); err == nil {
			host = h
		} else {
			host = strings.TrimSpace(target)
		}
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String()
	}
	return host
}

func hostPortKey(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func protocolPortKey(ip string, port int, protocol string) string {
	return fmt.Sprintf("%s:%d:%s", ip, port, protocol)
}
