package app

import "encoding/json"

// DiscoveryModeFromConfigSnapshot preserves the safe historical default for
// runs created before discovery mode was recorded.
func DiscoveryModeFromConfigSnapshot(snapshot string) string {
	var value struct {
		DiscoveryMode string `json:"discovery_mode"`
	}
	if json.Unmarshal([]byte(snapshot), &value) == nil && value.DiscoveryMode == "assume-up" {
		return value.DiscoveryMode
	}
	return "auto"
}
