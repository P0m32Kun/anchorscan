package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestWriteJSONOutputsHostAndPortData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	input := Build(
		[]fingerprint.ServiceFingerprint{
			{IP: "192.168.1.10", Port: 8080, Service: "http", Product: "tomcat", IsWeb: true, URL: "http://192.168.1.10:8080"},
		},
		[]Finding{
			{IP: "192.168.1.10", Port: 8080, Source: "nuclei", ID: "tomcat-default-login", Severity: "high", Summary: "Tomcat Default Login"},
		},
	)

	if err := WriteJSON(path, input); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	hosts := decoded["hosts"].([]any)
	firstHost := hosts[0].(map[string]any)
	ports := firstHost["ports"].([]any)
	firstPort := ports[0].(map[string]any)
	findings := firstPort["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one finding in report, got %#v", firstPort)
	}
	if _, ok := decoded["detection_checks"]; ok {
		t.Fatalf("legacy report unexpectedly contains detection checks: %#v", decoded)
	}
}

func TestReadJSONIgnoresUnknownFieldsAndReadsLegacyReport(t *testing.T) {
	legacy := `{"scan_meta":{"tool":"anchorscan"},"alive_ips":["192.0.2.10"],"hosts":[{"ip":"192.0.2.10","ports":[{"port":80,"protocol":"tcp","service":"http","findings":[]}]}],"unknown_future_field":"ignored"}`
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := ReadJSON(path)
	if err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if r.ScanMeta.Tool != "anchorscan" {
		t.Fatalf("tool = %q", r.ScanMeta.Tool)
	}
	if len(r.AliveIPs) != 1 || len(r.Hosts) != 1 {
		t.Fatalf("legacy fields not read: %#v", r)
	}
	if r.Provenance != nil {
		t.Fatalf("legacy report should not have provenance")
	}
}
