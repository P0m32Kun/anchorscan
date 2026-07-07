package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type NucleiFinding struct {
	TemplateID string
	Name       string
	Severity   string
	MatchedAt  string
}

func RunNuclei(ctx context.Context, runner Runner, binaryPath string, target string, tags []string, extraArgs []string) ([]byte, error) {
	args := []string{"-target", target, "-tags", strings.Join(tags, ","), "-jsonl"}
	args = append(args, extraArgs...)
	return runner.Run(ctx, binaryPath, args)
}

func ParseNucleiJSONL(input []byte) ([]NucleiFinding, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(input)))
	var findings []NucleiFinding
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var row struct {
			TemplateID string `json:"template-id"`
			Info       struct {
				Name     string `json:"name"`
				Severity string `json:"severity"`
			} `json:"info"`
			MatchedAt string `json:"matched-at"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, err
		}
		findings = append(findings, NucleiFinding{
			TemplateID: row.TemplateID,
			Name:       row.Info.Name,
			Severity:   row.Info.Severity,
			MatchedAt:  row.MatchedAt,
		})
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return findings, nil
}
