package web

import (
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestNewRunMetaViewSummarizesByRune(t *testing.T) {
	value := strings.Repeat("界", runMetaSummaryLimit+1)
	got := newRunMetaView(store.ScanRun{Target: value})
	if got.FullTarget != value || got.Target != strings.Repeat("界", runMetaSummaryLimit)+"..." {
		t.Fatalf("view = %#v", got)
	}
}
