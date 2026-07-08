package web

import (
	"encoding/csv"
	"net/url"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

type reportFilters struct {
	IP       string
	Port     string
	Service  string
	Keyword  string
	Severity string
	Source   string
}

type reportPage struct {
	Items      any
	Page       int
	TotalPages int
	HasPrev    bool
	HasNext    bool
	PrevURL    string
	NextURL    string
}

type hostAssetView struct {
	IP        string
	Ports     string
	Services  string
	Products  string
	URLs      string
	CopyPorts string
	CopyPairs string
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
		if filters.Severity != "" && item.Severity != filters.Severity {
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

func parsePage(value string) int {
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func paginateFingerprints(items []fingerprint.ServiceFingerprint, page int, base url.Values, key string, size int) reportPage {
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
	}
}

func paginateFindings(items []report.Finding, page int, base url.Values, key string, size int) reportPage {
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
	}
}

func paginateHostAssets(items []hostAssetView, page int, base url.Values, key string, size int) reportPage {
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

func withQuery(values url.Values, key string, value string) string {
	out := cloneValues(values)
	out.Set(key, value)
	return out.Encode()
}
