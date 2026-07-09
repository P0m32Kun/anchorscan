package tools

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
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

	parsed, err := fingerprint.ParseNmapXML(out)
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
	alive, _, err := DiscoverAliveWithOutput(ctx, runner, binaryPath, targets, extraArgs)
	return alive, err
}

func DiscoverAliveWithOutput(ctx context.Context, runner Runner, binaryPath string, targets []string, extraArgs []string) ([]string, []byte, error) {
	args := []string{"-sn"}
	args = append(args, targets...)
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
	seen := make(map[string]struct{}, len(parsed.Hosts))
	alive := make([]string, 0, len(parsed.Hosts))
	for _, host := range parsed.Hosts {
		if host.Status.State != "up" {
			continue
		}
		addr := host.Address.Addr
		if addr == "" && len(targets) == 1 {
			addr = targets[0]
		}
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		alive = append(alive, addr)
	}
	return alive, out, nil
}

func CheckAlive(ctx context.Context, runner Runner, binaryPath string, target string, extraArgs []string) (bool, error) {
	alive, err := DiscoverAlive(ctx, runner, binaryPath, []string{target}, extraArgs)
	if err != nil {
		return false, err
	}
	return len(alive) > 0, nil
}
