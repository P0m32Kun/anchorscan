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

func TestParseKeepsCIDRTargetsIntact(t *testing.T) {
	got, err := Parse("192.168.1.0/30")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := []string{"192.168.1.0/30"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse mismatch: got %#v want %#v", got, want)
	}
}

func TestExcludeUsesExactMatchesAndPreservesOrder(t *testing.T) {
	targets := []string{"10.0.0.1", "10.0.0.0/24", "host.local"}

	got, err := Exclude(targets, "host.local,10.0.0.1")
	if err != nil {
		t.Fatalf("Exclude returned error: %v", err)
	}

	want := []string{"10.0.0.0/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Exclude mismatch: got %#v want %#v", got, want)
	}
}

func TestExcludeReturnsAllTargetsWhenSpecIsEmpty(t *testing.T) {
	targets := []string{"10.0.0.1", "10.0.0.0/24", "host.local"}

	got, err := Exclude(targets, "")
	if err != nil {
		t.Fatalf("Exclude returned error: %v", err)
	}
	if !reflect.DeepEqual(got, targets) {
		t.Fatalf("Exclude mismatch: got %#v want %#v", got, targets)
	}
}

func TestExcludeDoesNotExpandCIDR(t *testing.T) {
	targets := []string{"10.0.0.1", "10.0.0.0/24"}

	got, err := Exclude(targets, "10.0.0.2")
	if err != nil {
		t.Fatalf("Exclude returned error: %v", err)
	}
	if !reflect.DeepEqual(got, targets) {
		t.Fatalf("Exclude mismatch: got %#v want %#v", got, targets)
	}
}
