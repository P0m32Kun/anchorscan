package target

import (
	"reflect"
	"testing"
)

// Ticket 07: Target Set import must validate, canonicalize, and dedup targets
// before anything is persisted. URL/flag/empty/invalid inputs are rejected with
// a reason; canonical duplicates are reported separately from accepted items.

func TestParseTargetSetAcceptsNormalizesAndDedups(t *testing.T) {
	// 192.0.2.10 and 2001:db8::1 appear twice: once verbatim, once as a case
	// variant that netip canonicalizes to the same value (IPv6 hex is
	// case-insensitive). Leading-zero IPv4 is invalid and must be rejected.
	input := "192.0.2.10\n192.000.002.10\n198.51.100.0/24\n2001:db8::1\n2001:DB8::1\n192.0.2.10\n  "
	result := ParseTargetSet(input)

	wantAccepted := []string{"192.0.2.10", "198.51.100.0/24", "2001:db8::1"}
	if !reflect.DeepEqual(result.Accepted, wantAccepted) {
		t.Fatalf("accepted mismatch:\n got %v\nwant %v", result.Accepted, wantAccepted)
	}
	if !reflect.DeepEqual(result.Duplicates, []string{"2001:db8::1"}) {
		t.Fatalf("duplicates mismatch: %v", result.Duplicates)
	}
	// Leading-zero octet is not a valid netip address.
	if len(result.Rejected) != 1 || result.Rejected[0].Original != "192.000.002.10" {
		t.Fatalf("unexpected rejects: %+v", result.Rejected)
	}
}

func TestParseTargetSetRejectsURLsFlagsAndInvalid(t *testing.T) {
	input := "https://example.com\nhttp://192.0.2.10\n-sV\n--script=http\n192.0.2.999\nexample.com\n10.0.0.5\n"
	result := ParseTargetSet(input)

	if !reflect.DeepEqual(result.Accepted, []string{"10.0.0.5"}) {
		t.Fatalf("accepted mismatch: %v", result.Accepted)
	}
	reasons := map[string]string{}
	for _, rejected := range result.Rejected {
		reasons[rejected.Original] = rejected.Reason
	}
	for _, want := range []string{"https://example.com", "http://192.0.2.10", "-sV", "--script=http", "192.0.2.999", "example.com"} {
		if reasons[want] == "" {
			t.Fatalf("expected %q to be rejected with a reason, got %+v", want, result.Rejected)
		}
	}
	if reasons["https://example.com"] == reasons["-sV"] {
		t.Fatalf("rejection reasons must be specific, not a generic catch-all")
	}
}

func TestParseTargetSetEmptyInput(t *testing.T) {
	result := ParseTargetSet("  \n , \n")
	if len(result.Accepted) != 0 || len(result.Duplicates) != 0 || len(result.Rejected) != 0 {
		t.Fatalf("empty input must produce an empty result, got %+v", result)
	}
}

func TestParseTargetSetKeepsHostOrderStable(t *testing.T) {
	input := "192.0.2.5\n192.0.2.1\n10.0.0.1\n2001:db8::1\n2001:DB8::1\n"
	result := ParseTargetSet(input)
	want := []string{"192.0.2.5", "192.0.2.1", "10.0.0.1", "2001:db8::1"}
	if !reflect.DeepEqual(result.Accepted, want) {
		t.Fatalf("order not stable: got %v want %v", result.Accepted, want)
	}
	if !reflect.DeepEqual(result.Duplicates, []string{"2001:db8::1"}) {
		t.Fatalf("duplicates mismatch: %v", result.Duplicates)
	}
}
