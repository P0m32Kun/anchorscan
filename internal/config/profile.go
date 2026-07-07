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
