package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

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
	if !strings.Contains(string(data), "matched-at") || !strings.Contains(string(data), "http://192.168.1.10:8080") {
		t.Fatalf("expected finding evidence in html: %s", string(data))
	}
}
