package web

import (
	"net/url"
	"strconv"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

const reportPageSize = 50

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
