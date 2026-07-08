package tools

import "testing"

func TestParseNucleiJSONLParsesFindings(t *testing.T) {
	input := []byte("{\"template-id\":\"redis-detect\",\"info\":{\"name\":\"Redis Detect\",\"severity\":\"info\"},\"matched-at\":\"192.168.1.10:6379\"}\n")
	got, err := ParseNucleiJSONL(input)
	if err != nil {
		t.Fatalf("ParseNucleiJSONL returned error: %v", err)
	}
	if len(got) != 1 || got[0].TemplateID != "redis-detect" || got[0].Severity != "info" {
		t.Fatalf("unexpected results: %#v", got)
	}
}

func TestParseNucleiJSONLIgnoresBannerLines(t *testing.T) {
	input := []byte(`[INF] Current nuclei version: v3.11.0
[INF] Templates loaded for current scan: 19
{"template-id":"exposed-redis","info":{"name":"Redis Server - Unauthenticated Access","severity":"high"},"matched-at":"127.0.0.1:6379"}
`)
	got, err := ParseNucleiJSONL(input)
	if err != nil {
		t.Fatalf("ParseNucleiJSONL returned error: %v", err)
	}
	if len(got) != 1 || got[0].TemplateID != "exposed-redis" || got[0].Severity != "high" {
		t.Fatalf("unexpected results: %#v", got)
	}
}

func TestParseNucleiJSONLParsesEvidenceFields(t *testing.T) {
	input := []byte("{\"template-id\":\"tomcat-default-login\",\"matcher-name\":\"basic-auth\",\"extractor-results\":[\"admin:admin\"],\"curl-command\":\"curl -u admin:admin http://127.0.0.1:8080/manager/html\",\"info\":{\"name\":\"Tomcat Default Login\",\"severity\":\"high\"},\"matched-at\":\"http://127.0.0.1:8080/manager/html\"}\n")
	got, err := ParseNucleiJSONL(input)
	if err != nil {
		t.Fatalf("ParseNucleiJSONL returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %#v", got)
	}
	if got[0].MatcherName != "basic-auth" || got[0].CurlCommand == "" || len(got[0].ExtractedResults) != 1 || got[0].Raw == "" {
		t.Fatalf("unexpected evidence fields: %#v", got[0])
	}
}
