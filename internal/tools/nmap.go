package tools

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/target"
)

func Fingerprint(ctx context.Context, runner Runner, binaryPath string, ip string, ports []int, extraArgs []string) ([]fingerprint.ServiceFingerprint, error) {
	result, _, err := FingerprintWithOutput(ctx, runner, binaryPath, ip, ports, extraArgs)
	return result, err
}

func FingerprintWithOutput(ctx context.Context, runner Runner, binaryPath string, ip string, ports []int, extraArgs []string) ([]fingerprint.ServiceFingerprint, []byte, error) {
	args := []string{"-sV", "--version-intensity", "7", "-p", joinPorts(ports), ip, "-oX", "-"}
	args = append(args, extraArgs...)

	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return nil, out, withOutputError(err, out)
	}

	parsed, _, err := fingerprint.ParseNmapXML(out)
	if err != nil {
		return nil, out, err
	}

	result := make([]fingerprint.ServiceFingerprint, 0, len(parsed))
	for _, fp := range parsed {
		result = append(result, fingerprint.Classify(fp))
	}
	return result, out, nil
}

func joinPorts(ports []int) string {
	items := make([]string, 0, len(ports))
	for _, port := range ports {
		items = append(items, strconv.Itoa(port))
	}
	return strings.Join(items, ",")
}

type aliveXML struct {
	Hosts []struct {
		Status struct {
			State string `xml:"state,attr"`
		} `xml:"status"`
		Address struct {
			Addr string `xml:"addr,attr"`
		} `xml:"address"`
	} `xml:"host"`
}

func DiscoverAlive(ctx context.Context, runner Runner, binaryPath string, targets []string, extraArgs []string) ([]string, error) {
	scope, err := target.ParseScope(strings.Join(targets, ","), "")
	if err != nil {
		return nil, err
	}
	return DiscoverAliveInScope(ctx, runner, binaryPath, scope, extraArgs)
}

func DiscoverAliveWithOutput(ctx context.Context, runner Runner, binaryPath string, targets []string, extraArgs []string) ([]string, []byte, error) {
	scope, err := target.ParseScope(strings.Join(targets, ","), "")
	if err != nil {
		return nil, nil, err
	}
	return DiscoverAliveInScopeWithOutput(ctx, runner, binaryPath, scope, extraArgs)
}

func DiscoverAliveInScope(ctx context.Context, runner Runner, binaryPath string, scope target.Scope, extraArgs []string) ([]string, error) {
	alive, _, err := DiscoverAliveInScopeWithOutput(ctx, runner, binaryPath, scope, extraArgs)
	return alive, err
}

func DiscoverAliveInScopeWithOutput(ctx context.Context, runner Runner, binaryPath string, scope target.Scope, extraArgs []string) ([]string, []byte, error) {
	args := []string{"-sn"}
	args = append(args, scope.NmapTargets()...)
	if excludes := scope.NmapExcludes(); len(excludes) > 0 {
		args = append(args, "--exclude", strings.Join(excludes, ","))
	}
	args = append(args, "-oX", "-")
	args = append(args, extraArgs...)

	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return nil, out, withOutputError(err, out)
	}

	var parsed aliveXML
	if err := xml.Unmarshal(out, &parsed); err != nil {
		return nil, out, err
	}
	addresses := make([]string, 0, len(parsed.Hosts))
	for _, host := range parsed.Hosts {
		if host.Status.State != "up" {
			continue
		}
		address := host.Address.Addr
		if address == "" {
			address, _ = scope.SingleAddress()
		}
		addresses = append(addresses, address)
	}
	return scope.Filter(addresses), out, nil
}

func CheckAlive(ctx context.Context, runner Runner, binaryPath string, target string, extraArgs []string) (bool, error) {
	alive, err := DiscoverAlive(ctx, runner, binaryPath, []string{target}, extraArgs)
	if err != nil {
		return false, err
	}
	return len(alive) > 0, nil
}
