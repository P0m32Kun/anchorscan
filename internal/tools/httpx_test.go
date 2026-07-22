package tools

import (
	"context"
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestEnrichWebBuildsHTTPXCommandAndParsesJSON(t *testing.T) {
	runner := &fakeRunner{
		output: []byte(`{"url":"https://192.168.1.10:8443","status_code":404,"title":"Admin","tech":["nginx","Vue.js"]}`),
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
	if got.URL != "https://192.168.1.10:8443" || got.StatusCode != 404 || got.Title != "Admin" {
		t.Fatalf("unexpected http result: %#v", got)
	}

	wantArgs := []string{"/opt/httpx", "-json", "-silent", "-status-code", "-title", "-tech-detect", "-follow-redirects", "-u", "https://192.168.1.10:8443"}
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
	if got.URL != "http://127.0.0.1:8080" || got.StatusCode != 200 || got.Title != "Apache Tomcat" {
		t.Fatalf("unexpected http result: %#v", got)
	}
}

func TestEnrichWebTreatsMissingJSONLineAsNoResult(t *testing.T) {
	// Windows 实测：httpx 探测失败时退出码为 0 但没有任何 JSON 行，
	// CombinedOutput 里只剩 banner（首个非空白字符是 '_'），旧行为把它
	// 交给 json.Unmarshal，报 "invalid character '_' looking for beginning of value"，
	// 在 v1.8.2 之前这会导致整个 target 失败。
	runner := &fakeRunner{
		output: []byte("    __    __  __       _  __\n   / /_  / /_/ /_____ | |/ /\n  / __ \\/ __/ __/ __ \\|   /\n"),
	}

	fp := fingerprint.ServiceFingerprint{
		IP:    "127.0.0.1",
		Port:  8080,
		IsWeb: true,
		URL:   "http://127.0.0.1:8080",
	}

	got, err := EnrichWeb(context.Background(), runner, "/opt/httpx", fp, nil)
	if err != nil {
		t.Fatalf("EnrichWeb returned error for banner-only output: %v", err)
	}
	if got.URL != "" || got.StatusCode != 0 {
		t.Fatalf("expected empty result for missing JSON line, got %#v", got)
	}
}

func TestEnrichWebWithOutputReturnsRawOutput(t *testing.T) {
	raw := []byte(`{"url":"https://127.0.0.1:8443","status-code":200,"title":"Admin","tech":["nginx"]}`)
	runner := &fakeRunner{output: raw}

	fp := fingerprint.ServiceFingerprint{
		IP:    "127.0.0.1",
		Port:  8443,
		IsWeb: true,
		URL:   "https://127.0.0.1:8443",
	}

	got, out, err := EnrichWebWithOutput(context.Background(), runner, "/opt/httpx", fp, nil)
	if err != nil {
		t.Fatalf("EnrichWebWithOutput returned error: %v", err)
	}
	if got.URL != "https://127.0.0.1:8443" || got.Title != "Admin" {
		t.Fatalf("unexpected http result: %#v", got)
	}
	if string(out) != string(raw) {
		t.Fatalf("raw output mismatch: got %q want %q", out, raw)
	}
}
