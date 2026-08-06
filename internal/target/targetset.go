package target

import (
	"net/netip"
	"strings"
)

// TargetSet is a Project's curated list of scan targets. It is an *intake* and
// prefill aid only: it never stores vulnerabilities, liveness, or run results,
// and it does not evolve into a CMDB.
//
// Import semantics (ParseTargetSet):
//   - Input is split with the same rules as Scan targets (commas / newlines).
//   - Every value must be a canonical IPv4, IPv6, or CIDR; URLs, flag-like
//     command fragments (leading dash), hostnames, and empty values are
//     rejected with a specific reason.
//   - Values are canonicalized (netip) and deduplicated stably: a canonical
//     value seen twice is reported as a duplicate, never double-listed.
//
// The accepted list is stored verbatim (newline-joined) and feeds the Scan
// Create form prefill.

// RejectedTarget records one rejected input value and why.
type RejectedTarget struct {
	Original string `json:"original"`
	Reason   string `json:"reason"`
}

// TargetSetImportResult is the full outcome of an import attempt.
type TargetSetImportResult struct {
	Accepted   []string         `json:"accepted"`
	Duplicates []string         `json:"duplicates"`
	Rejected   []RejectedTarget `json:"rejected"`
}

// ParseTargetSet validates, canonicalizes, and dedups an imported target list.
func ParseTargetSet(input string) TargetSetImportResult {
	var result TargetSetImportResult
	acceptedSet := map[string]struct{}{}
	reportedDuplicate := map[string]struct{}{}

	items, err := Parse(input)
	if err != nil {
		return result
	}
	for _, raw := range items {
		canonical, ok := canonicalTarget(raw)
		if !ok {
			result.Rejected = append(result.Rejected, RejectedTarget{Original: raw, Reason: rejectReason(raw)})
			continue
		}
		if _, seen := acceptedSet[canonical]; seen {
			if _, reported := reportedDuplicate[canonical]; !reported {
				reportedDuplicate[canonical] = struct{}{}
				result.Duplicates = append(result.Duplicates, canonical)
			}
			continue
		}
		acceptedSet[canonical] = struct{}{}
		result.Accepted = append(result.Accepted, canonical)
	}
	return result
}

// canonicalTarget returns the netip-canonical form of a target value, or false
// when the value is not a valid IP or CIDR. It mirrors the scope parser's
// validation (no flags, no hostnames, no URLs).
func canonicalTarget(value string) (string, bool) {
	if strings.HasPrefix(value, "-") {
		return "", false
	}
	if strings.Contains(value, "://") {
		return "", false
	}
	if address, err := netip.ParseAddr(value); err == nil {
		return address.String(), true
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), true
	}
	return "", false
}

func rejectReason(value string) string {
	switch {
	case strings.TrimSpace(value) == "":
		return "空值"
	case strings.HasPrefix(value, "-"):
		return "以 - 开头，可能是扫描器参数（命令片段）而非目标"
	case strings.Contains(value, "://"):
		return "URL 不是合法目标，请使用主机 IP 或网段"
	default:
		return "不是合法的 IPv4、IPv6 或 CIDR"
	}
}
