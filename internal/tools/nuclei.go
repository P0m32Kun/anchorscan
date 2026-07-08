package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type NucleiFinding struct {
	TemplateID       string
	Name             string
	Severity         string
	MatchedAt        string
	MatcherName      string
	ExtractedResults []string
	CurlCommand      string
	Raw              string
}

func RunNuclei(ctx context.Context, runner Runner, binaryPath string, target string, tags []string, extraArgs []string) ([]byte, error) {
	args := []string{"-target", target, "-tags", strings.Join(tags, ","), "-jsonl"}
	args = append(args, extraArgs...)
	return runner.Run(ctx, binaryPath, args)
}

func RunNucleiTemplate(ctx context.Context, runner Runner, binaryPath string, target string, template string, extraArgs []string) ([]byte, error) {
	args := []string{"-target", target, "-t", template, "-jsonl"}
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
			TemplateID       string   `json:"template-id"`
			MatcherName      string   `json:"matcher-name"`
			ExtractedResults []string `json:"extracted-results"`
			ExtractorResults []string `json:"extractor-results"`
			CurlCommand      string   `json:"curl-command"`
			Info             struct {
				Name     string `json:"name"`
				Severity string `json:"severity"`
			} `json:"info"`
			MatchedAt string `json:"matched-at"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, err
		}
		extracted := row.ExtractedResults
		if len(extracted) == 0 {
			extracted = row.ExtractorResults
		}
		raw := line
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(line), "", "  "); err == nil {
			raw = pretty.String()
		}
		findings = append(findings, NucleiFinding{
			TemplateID:       row.TemplateID,
			Name:             row.Info.Name,
			Severity:         row.Info.Severity,
			MatchedAt:        row.MatchedAt,
			MatcherName:      row.MatcherName,
			ExtractedResults: extracted,
			CurlCommand:      row.CurlCommand,
			Raw:              raw,
		})
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return findings, nil
}
