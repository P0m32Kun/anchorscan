package tools

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

func Fingerprint(ctx context.Context, runner Runner, binaryPath string, ip string, ports []int, extraArgs []string) ([]fingerprint.ServiceFingerprint, error) {
	args := []string{"-sV", "--version-intensity", "7", "-p", joinPorts(ports), ip, "-oX", "-"}
	args = append(args, extraArgs...)

	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return nil, err
	}

	parsed, err := fingerprint.ParseNmapXML(out)
	if err != nil {
		return nil, err
	}

	result := make([]fingerprint.ServiceFingerprint, 0, len(parsed))
	for _, fp := range parsed {
		result = append(result, fingerprint.Classify(fp))
	}
	return result, nil
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
	} `xml:"host"`
}

func CheckAlive(ctx context.Context, runner Runner, binaryPath string, target string, extraArgs []string) (bool, error) {
	args := []string{"-sn", target, "-oX", "-"}
	args = append(args, extraArgs...)

	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return false, err
	}

	var parsed aliveXML
	if err := xml.Unmarshal(out, &parsed); err != nil {
		return false, err
	}
	for _, host := range parsed.Hosts {
		if host.Status.State == "up" {
			return true, nil
		}
	}
	return false, nil
}
