package app

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

// Progress is the narrow dependency a scan has on reporting progress: it both
// logs (via the configured logger) and records a ScanEvent for the live
// progress feed (/runs/:id/status, /runs/:id/events). Defining it at the
// consumer side lets the scan pipeline depend on one method instead of the
// whole *store.Store.
type Progress interface {
	Emit(level, stage, format string, args ...any)
}

// storeProgress adapts *store.Store (plus the run's logger and clock) to Progress.
type storeProgress struct {
	runID string
	log   func(format string, args ...any)
	store *store.Store
	now   func() time.Time
}

// ansiEscape removes terminal control sequences before ScanEvent reaches the web UI.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

// summarizeScanEvent keeps live progress readable without changing the raw log
// or tool artifact that remain the diagnostic record.
func summarizeScanEvent(message string) string {
	lines := strings.Split(ansiEscape.ReplaceAllString(message, ""), "\n")
	context := strings.TrimSpace(lines[0])
	var detail string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" || isScanEventNoise(line) {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "[FTL]"))
		if line != "" {
			detail = line
		}
	}
	if detail == "" {
		return context
	}
	if context == "" {
		return detail
	}
	if failedAt := strings.Index(context, " failed:"); failedAt >= 0 {
		context = context[:failedAt+len(" failed")]
	}
	return context + ": " + detail
}

func isScanEventNoise(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "current nuclei version") ||
		strings.Contains(lower, "current nuclei-templates version") ||
		strings.Contains(lower, "new templates added") ||
		strings.Contains(lower, "templates loaded for current scan") {
		return true
	}
	return strings.Trim(line, " _/\\|()[]{}<>-=+*.:") == ""
}

// Emit formats the message, forwards the original to the logger, and — when
// attached to a real run — appends a readable ScanEvent for the live feed.
func (p storeProgress) Emit(level, stage, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if p.log != nil {
		p.log("%s", message)
	}
	if p.runID == "" || p.store == nil {
		return
	}
	_ = p.store.AppendScanEvent(store.ScanEvent{
		RunID:   p.runID,
		Time:    p.now(),
		Level:   level,
		Stage:   stage,
		Message: scanEventMessage(level, message),
	})
}

// scanEventMessage keeps the live feed readable for regular events, but stores
// tool-terminal events (the echoed command and the raw tool output) verbatim —
// ANSI colors included — so the single-tool page can render them exactly like
// an external terminal.
func scanEventMessage(level, message string) string {
	if level == "command" || level == "raw" {
		return message
	}
	return summarizeScanEvent(message)
}
