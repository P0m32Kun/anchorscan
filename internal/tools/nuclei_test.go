package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type nucleiRunner struct {
	args []string
}

func (r *nucleiRunner) Run(_ context.Context, _ string, args []string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	return []byte(`{"template-id":"x","info":{"name":"x","severity":"info"},"matched-at":"http://example.test"}` + "\n"), nil
}

func TestRunNucleiTemplateUsesTemplateFlag(t *testing.T) {
	runner := &nucleiRunner{}

	if _, err := RunNucleiTemplate(context.Background(), runner, "nuclei", "http://example.test", "cves/2021/test.yaml", []string{"-rate-limit", "5"}); err != nil {
		t.Fatal(err)
	}

	want := []string{"-target", "http://example.test", "-t", "cves/2021/test.yaml", "-jsonl", "-rate-limit", "5"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}

func TestParseNucleiJSONLHandlesOversizedLine(t *testing.T) {
	// Regression for the mysql-show-variables case: a single JSONL line can
	// exceed the default 64KB bufio.Scanner limit (observed 87KB for MariaDB
	// 11.8 SHOW VARIABLES). ParseNucleiJSONL must not fail with
	// "token too long".
	big := make([]string, 702)
	for i := range big {
		big[i] = "variable_" + strings.Repeat("x", 100)
	}
	extracted, err := json.Marshal(big)
	if err != nil {
		t.Fatal(err)
	}
	line := `{"template-id":"mysql-show-variables","info":{"name":"MySQL - Show Variables","severity":"info"},"matched-at":"172.22.0.5:3306","extracted-results":` + string(extracted) + "}\n"
	if len(line) < 70*1024 {
		t.Fatalf("test line too small: %d bytes", len(line))
	}

	findings, err := ParseNucleiJSONL([]byte(line))
	if err != nil {
		t.Fatalf("ParseNucleiJSONL failed on oversized line: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].TemplateID != "mysql-show-variables" {
		t.Fatalf("template-id = %q, want mysql-show-variables", findings[0].TemplateID)
	}
	if len(findings[0].ExtractedResults) != 702 {
		t.Fatalf("extracted-results = %d, want 702", len(findings[0].ExtractedResults))
	}
}
