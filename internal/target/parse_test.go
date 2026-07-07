package target

import (
	"reflect"
	"testing"
)

func TestParseDeduplicatesCommaSeparatedTargets(t *testing.T) {
	got, err := Parse("192.168.1.10,192.168.1.10,192.168.1.11")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := []string{"192.168.1.10", "192.168.1.11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse mismatch: got %#v want %#v", got, want)
	}
}
