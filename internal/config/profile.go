package config

import (
	"fmt"
	"strings"
)

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

func ValidateScopeSafeToolArgs(args ToolArgs) error {
	for _, item := range []struct {
		name string
		args []string
	}{
		{"rustscan", args.Rustscan},
		{"nmap", args.Nmap},
		{"httpx", args.Httpx},
		{"nuclei", args.Nuclei},
	} {
		if err := validateScopeSafeArgs(item.name, item.args); err != nil {
			return err
		}
	}
	return nil
}

func validateScopeSafeArgs(tool string, args []string) error {
	allowed := map[string]map[string]bool{
		"rustscan": {
			"--batch-size": true, "--timeout": true, "--tries": true, "--ulimit": true,
		},
		"nmap": {
			"-T0": false, "-T1": false, "-T2": false, "-T3": false, "-T4": false, "-T5": false,
			"-sV": false, "--version-light": false,
			"--max-retries": true, "--scan-delay": true, "--min-rate": true, "--max-rate": true,
			"--host-timeout": true, "--version-intensity": true,
		},
		"httpx": {
			"-rate-limit": true, "-threads": true, "-timeout": true, "-retries": true,
		},
		"nuclei": {
			"-rate-limit": true, "-c": true, "-retries": true, "-timeout": true,
		},
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		flag, _, hasValue := strings.Cut(arg, "=")
		takesValue, ok := allowed[tool][flag]
		if !ok {
			return fmt.Errorf("%s args are not allowed by the scan scope: %s", tool, arg)
		}
		if !takesValue {
			if hasValue {
				return fmt.Errorf("%s arg does not accept a value: %s", tool, flag)
			}
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return fmt.Errorf("%s arg requires a value: %s", tool, flag)
			}
			if strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("%s arg requires a non-flag value: %s", tool, flag)
			}
			i++
		}
	}
	return nil
}

func builtInProfiles() map[string]Profile {
	return map[string]Profile{
		"slow": {
			HostWorkers: 1,
			ToolArgs: ToolArgs{
				Rustscan: []string{"--batch-size", "500", "--timeout", "3000", "--tries", "2", "--ulimit", "5000"},
				Nmap:     []string{"-T2", "--max-retries", "3", "--scan-delay", "100ms"},
				Httpx:    []string{"-rate-limit", "20", "-threads", "5"},
				Nuclei:   []string{"-rate-limit", "10", "-c", "5", "-retries", "2"},
			},
		},
		"normal": {
			HostWorkers: 3,
			ToolArgs: ToolArgs{
				Rustscan: []string{"--batch-size", "1000", "--tries", "2", "--ulimit", "5000"},
				Nmap:     []string{"-T3", "--max-retries", "2"},
				Httpx:    []string{"-rate-limit", "100", "-threads", "20"},
				Nuclei:   []string{"-rate-limit", "50", "-c", "20"},
			},
		},
		"fast": {
			HostWorkers: 8,
			ToolArgs: ToolArgs{
				Rustscan: []string{"--batch-size", "1500", "--tries", "2", "--ulimit", "5000"},
				Nmap:     []string{"-T4", "--max-retries", "1"},
				Httpx:    []string{"-rate-limit", "300", "-threads", "50"},
				Nuclei:   []string{"-rate-limit", "150", "-c", "50"},
			},
		},
	}
}
