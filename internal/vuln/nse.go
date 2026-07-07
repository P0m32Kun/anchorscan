package vuln

import "github.com/P0m32Kun/anchorscan/internal/fingerprint"

func MatchNSE(fp fingerprint.ServiceFingerprint, rules map[string][]string) []string {
	return append([]string(nil), rules[fp.Normalized]...)
}
