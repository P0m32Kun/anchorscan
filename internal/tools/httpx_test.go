package tools

import (
	"context"
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestEnrichWebBuildsHTTPXCommandAndParsesJSON(t *testing.T) {
	runner := &fakeRunner{
		output: []byte(`{"url":"https://192.168.1.10:8443","status-code":200,"title":"Admin","tech":["nginx","Vue.js"]}`),
	}

	fp := fingerprint.ServiceFingerprint{
		IP:    "192.168.1.10",
		Port:  8443,
		IsWeb: true,
		URL:   "https://192.168.1.10:8443",
	}

	got, err := EnrichWeb(context.Background(), runner, "/opt/httpx", fp, nil)
	if err != nil {
		t.Fatalf("EnrichWeb returned error: %v", err)
	}
	if got.URL != "https://192.168.1.10:8443" || got.Title != "Admin" {
		t.Fatalf("unexpected http result: %#v", got)
	}

	wantArgs := []string{"/opt/httpx", "-json", "-status-code", "-title", "-tech-detect", "-follow-redirects", "-u", "https://192.168.1.10:8443"}
	if !reflect.DeepEqual(runner.args, wantArgs) {
		t.Fatalf("args mismatch: got %#v want %#v", runner.args, wantArgs)
	}
}

func TestEnrichWebIgnoresHTTPXBannerLines(t *testing.T) {
	runner := &fakeRunner{
		output: []byte(`[INF] Current httpx version v1.9.0
[WRN] UI Dashboard is disabled
{"url":"http://127.0.0.1:8080","status-code":200,"title":"Apache Tomcat","tech":["tomcat"]}`),
	}

	fp := fingerprint.ServiceFingerprint{
		IP:    "127.0.0.1",
		Port:  8080,
		IsWeb: true,
		URL:   "http://127.0.0.1:8080",
	}

	got, err := EnrichWeb(context.Background(), runner, "/opt/httpx", fp, nil)
	if err != nil {
		t.Fatalf("EnrichWeb returned error: %v", err)
	}
	if got.URL != "http://127.0.0.1:8080" || got.Title != "Apache Tomcat" {
		t.Fatalf("unexpected http result: %#v", got)
	}
}
