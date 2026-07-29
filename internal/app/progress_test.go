package app

import (
	"fmt"
	"testing"
	"time"
)

func TestStoreProgressPreservesRawLogsAndStoresActionableSummary(t *testing.T) {
	st := newScanStore(t)
	var logged string
	raw := "nuclei 192.0.2.10:443 failed: exit status 1: __     _\n   ____  __  _______/ /__  (_)\n  / __ \\/ / / / ___/ / _ \\/ /\n / / / / /_/ / /__/ /  __/ /\n/_/ /_/\\__,_/\\___/_/\\___/_/   v3.11.0\n\n\t\tprojectdiscovery.io\n\n\x1b[31m[WRN]\x1b[0m Found 1 templates with runtime error\n\x1b[34m[INF]\x1b[0m Current nuclei version: v3.11.0\n\x1b[34m[INF]\x1b[0m Current nuclei-templates version: v10.4.6\n\x1b[34m[INF]\x1b[0m Targets loaded for current scan: 1\n\x1b[1;31m[FTL]\x1b[0m Could not run nuclei: no templates provided for scan"
	progress := storeProgress{
		runID: "run-summary",
		log:   func(format string, args ...any) { logged = fmt.Sprintf(format, args...) },
		store: st,
		now:   func() time.Time { return time.Unix(0, 0).UTC() },
	}

	progress.Emit("error", "nuclei", "%s", raw)

	if logged != raw {
		t.Fatalf("log = %q, want raw %q", logged, raw)
	}
	events, err := st.ListScanEvents("run-summary", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v, want one event", events)
	}
	const want = "nuclei 192.0.2.10:443 failed: Could not run nuclei: no templates provided for scan"
	if events[0].Message != want {
		t.Fatalf("ScanEvent.Message = %q, want %q", events[0].Message, want)
	}
}

func TestSummarizeScanEventKeepsSingleLineProgress(t *testing.T) {
	got := summarizeScanEvent("\x1b[32mrustscan 192.0.2.10 open=[22,443]\x1b[0m")
	const want = "rustscan 192.0.2.10 open=[22,443]"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}
