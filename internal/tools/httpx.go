package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
)

type HTTPResult struct {
	URL        string   `json:"url"`
	StatusCode int      `json:"status-code"`
	Title      string   `json:"title"`
	Tech       []string `json:"tech"`
}

func EnrichWeb(ctx context.Context, runner Runner, binaryPath string, fp fingerprint.ServiceFingerprint, extraArgs []string) (HTTPResult, error) {
	args := []string{"-json", "-status-code", "-title", "-tech-detect", "-follow-redirects", "-u", fp.URL}
	args = append(args, extraArgs...)

	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return HTTPResult{}, err
	}

	line := lastJSONLine(string(out))
	var result HTTPResult
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return HTTPResult{}, err
	}
	return result, nil
}

func lastJSONLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			return line
		}
	}
	return strings.TrimSpace(output)
}
