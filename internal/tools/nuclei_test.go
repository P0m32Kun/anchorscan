package tools

import (
	"context"
	"reflect"
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
