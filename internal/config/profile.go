package config

import "fmt"

type Overrides struct {
	ProfileName  string
	HostWorkers  int
	RustscanArgs string
	NmapArgs     string
	HttpxArgs    string
	NucleiArgs   string
}

type EffectiveScan struct {
	ProfileName string
	HostWorkers int
	ToolArgs
}

func ResolveScan(cfg Config, overrides Overrides) (EffectiveScan, error) {
	if len(cfg.Profiles) == 0 {
		cfg.Profiles = builtInProfiles()
	}

	name := cfg.Scan.Profile
	if overrides.ProfileName != "" {
		name = overrides.ProfileName
	}
	if name == "" {
		name = "normal"
	}

	profile, ok := cfg.Profiles[name]
	if !ok {
		return EffectiveScan{}, fmt.Errorf("unknown scan profile: %s", name)
	}

	out := EffectiveScan{
		ProfileName: name,
		HostWorkers: profile.HostWorkers,
		ToolArgs:    profile.ToolArgs,
	}
	if out.HostWorkers <= 0 {
		out.HostWorkers = 1
	}
	if overrides.HostWorkers > 0 {
		out.HostWorkers = overrides.HostWorkers
	}

	var err error
	if overrides.RustscanArgs != "" {
		out.Rustscan, err = SplitArgs(overrides.RustscanArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	if overrides.NmapArgs != "" {
		out.Nmap, err = SplitArgs(overrides.NmapArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	if overrides.HttpxArgs != "" {
		out.Httpx, err = SplitArgs(overrides.HttpxArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	if overrides.NucleiArgs != "" {
		out.Nuclei, err = SplitArgs(overrides.NucleiArgs)
		if err != nil {
			return EffectiveScan{}, err
		}
	}
	return out, nil
}

func builtInProfiles() map[string]Profile {
	return map[string]Profile{
		"slow": {
			HostWorkers: 1,
			ToolArgs: ToolArgs{
				Rustscan: []string{"--batch-size", "100", "--timeout", "3000"},
				Nmap:     []string{"-T2", "--max-retries", "3", "--scan-delay", "100ms"},
				Httpx:    []string{"-rate-limit", "20", "-threads", "5"},
				Nuclei:   []string{"-rate-limit", "10", "-c", "5", "-retries", "2"},
			},
		},
		"normal": {
			HostWorkers: 3,
			ToolArgs: ToolArgs{
				Rustscan: []string{"--batch-size", "500"},
				Nmap:     []string{"-T3", "--max-retries", "2"},
				Httpx:    []string{"-rate-limit", "100", "-threads", "20"},
				Nuclei:   []string{"-rate-limit", "50", "-c", "20"},
			},
		},
		"fast": {
			HostWorkers: 8,
			ToolArgs: ToolArgs{
				Rustscan: []string{"--batch-size", "1000"},
				Nmap:     []string{"-T4", "--max-retries", "1"},
				Httpx:    []string{"-rate-limit", "300", "-threads", "50"},
				Nuclei:   []string{"-rate-limit", "150", "-c", "50"},
			},
		},
	}
}
