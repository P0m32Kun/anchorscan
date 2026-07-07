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
