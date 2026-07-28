package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	DiscoveryAuto     = "auto"
	DiscoveryAssumeUp = "assume-up"
)

func normalizeDiscoveryMode(mode string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "", DiscoveryAuto:
		return DiscoveryAuto, nil
	case DiscoveryAssumeUp:
		return DiscoveryAssumeUp, nil
	default:
		return "", fmt.Errorf("invalid discovery mode: %s (expected auto or assume-up)", mode)
	}
}

// DiscoveryModeFromConfigSnapshot preserves the safe default for runs created
// before discovery mode was stored in their configuration snapshot.
func DiscoveryModeFromConfigSnapshot(snapshot string) string {
	var value struct {
		DiscoveryMode string `json:"discovery_mode"`
	}
	if json.Unmarshal([]byte(snapshot), &value) != nil {
		return DiscoveryAuto
	}
	mode, err := normalizeDiscoveryMode(value.DiscoveryMode)
	if err != nil {
		return DiscoveryAuto
	}
	return mode
}
