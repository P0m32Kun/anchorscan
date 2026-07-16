package app

import (
	"fmt"
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

// Emit formats the message, forwards it to the logger, and — when attached to a
// real run — appends a ScanEvent so the web UI's progress feed stays live.
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
		Message: message,
	})
}
