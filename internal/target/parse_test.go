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

func TestParseSupportsNewlineSeparatedTargets(t *testing.T) {
	got, err := Parse("192.168.1.10\n192.168.1.11\n192.168.1.10")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := []string{"192.168.1.10", "192.168.1.11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse mismatch: got %#v want %#v", got, want)
	}
}

func TestParseSupportsCommaAndNewlineSeparatedTargets(t *testing.T) {
	got, err := Parse("192.168.1.10,192.168.1.11\n192.168.1.10\n192.168.1.12")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := []string{"192.168.1.10", "192.168.1.11", "192.168.1.12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse mismatch: got %#v want %#v", got, want)
	}
}
