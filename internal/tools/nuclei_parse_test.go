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

func TestParseNucleiJSONLParsesEndpointFields(t *testing.T) {
	input := []byte(`{"template-id":"redis-default-logins","host":"172.22.0.1","ip":"172.22.0.1","port":"6379","url":"172.22.0.1:6379","info":{"name":"Redis Default Login","severity":"high"},"matched-at":"172.22.0.1:6379"}` + "\n")
	got, err := ParseNucleiJSONL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %#v", got)
	}
	if got[0].Host != "172.22.0.1" || got[0].IP != "172.22.0.1" || got[0].Port != "6379" || got[0].URL != "172.22.0.1:6379" {
		t.Fatalf("endpoint fields = %#v", got[0])
	}
}

func TestNucleiFindingEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		finding      NucleiFinding
		fallbackHost string
		fallbackPort int
		wantHost     string
		wantPort     int
	}{
		{
			name:         "structured redis endpoint",
			finding:      NucleiFinding{IP: "172.22.0.1", Port: "6379", MatchedAt: "172.22.0.1:6379"},
			fallbackHost: "172.22.0.1", fallbackPort: 8080,
			wantHost: "172.22.0.1", wantPort: 6379,
		},
		{
			name:         "https standard port",
			finding:      NucleiFinding{MatchedAt: "https://example.test/manager"},
			fallbackHost: "fallback.test", fallbackPort: 8080,
			wantHost: "example.test", wantPort: 443,
		},
		{
			name:         "bracketed ipv6",
			finding:      NucleiFinding{Host: "[2001:db8::10]:8443"},
			fallbackHost: "2001:db8::20", fallbackPort: 8080,
			wantHost: "2001:db8::10", wantPort: 8443,
		},
		{
			name:         "invalid port falls back",
			finding:      NucleiFinding{IP: "192.0.2.10", Port: "70000"},
			fallbackHost: "192.0.2.20", fallbackPort: 8080,
			wantHost: "192.0.2.10", wantPort: 8080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := tt.finding.Endpoint(tt.fallbackHost, tt.fallbackPort)
			if host != tt.wantHost || port != tt.wantPort {
				t.Fatalf("Endpoint() = %s:%d, want %s:%d", host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}
