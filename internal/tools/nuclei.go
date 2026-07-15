package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type NucleiFinding struct {
	TemplateID       string
	Name             string
	Severity         string
	Host             string
	IP               string
	Port             string
	URL              string
	MatchedAt        string
	MatcherName      string
	ExtractedResults []string
	CurlCommand      string
	Raw              string
}

func RunNuclei(ctx context.Context, runner Runner, binaryPath string, target string, tags []string, excludeTags []string, extraArgs []string) ([]byte, error) {
	args := []string{"-target", target, "-tags", strings.Join(tags, ","), "-jsonl"}
	if len(excludeTags) > 0 {
		args = append(args, "-etags", strings.Join(excludeTags, ","))
	}
	args = append(args, extraArgs...)
	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return out, withOutputError(err, out)
	}
	return out, nil
}

func RunNucleiTemplate(ctx context.Context, runner Runner, binaryPath string, target string, template string, extraArgs []string) ([]byte, error) {
	args := []string{"-target", target, "-t", template, "-jsonl"}
	args = append(args, extraArgs...)
	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return out, withOutputError(err, out)
	}
	return out, nil
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
			Host             string   `json:"host"`
			IP               string   `json:"ip"`
			Port             string   `json:"port"`
			URL              string   `json:"url"`
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
			Host:             row.Host,
			IP:               row.IP,
			Port:             row.Port,
			URL:              row.URL,
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

// Endpoint returns Nuclei's endpoint when present, otherwise the scan target.
func (f NucleiFinding) Endpoint(fallbackHost string, fallbackPort int) (string, int) {
	host := strings.TrimSpace(f.IP)
	if host == "" {
		for _, value := range []string{f.Host, f.MatchedAt} {
			host, _ = parseNucleiEndpoint(value)
			if host != "" {
				break
			}
		}
	}
	if host == "" {
		host = fallbackHost
	}

	port := parseNucleiPort(f.Port)
	if port == 0 {
		for _, value := range []string{f.Host, f.URL, f.MatchedAt} {
			_, port = parseNucleiEndpoint(value)
			if port != 0 {
				break
			}
		}
	}
	if port == 0 {
		port = fallbackPort
	}
	return host, port
}

func parseNucleiEndpoint(value string) (string, int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0
	}
	if addr, err := netip.ParseAddr(strings.Trim(value, "[]")); err == nil {
		return addr.String(), 0
	}

	candidate := value
	if !strings.Contains(candidate, "://") {
		candidate = "//" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Hostname() == "" {
		return "", 0
	}
	port := parseNucleiPort(parsed.Port())
	if port == 0 {
		switch parsed.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		}
	}
	return parsed.Hostname(), port
}

func parseNucleiPort(value string) int {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0
	}
	return port
}
