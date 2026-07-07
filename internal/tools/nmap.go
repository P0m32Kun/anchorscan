package tools

import (
	"context"
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
