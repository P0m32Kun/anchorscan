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
	CPE        string
	Normalized string
	IsWeb      bool
	URL        string
}

// ImportedScript 承载从 Nmap XML 解析出的 NSE 脚本输出及其作用域。
// Scope 取值：port（端口级）、host（主机级 hostscript）、pre（prescript）、post（postscript）。
type ImportedScript struct {
	Scope    string
	IP       string
	Port     int
	Protocol string
	ID       string
	Output   string
}
