package tools

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

// fathomFakeRunner records the args fathom would be invoked with and returns a
// canned stdout (mirroring what `fathom scan --json` emits).
type fathomFakeRunner struct {
	args []string
	out  []byte
	err  error
}

func (r *fathomFakeRunner) Run(_ context.Context, _ string, args []string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	return r.out, r.err
}

// Fixture shape below is the exact byte layout of `fathom scan --json` output.
// It was confirmed live against ~/DEV/fathom target/release/fathom with a redis
// mock on 127.0.0.1:6379 (see docs/reports/fathom-m41-report.md "schema 确认来源");
// the test uses port 16379 only to keep the synthetic fixture decoupled from a
// real redis default port.
// Schema source: src/fingerprint.rs:25-37 (Fingerprint::json),
// src/checks.rs:46-54 (CheckResult::json), src/main.rs:162-182 (write_results,
// one JSON object per line).
const fathomRedisFixture = `{"host":"127.0.0.1","port":16379,"service":"redis","product":"Redis","version":"","checks":[{"id":"redis-unauth","verdict":"vulnerable","proof":"redis_version:7.0.0"},{"id":"redis-weak","verdict":"safe","proof":"no weak password matched"}]}
`

// Multi-service fixture covering: shared names (ssh), unknown service (TLS
// candidate port 443), and a fathom-only token (mssql) with a safe check.
const fathomMultiFixture = `{"host":"192.0.2.10","port":22,"service":"ssh","product":"OpenSSH","version":"9.0"}
{"host":"192.0.2.10","port":443,"service":"unknown","product":"","version":""}
{"host":"192.0.2.10","port":1433,"service":"mssql","product":"Microsoft SQL Server","version":"","checks":[{"id":"mssql-weak","verdict":"safe","proof":"no weak password matched"}]}
`

func TestRunFathomScanBuildsArgs(t *testing.T) {
	runner := &fathomFakeRunner{out: []byte(fathomRedisFixture)}
	if _, err := RunFathomScan(context.Background(), runner, "/usr/local/bin/fathom", "127.0.0.1", []int{16379}); err != nil {
		t.Fatal(err)
	}
	want := []string{"scan", "--json", "127.0.0.1", "-p", "16379"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}

func TestRunFathomScanParsesRedisVulnerable(t *testing.T) {
	runner := &fathomFakeRunner{out: []byte(fathomRedisFixture)}
	res, err := RunFathomScan(context.Background(), runner, "fathom", "127.0.0.1", []int{16379})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fingerprints) != 1 {
		t.Fatalf("fingerprints = %d, want 1", len(res.Fingerprints))
	}
	fp := res.Fingerprints[0]
	if fp.IP != "127.0.0.1" || fp.Port != 16379 || fp.Protocol != "tcp" || fp.Service != "redis" || fp.Product != "Redis" || fp.Normalized != "redis" {
		t.Fatalf("fingerprint = %#v", fp)
	}
	if fp.CPE != "" {
		t.Fatalf("CPE should be empty (degradation), got %q", fp.CPE)
	}
	if fp.IsWeb || fp.URL != "" {
		t.Fatalf("redis must not be flagged web: IsWeb=%v URL=%q", fp.IsWeb, fp.URL)
	}

	// Only the vulnerable check becomes a Finding.
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %d, want 1 (vulnerable only)", len(res.Findings))
	}
	f := res.Findings[0]
	if f.Source != "fathom" || f.ID != "redis-unauth" || f.Severity != "high" {
		t.Fatalf("finding = %#v", f)
	}
	if f.Output != "redis_version:7.0.0" {
		t.Fatalf("finding output = %q, want proof", f.Output)
	}
	if f.Target != "127.0.0.1:16379" || f.IP != "127.0.0.1" || f.Port != 16379 || f.Protocol != "tcp" {
		t.Fatalf("finding target fields = %#v", f)
	}

	// Both checks land in the detection audit trail regardless of verdict.
	if len(res.Checks) != 2 {
		t.Fatalf("detection checks = %d, want 2", len(res.Checks))
	}
	detailByVerdict := map[string]string{}
	for _, c := range res.Checks {
		if c.Engine != "fathom" || c.Status != "completed" || c.IP != "127.0.0.1" || c.Port != 16379 || c.Protocol != "tcp" {
			t.Fatalf("check field wrong: %#v", c)
		}
		detailByVerdict[c.ReasonCode] = c.Detail
	}
	if detailByVerdict["vulnerable"] != "redis_version:7.0.0" {
		t.Fatalf("vulnerable check detail wrong: %#v", detailByVerdict)
	}
	if detailByVerdict["safe"] != "no weak password matched" {
		t.Fatalf("safe check detail wrong: %#v", detailByVerdict)
	}
	if len(res.Output) == 0 {
		t.Fatal("raw output should be preserved for artifact persistence")
	}
}

func TestRunFathomScanParsesMultipleServices(t *testing.T) {
	runner := &fathomFakeRunner{out: []byte(fathomMultiFixture)}
	res, err := RunFathomScan(context.Background(), runner, "fathom", "192.0.2.10", []int{22, 443, 1433})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fingerprints) != 3 {
		t.Fatalf("fingerprints = %d, want 3", len(res.Fingerprints))
	}
	normalized := map[int]string{}
	for _, fp := range res.Fingerprints {
		normalized[fp.Port] = fp.Normalized
	}
	if normalized[22] != "ssh" {
		t.Errorf("port 22 normalized = %q, want ssh", normalized[22])
	}
	if normalized[443] != "unknown" {
		t.Errorf("port 443 normalized = %q, want unknown", normalized[443])
	}
	if normalized[1433] != "ms-sql" {
		t.Errorf("port 1433 (fathom mssql) normalized = %q, want ms-sql", normalized[1433])
	}
	// No vulnerable check in this fixture -> zero findings.
	if len(res.Findings) != 0 {
		t.Fatalf("findings = %d, want 0", len(res.Findings))
	}
	// One safe mssql-weak check on the audit trail.
	if len(res.Checks) != 1 || res.Checks[0].ReasonCode != "safe" {
		t.Fatalf("checks = %#v, want one safe mssql-weak", res.Checks)
	}
}

func TestRunFathomScanSkipsNonJSONLines(t *testing.T) {
	// fathom --json prints one JSON object per line; a stray banner/diagnostic
	// line merged by the runner must not break parsing.
	mixed := []byte("fathom 0.1.0\n" + fathomRedisFixture + "\n")
	runner := &fathomFakeRunner{out: mixed}
	res, err := RunFathomScan(context.Background(), runner, "fathom", "127.0.0.1", []int{16379})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fingerprints) != 1 {
		t.Fatalf("fingerprints = %d, want 1 (banner skipped)", len(res.Fingerprints))
	}
}

func TestRunFathomScanRejectsMalformedJSONLine(t *testing.T) {
	bad := []byte(`{"host":"127.0.0.1","port":1,"service":`) // truncated
	runner := &fathomFakeRunner{out: bad}
	_, err := RunFathomScan(context.Background(), runner, "fathom", "127.0.0.1", []int{1})
	if err == nil {
		t.Fatal("expected error for malformed JSON line")
	}
}

func TestRunFathomScanPropagatesRunnerError(t *testing.T) {
	runner := &fathomFakeRunner{out: []byte("connection refused"), err: fmt.Errorf("exit status 1")}
	res, err := RunFathomScan(context.Background(), runner, "fathom", "127.0.0.1", []int{1})
	if err == nil {
		t.Fatal("expected runner error to propagate")
	}
	if res.Output == nil {
		t.Fatal("raw output should still be preserved on runner error")
	}
}

func TestRunFathomScanWebFingerprintSetsURL(t *testing.T) {
	web := []byte(`{"host":"192.0.2.10","port":80,"service":"http","product":"nginx","version":"1.24.0"}`)
	runner := &fathomFakeRunner{out: web}
	res, err := RunFathomScan(context.Background(), runner, "fathom", "192.0.2.10", []int{80})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fingerprints) != 1 || !res.Fingerprints[0].IsWeb {
		t.Fatalf("expected web flag: %#v", res.Fingerprints)
	}
	if res.Fingerprints[0].URL != "http://192.0.2.10:80" {
		t.Fatalf("URL = %q, want http://192.0.2.10:80", res.Fingerprints[0].URL)
	}
}

// TestNeedsTLSWebEnhancement pins the spec decision-2 predicate: fathom cannot
// complete a TLS handshake, so an unknown service on a TLS web candidate port
// is flagged for httpx enhancement in M4.2.
func TestNeedsTLSWebEnhancement(t *testing.T) {
	cases := []struct {
		service string
		port    int
		want    bool
	}{
		{"unknown", 443, true},
		{"unknown", 8443, true},
		{"unknown", 9443, true},
		{"", 443, false}, // blank service is not fathom's literal "unknown"
		{"unknown", 80, false},     // not a TLS candidate port
		{"redis", 443, false},      // identified service: not unknown
		{"http", 8443, false},      // identified service: not unknown
		{"unknown", 8080, false},   // not a TLS candidate port
	}
	for _, c := range cases {
		if got := NeedsTLSWebEnhancement(c.service, c.port); got != c.want {
			t.Errorf("NeedsTLSWebEnhancement(%q,%d) = %v, want %v", c.service, c.port, got, c.want)
		}
	}
}

// TestFathomNmapNormalizationParity is the M4.1 parity gate: the same logical
// service, fed through nmap XML parsing and fathom JSONL parsing, must produce
// the identical normalized service name (the key nse.yaml / service-tags.yaml
// look up). Diverging services are exactly the three fathom-only tokens
// covered by the normalize.go alias table.
func TestFathomNmapNormalizationParity(t *testing.T) {
	cases := []struct {
		name          string
		fathomService string // fathom JSONL "service" value
		nmapService   string // nmap XML <service name="..."> value
		port          int
	}{
		{"ms-sql", "mssql", "ms-sql", 1433},
		{"postgresql", "postgres", "postgresql", 5432},
		{"amqp", "rabbitmq", "amqp", 5672},
		{"redis", "redis", "redis", 6379},
		{"mysql", "mysql", "mysql", 3306},
		{"ssh", "ssh", "ssh", 22},
		{"smb", "smb", "microsoft-ds", 445},
		{"rdp", "rdp", "ms-wbt-server", 3389},
		{"http", "http", "http", 80},
		{"mongodb", "mongodb", "mongodb", 27017},
		{"dameng", "dameng", "dameng", 5236},
	}
	ctx := context.Background()
	for _, c := range cases {
		// --- fathom side: JSONL -> RunFathomScan (Classify applied) ---
		fathomJSONL := []byte(fmt.Sprintf(
			`{"host":"192.0.2.10","port":%d,"service":"%s","product":"","version":""}`,
			c.port, c.fathomService))
		frunner := &fathomFakeRunner{out: fathomJSONL}
		fres, err := RunFathomScan(ctx, frunner, "fathom", "192.0.2.10", []int{c.port})
		if err != nil {
			t.Fatalf("%s: fathom parse error: %v", c.name, err)
		}
		if len(fres.Fingerprints) != 1 {
			t.Fatalf("%s: fathom fingerprints = %d, want 1", c.name, len(fres.Fingerprints))
		}
		fathomNorm := fres.Fingerprints[0].Normalized

		// --- nmap side: XML -> ParseNmapXML -> Classify ---
		nmapXML := []byte(fmt.Sprintf(
			`<nmaprun><host><address addr="192.0.2.10" addrtype="ipv4"/><ports><port protocol="tcp" portid="%d"><state state="open"/><service name="%s"/></port></ports></host></nmaprun>`,
			c.port, c.nmapService))
		nfps, _, err := fingerprint.ParseNmapXML(nmapXML)
		if err != nil {
			t.Fatalf("%s: nmap parse error: %v", c.name, err)
		}
		if len(nfps) != 1 {
			t.Fatalf("%s: nmap fingerprints = %d, want 1", c.name, len(nfps))
		}
		nmapNorm := fingerprint.Classify(nfps[0]).Normalized

		if fathomNorm != nmapNorm {
			t.Errorf("%s: parity FAIL — fathom normalized %q vs nmap %q", c.name, fathomNorm, nmapNorm)
		}
		if fathomNorm != c.name {
			t.Errorf("%s: normalized = %q, want %q", c.name, fathomNorm, c.name)
		}
	}
}
