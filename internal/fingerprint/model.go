package fingerprint

type ServiceFingerprint struct {
	IP         string
	Port       int
	Protocol   string
	Service    string
	Product    string
	Version    string
	ExtraInfo  string
	Tunnel     string
	Normalized string
	IsWeb      bool
	URL        string
}
