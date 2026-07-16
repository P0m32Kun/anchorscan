package report

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func TestWriteHTMLStableBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	input := Build(
		[]fingerprint.ServiceFingerprint{
			{IP: "192.168.1.10", Port: 8080, Service: "http", Product: "tomcat", IsWeb: true, URL: "http://192.168.1.10:8080"},
		},
		[]Finding{
			{IP: "192.168.1.10", Port: 8080, Source: "nuclei", ID: "tomcat-default-login", Severity: "high", Summary: "Tomcat Default Login", Target: "http://192.168.1.10:8080", Output: "{\"matched-at\":\"http://192.168.1.10:8080\"}"},
		},
	)
	if err := WriteHTML(path, input); err != nil {
		t.Fatalf("WriteHTML returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := sha256.Sum256(data)
	const want = "d96399f2bada192e26884948ecada622af32088422193859e460185226dd4859"
	if actual := hex.EncodeToString(got[:]); actual != want {
		t.Fatalf("unexpected HTML SHA-256: got %s, want %s", actual, want)
	}
}

func TestWriteHTMLIncludesFindingSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	input := Build(
		[]fingerprint.ServiceFingerprint{
			{IP: "192.168.1.10", Port: 8080, Service: "http", Product: "tomcat", IsWeb: true, URL: "http://192.168.1.10:8080"},
		},
		[]Finding{
			{IP: "192.168.1.10", Port: 8080, Source: "nuclei", ID: "tomcat-default-login", Severity: "high", Summary: "Tomcat Default Login", Target: "http://192.168.1.10:8080", Output: "{\"matched-at\":\"http://192.168.1.10:8080\"}"},
		},
	)

	if err := WriteHTML(path, input); err != nil {
		t.Fatalf("WriteHTML returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Tomcat Default Login") {
		t.Fatalf("expected finding summary in html: %s", string(data))
	}
	if !strings.Contains(string(data), `<details class="finding-details">`) {
		t.Fatalf("expected collapsible finding details in html: %s", string(data))
	}
	if !strings.Contains(string(data), "matched-at") || !strings.Contains(string(data), "http://192.168.1.10:8080") {
		t.Fatalf("expected finding evidence in html: %s", string(data))
	}
}

func TestWriteHTMLIncludesMatchedVulnerabilityDelivery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	input := Build(nil, nil)
	input.Vulnerabilities = []VulnerabilityDelivery{{
		Title:       "Redis 默认凭据（高危）",
		Description: "Redis 未限制默认访问。",
		Remediation: "设置强密码并限制访问来源。",
	}}

	if err := WriteHTML(path, input); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Redis 未限制默认访问。") || !strings.Contains(string(data), "设置强密码并限制访问来源。") {
		t.Fatalf("expected matched vulnerability delivery in html: %s", data)
	}
}
