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
	const want = "561541194834b0675cc63519cc08bc03f3c5291e5844da7efbc1c27f741fd8b7"
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
	if !strings.Contains(string(data), `<tr class="finding-detail-row">`) || !strings.Contains(string(data), `<td colspan="6">`) {
		t.Fatalf("expected full-width finding details in html: %s", string(data))
	}
	if !strings.Contains(string(data), "findingText.includes(keywordVal)") {
		t.Fatalf("expected keyword filtering to include finding details: %s", string(data))
	}
	if !strings.Contains(string(data), "matched-at") || !strings.Contains(string(data), "http://192.168.1.10:8080") {
		t.Fatalf("expected finding evidence in html: %s", string(data))
	}
}

func TestWriteHTMLIncludesMatchedVulnerabilityDelivery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	input := Build([]fingerprint.ServiceFingerprint{{IP: "192.0.2.10", Port: 6379}}, []Finding{{
		IP: "192.0.2.10", Port: 6379, Description: "Redis 未限制默认访问。", Remediation: "设置强密码并限制访问来源。",
	}})

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

func TestWriteHTMLUsesSelfContainedLightTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	if err := WriteHTML(path, Build(nil, nil)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if !strings.Contains(html, "--canvas: #f5f5f7") {
		t.Fatalf("expected approved light canvas in exported report")
	}
	if strings.Contains(html, "fonts.googleapis.com") || strings.Contains(html, "fonts.gstatic.com") || strings.Contains(html, "href=\"/static/") {
		t.Fatalf("exported report must not depend on external resources")
	}
}
