package web

import (
	"net/url"
	"sort"
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
	PageURLs   []pageLink
	Size       int
	SizeStr    string
	SizeURLs   map[int]string
}

// pageLink is a single entry in the numeric pagination control. Label carries
// the display text (the page number, or "..." for a gap sentinel); URL is empty
// for gap sentinels so the template can render them as non-clickable text.
// IsCurrent is precomputed by the backend so the template avoids a cross-type
// equality check between the loop variable and the page-number field.
type pageLink struct {
	Page      int
	Label     string
	URL       string
	IsCurrent bool
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
		PageURLs:   buildPageURLs(values, key, current, total),
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
		PageURLs:   buildPageURLs(values, key, current, total),
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
		PageURLs:   buildPageURLs(values, key, current, total),
		Size:       size,
		SizeStr:    strconv.Itoa(size),
		SizeURLs:   pageSizeURLs(values, key, sizeKey),
	}
}

// buildPageURLs generates an ordered list of page links for the numeric
// pagination control. It always includes the first and last page plus a window
// of pages around the current page, inserting "..." gap sentinels (URL empty)
// wherever pages are skipped so users can jump to any nearby or boundary page.
func buildPageURLs(values url.Values, key string, current, total int) []pageLink {
	if total <= 0 {
		return nil
	}
	window := 2
	start := current - window
	if start < 1 {
		start = 1
	}
	end := current + window
	if end > total {
		end = total
	}

	want := make(map[int]struct{}, end-start+3)
	want[1] = struct{}{}
	want[total] = struct{}{}
	for p := start; p <= end; p++ {
		want[p] = struct{}{}
	}

	pages := make([]int, 0, len(want))
	for p := range want {
		pages = append(pages, p)
	}
	sort.Ints(pages)

	links := make([]pageLink, 0, len(pages)+2)
	prev := 0
	for _, p := range pages {
		if prev != 0 && p > prev+1 {
			links = append(links, pageLink{Page: 0, Label: "..."})
		}
		links = append(links, pageLink{
			Page:      p,
			Label:     strconv.Itoa(p),
			URL:       pageURL(cloneValues(values), key, p),
			IsCurrent: p == current,
		})
		prev = p
	}
	return links
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
