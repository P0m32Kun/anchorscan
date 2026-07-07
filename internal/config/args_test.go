package config

import (
	"reflect"
	"testing"
)

func TestSplitArgsHonorsQuotes(t *testing.T) {
	got, err := SplitArgs(`-T2 --script "redis-info,mysql-info" --max-retries 3`)
	if err != nil {
		t.Fatalf("SplitArgs returned error: %v", err)
	}
	want := []string{"-T2", "--script", "redis-info,mysql-info", "--max-retries", "3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %#v want %#v", got, want)
	}
}

func TestSplitArgsRejectsUnclosedQuote(t *testing.T) {
	_, err := SplitArgs(`-T2 "broken`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSplitArgsPreservesEmptyQuotedToken(t *testing.T) {
	got, err := SplitArgs(`--header "" --flag`)
	if err != nil {
		t.Fatalf("SplitArgs returned error: %v", err)
	}
	want := []string{"--header", "", "--flag"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args mismatch: got %#v want %#v", got, want)
	}
}
