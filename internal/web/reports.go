package web

import (
	"encoding/csv"
	"net/url"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

const runMetaSummaryLimit = 80

type reportFilters struct {
	IP         string
	Port       string
	Service    string
	Keyword    string
	Severity   string
	Severities []string
	Source     string
}

type reportPage struct {
	Items      any
	Page       int
	TotalPages int
	HasPrev    bool
	HasNext    bool
	PrevURL    string
	NextURL    string
	Size       int
	SizeStr    string
	SizeURLs   map[int]string
}

var reportPageSizes = []int{10, 20, 50}
var supportedSeverities = []string{"critical", "high", "medium", "low", "info"}

type hostAssetView struct {
	IP        string
	Ports     string
	Services  string
	Products  string
	URLs      string
	CopyPorts string
	CopyPairs string
}

type runMetaView struct {
	Target      string
	Ports       string
	Profile     string
	FullTarget  string
	FullPorts   string
	FullProfile string
}

func newRunMetaView(run store.ScanRun) runMetaView {
	return runMetaView{
		Target:      summarizeRunValue(run.Target),
		Ports:       summarizeRunValue(run.Ports),
		Profile:     summarizeRunValue(run.Profile),
		FullTarget:  run.Target,
		FullPorts:   run.Ports,
		FullProfile: run.Profile,
	}
}

func summarizeRunValue(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= runMetaSummaryLimit {
		return value
	}
	return string(runes[:runMetaSummaryLimit]) + "..."
}

func filterFingerprints(items []fingerprint.ServiceFingerprint, filters reportFilters) []fingerprint.ServiceFingerprint {
	var out []fingerprint.ServiceFingerprint
	for _, item := range items {
		if filters.IP != "" && item.IP != filters.IP {
			continue
		}
		if filters.Service != "" && item.Service != filters.Service {
			continue
		}
		if filters.Port != "" {
			port, err := strconv.Atoi(filters.Port)
			if err != nil || item.Port != port {
				continue
			}
		}
		if filters.Keyword != "" && !fingerprintMatchesKeyword(item, filters.Keyword) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterFindings(items []report.Finding, fps []fingerprint.ServiceFingerprint, filters reportFilters) []report.Finding {
	var out []report.Finding
	for _, item := range items {
		if filters.IP != "" && item.IP != filters.IP {
			continue
		}
		if filters.Service != "" && !findingMatchesService(item, fps, filters.Service) {
			continue
		}
		if len(filters.Severities) > 0 && !containsValue(filters.Severities, item.Severity) {
			continue
		}
		if len(filters.Severities) == 0 && filters.Severity != "" && item.Severity != filters.Severity {
			continue
		}
		if filters.Source != "" && item.Source != filters.Source {
			continue
		}
		if filters.Port != "" {
			port, err := strconv.Atoi(filters.Port)
			if err != nil || item.Port != port {
				continue
			}
		}
		if filters.Keyword != "" && !findingMatchesKeyword(item, fps, filters.Keyword) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func containsValue(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func findingMatchesService(item report.Finding, fps []fingerprint.ServiceFingerprint, service string) bool {
	for _, fp := range fps {
		if fp.IP == item.IP && fp.Port == item.Port && fp.Service == service {
			return true
		}
	}
	return false
}

func fingerprintMatchesKeyword(item fingerprint.ServiceFingerprint, keyword string) bool {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return true
	}
	haystack := []string{
		item.IP,
		strconv.Itoa(item.Port),
		item.Service,
		item.Product,
		item.Version,
		item.ExtraInfo,
		item.Normalized,
		item.URL,
	}
	for _, field := range haystack {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func findingMatchesKeyword(item report.Finding, fps []fingerprint.ServiceFingerprint, keyword string) bool {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return true
	}
	haystack := []string{
		item.IP,
		strconv.Itoa(item.Port),
		item.Source,
		item.ID,
		item.Severity,
		item.Summary,
		item.Target,
		item.Output,
	}
	for _, field := range haystack {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	for _, fp := range fps {
		if fp.IP == item.IP && fp.Port == item.Port && fingerprintMatchesKeyword(fp, keyword) {
			return true
		}
	}
	return false
}

func groupFingerprintsByHost(items []fingerprint.ServiceFingerprint) []hostAssetView {
	type hostAccumulator struct {
		ip       string
		ports    []string
		services []string
		products []string
		urls     []string
		pairs    []string
	}
	order := make([]string, 0)
	hostMap := map[string]*hostAccumulator{}
	for _, item := range items {
		host := hostMap[item.IP]
		if host == nil {
			host = &hostAccumulator{ip: item.IP}
			hostMap[item.IP] = host
			order = append(order, item.IP)
		}
		port := strconv.Itoa(item.Port)
		host.ports = append(host.ports, port)
		host.services = appendUnique(host.services, item.Service)
		host.products = appendUnique(host.products, item.Product)
		if item.URL != "" {
			host.urls = append(host.urls, item.URL)
		}
		host.pairs = append(host.pairs, item.IP+":"+port)
	}

	out := make([]hostAssetView, 0, len(order))
	for _, ip := range order {
		host := hostMap[ip]
		out = append(out, hostAssetView{
			IP:        host.ip,
			Ports:     strings.Join(host.ports, ","),
			Services:  strings.Join(host.services, ", "),
			Products:  strings.Join(host.products, ", "),
			URLs:      strings.Join(host.urls, "\n"),
			CopyPorts: strings.Join(host.ports, ","),
			CopyPairs: strings.Join(host.pairs, "\n"),
		})
	}
	return out
}

func appendUnique(items []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func exportAssetsTXT(items []fingerprint.ServiceFingerprint, kind string) string {
	var lines []string
	switch kind {
	case "ip":
		seen := map[string]struct{}{}
		for _, item := range items {
			if _, ok := seen[item.IP]; ok {
				continue
			}
			seen[item.IP] = struct{}{}
			lines = append(lines, item.IP)
		}
	case "url":
		for _, item := range items {
			if item.URL != "" {
				lines = append(lines, item.URL)
			}
		}
	default:
		for _, item := range items {
			lines = append(lines, item.IP+":"+strconv.Itoa(item.Port))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func exportAssetsCSV(items []fingerprint.ServiceFingerprint) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"ip", "port", "service", "product", "version", "url"}); err != nil {
		return "", err
	}
	for _, item := range items {
		if err := w.Write([]string{
			item.IP,
			strconv.Itoa(item.Port),
			item.Service,
			item.Product,
			item.Version,
			item.URL,
		}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func exportFindingsCSV(items []report.Finding, fps []fingerprint.ServiceFingerprint) (string, error) {
	services := map[string]fingerprint.ServiceFingerprint{}
	for _, item := range fps {
		services[item.IP+":"+strconv.Itoa(item.Port)] = item
	}

	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"severity", "source", "id", "ip", "port", "service", "product", "target", "summary", "evidence"}); err != nil {
		return "", err
	}
	for _, item := range items {
		fp := services[item.IP+":"+strconv.Itoa(item.Port)]
		if err := w.Write([]string{
			item.Severity,
			item.Source,
			item.ID,
			item.IP,
			strconv.Itoa(item.Port),
			fp.Service,
			fp.Product,
			item.Target,
			item.Summary,
			item.Output,
		}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func parseSeverityFilters(values url.Values) []string {
	if len(values["severity"]) == 0 {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range values["severity"] {
		for _, item := range strings.Split(raw, ",") {
			value := strings.ToLower(strings.TrimSpace(item))
			if !containsValue(supportedSeverities, value) {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func parsePage(value string) int {
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// parseSize resolves the per-page size from a query value. Only the values in
// reportPageSizes are accepted; anything else (including empty) falls back to
// reportPageSize so the default behavior is unchanged.
func parseSize(value string) int {
	size, err := strconv.Atoi(value)
	if err != nil {
		return reportPageSize
	}
	for _, allowed := range reportPageSizes {
		if size == allowed {
			return size
		}
	}
	return reportPageSize
}

func paginateFingerprints(items []fingerprint.ServiceFingerprint, page int, base url.Values, key, sizeKey string, size int) reportPage {
	sliced, current, total := paginate(items, page, size)
	values := cloneValues(base)
	return reportPage{
		Items:      sliced,
		Page:       current,
		TotalPages: total,
		HasPrev:    current > 1,
		HasNext:    current < total,
		PrevURL:    pageURL(values, key, current-1),
		NextURL:    pageURL(values, key, current+1),
		Size:       size,
		SizeStr:    strconv.Itoa(size),
		SizeURLs:   pageSizeURLs(values, key, sizeKey),
	}
}

func paginateFindings(items []report.Finding, page int, base url.Values, key, sizeKey string, size int) reportPage {
	sliced, current, total := paginate(items, page, size)
	values := cloneValues(base)
	return reportPage{
		Items:      sliced,
		Page:       current,
		TotalPages: total,
		HasPrev:    current > 1,
		HasNext:    current < total,
		PrevURL:    pageURL(values, key, current-1),
		NextURL:    pageURL(values, key, current+1),
		Size:       size,
		SizeStr:    strconv.Itoa(size),
		SizeURLs:   pageSizeURLs(values, key, sizeKey),
	}
}

func paginateHostAssets(items []hostAssetView, page int, base url.Values, key, sizeKey string, size int) reportPage {
	sliced, current, total := paginate(items, page, size)
	values := cloneValues(base)
	return reportPage{
		Items:      sliced,
		Page:       current,
		TotalPages: total,
		HasPrev:    current > 1,
		HasNext:    current < total,
		PrevURL:    pageURL(values, key, current-1),
		NextURL:    pageURL(values, key, current+1),
		Size:       size,
		SizeStr:    strconv.Itoa(size),
		SizeURLs:   pageSizeURLs(values, key, sizeKey),
	}
}

func paginate[T any](items []T, page int, size int) ([]T, int, int) {
	if size <= 0 {
		size = 50
	}
	totalPages := 1
	if len(items) > 0 {
		totalPages = (len(items) + size - 1) / size
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * size
	if start > len(items) {
		start = len(items)
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], page, totalPages
}

func cloneValues(values url.Values) url.Values {
	out := url.Values{}
	for key, list := range values {
		copied := make([]string, len(list))
		copy(copied, list)
		out[key] = copied
	}
	return out
}

func pageURL(values url.Values, key string, page int) string {
	if page < 1 {
		page = 1
	}
	values.Set(key, strconv.Itoa(page))
	encoded := values.Encode()
	if encoded == "" {
		return "?"
	}
	return "?" + encoded
}

// pageSizeURLs builds the "?assets_size=20&..." style links for each option in
// reportPageSizes. It drops the page key so switching the size always lands on
// the first page, and preserves every other filter parameter.
func pageSizeURLs(values url.Values, pageKey, sizeKey string) map[int]string {
	urls := make(map[int]string, len(reportPageSizes))
	for _, size := range reportPageSizes {
		clone := cloneValues(values)
		if pageKey != "" {
			clone.Del(pageKey)
		}
		clone.Set(sizeKey, strconv.Itoa(size))
		encoded := clone.Encode()
		if encoded == "" {
			urls[size] = "?"
		} else {
			urls[size] = "?" + encoded
		}
	}
	return urls
}

func withQuery(values url.Values, key string, value string) string {
	out := cloneValues(values)
	out.Set(key, value)
	return out.Encode()
}
