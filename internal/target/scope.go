package target

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// MaxAddresses is the largest scope that the local scanner accepts without an
// explicit product-level expansion workflow.
const MaxAddresses uint64 = 4096

// Scope is an authorized IP address space. It keeps prefixes compact so large
// inputs are never expanded solely to count or exclude addresses.
type Scope struct {
	includes  []netip.Prefix
	excludes  []netip.Prefix
	estimated uint64
}

// Snapshot is the stable, serializable representation of an authorized scope.
type Snapshot struct {
	Includes           []string `json:"includes"`
	Excludes           []string `json:"excludes"`
	EstimatedAddresses uint64   `json:"estimated_addresses"`
}

func ParseScope(includeSpec, excludeSpec string) (Scope, error) {
	includes, err := parsePrefixes(includeSpec, "target")
	if err != nil {
		return Scope{}, err
	}
	includes = removeCoveredPrefixes(includes)
	excludes, err := parsePrefixesOptional(excludeSpec, "exclude target")
	if err != nil {
		return Scope{}, err
	}
	excludes = removeCoveredPrefixes(excludes)
	includes = removeExcludedPrefixes(includes, excludes)
	if len(includes) == 0 {
		return Scope{}, fmt.Errorf("target scope is empty after exclusions")
	}
	estimated := estimateAddresses(includes)
	if estimated > MaxAddresses {
		return Scope{}, fmt.Errorf("target scope contains more than %d addresses", MaxAddresses)
	}
	return Scope{includes: includes, excludes: excludes, estimated: estimated}, nil
}

func parsePrefixesOptional(spec, field string) ([]netip.Prefix, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	return parsePrefixes(spec, field)
}

func parsePrefixes(spec, field string) ([]netip.Prefix, error) {
	items := splitSpec(spec)
	if len(items) == 0 {
		return nil, fmt.Errorf("%s is empty", field)
	}

	prefixes := make([]netip.Prefix, 0, len(items))
	seen := make(map[netip.Prefix]struct{}, len(items))
	for _, item := range items {
		prefix, err := parsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("invalid %s %q: %w", field, item, err)
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		if compared := prefixes[i].Addr().Compare(prefixes[j].Addr()); compared != 0 {
			return compared < 0
		}
		return prefixes[i].Bits() < prefixes[j].Bits()
	})
	return removeCoveredPrefixes(prefixes), nil
}

func splitSpec(spec string) []string {
	spec = strings.ReplaceAll(spec, "\r\n", "\n")
	spec = strings.ReplaceAll(spec, "\r", "\n")
	var items []string
	for _, item := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == '\n' }) {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func parsePrefix(value string) (netip.Prefix, error) {
	if strings.HasPrefix(value, "-") {
		return netip.Prefix{}, fmt.Errorf("flags are not valid targets")
	}
	if address, err := netip.ParseAddr(value); err == nil {
		return netip.PrefixFrom(address, address.BitLen()), nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return netip.Prefix{}, err
	}
	return prefix.Masked(), nil
}

func removeCoveredPrefixes(prefixes []netip.Prefix) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(prefixes))
	for _, prefix := range prefixes {
		covered := false
		for _, existing := range result {
			if existing.Contains(prefix.Addr()) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, prefix)
		}
	}
	return result
}

func (s Scope) DiscoveryScopes() []Scope {
	var ipv4, ipv6 []netip.Prefix
	for _, prefix := range s.includes {
		if prefix.Addr().Is4() {
			ipv4 = append(ipv4, prefix)
		} else {
			ipv6 = append(ipv6, prefix)
		}
	}
	return compactScopes(ipv4, ipv6, s.excludes)
}

func compactScopes(ipv4, ipv6, excludes []netip.Prefix) []Scope {
	var scopes []Scope
	for _, includes := range [][]netip.Prefix{ipv4, ipv6} {
		if len(includes) == 0 {
			continue
		}
		familyExcludes := make([]netip.Prefix, 0)
		for _, exclude := range excludes {
			if exclude.Addr().Is4() == includes[0].Addr().Is4() {
				familyExcludes = append(familyExcludes, exclude)
			}
		}
		scopes = append(scopes, Scope{includes: includes, excludes: familyExcludes, estimated: estimateAddresses(includes)})
	}
	return scopes
}

func mixedAddressFamilies(prefixes []netip.Prefix) bool {
	if len(prefixes) < 2 {
		return false
	}
	family := prefixes[0].Addr().Is4()
	for _, prefix := range prefixes[1:] {
		if prefix.Addr().Is4() != family {
			return true
		}
	}
	return false
}

func removeExcludedPrefixes(includes, excludes []netip.Prefix) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(includes))
	for _, included := range includes {
		excluded := false
		for _, exclusion := range excludes {
			if exclusion.Contains(included.Addr()) && exclusion.Bits() <= included.Bits() {
				excluded = true
				break
			}
		}
		if !excluded {
			result = append(result, included)
		}
	}
	return result
}

func estimateAddresses(prefixes []netip.Prefix) uint64 {
	var total uint64
	for _, prefix := range prefixes {
		bits := prefix.Addr().BitLen() - prefix.Bits()
		if bits >= 63 {
			return MaxAddresses + 1
		}
		count := uint64(1) << bits
		if count > MaxAddresses-total {
			return MaxAddresses + 1
		}
		total += count
	}
	return total
}

func (s Scope) IsZero() bool { return len(s.includes) == 0 }

func (s Scope) Targets() []string { return prefixStrings(s.includes) }

func (s Scope) Excludes() []string { return prefixStrings(s.excludes) }

func (s Scope) EstimatedAddresses() uint64 { return s.estimated }

func (s Scope) RequiresNmapDiscovery() bool {
	for _, prefix := range s.includes {
		if prefix.Bits() != prefix.Addr().BitLen() {
			return true
		}
	}
	return len(s.excludes) > 0
}

func (s Scope) IsIPv6() bool {
	return len(s.includes) > 0 && !s.includes[0].Addr().Is4()
}

func (s Scope) NmapTargets() []string { return nmapPrefixStrings(s.includes) }

func (s Scope) NmapExcludes() []string { return nmapPrefixStrings(s.excludes) }

// Addresses expands this already-bounded scope into individual authorized
// addresses. It is for tools that cannot represent exclusions in their own
// target syntax; ParseScope caps the expansion at MaxAddresses.
func (s Scope) Addresses() []string {
	addresses := make([]string, 0, int(s.estimated))
	seen := make(map[netip.Addr]struct{}, int(s.estimated))
	for _, prefix := range s.includes {
		for address := prefix.Masked().Addr(); prefix.Contains(address); address = address.Next() {
			if !s.AllowsAddr(address) {
				continue
			}
			if _, ok := seen[address]; ok {
				continue
			}
			seen[address] = struct{}{}
			addresses = append(addresses, address.String())
		}
	}
	return addresses
}

func (s Scope) SingleAddress() (string, bool) {
	if len(s.includes) != 1 {
		return "", false
	}
	prefix := s.includes[0]
	if prefix.Bits() != prefix.Addr().BitLen() || !s.AllowsAddr(prefix.Addr()) {
		return "", false
	}
	return prefix.Addr().String(), true
}

func (s Scope) Snapshot() Snapshot {
	return Snapshot{
		Includes:           s.Targets(),
		Excludes:           s.Excludes(),
		EstimatedAddresses: s.estimated,
	}
}

func (s Scope) Allows(value string) bool {
	address, err := netip.ParseAddr(value)
	return err == nil && s.AllowsAddr(address)
}

func (s Scope) AllowsAddr(address netip.Addr) bool {
	for _, excluded := range s.excludes {
		if excluded.Contains(address) {
			return false
		}
	}
	for _, included := range s.includes {
		if included.Contains(address) {
			return true
		}
	}
	return false
}

func (s Scope) Filter(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		address, err := netip.ParseAddr(value)
		if err != nil || !s.AllowsAddr(address) {
			continue
		}
		canonical := address.String()
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		filtered = append(filtered, canonical)
	}
	return filtered
}

func nmapPrefixStrings(prefixes []netip.Prefix) []string {
	if len(prefixes) == 0 {
		return nil
	}
	values := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		if prefix.Bits() == prefix.Addr().BitLen() {
			values = append(values, prefix.Addr().String())
			continue
		}
		values = append(values, prefix.String())
	}
	return values
}

func prefixStrings(prefixes []netip.Prefix) []string {
	if len(prefixes) == 0 {
		return nil
	}
	values := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		values = append(values, prefix.String())
	}
	return values
}
