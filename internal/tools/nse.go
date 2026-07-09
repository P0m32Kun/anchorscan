package tools

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"
)

type NSEScripResult struct {
	ID     string
	Output string
}

type nseXML struct {
	Hosts []struct {
		Ports []struct {
			Scripts []struct {
				ID     string `xml:"id,attr"`
				Output string `xml:"output,attr"`
			} `xml:"script"`
		} `xml:"ports>port"`
	} `xml:"host"`
}

func RunNSE(ctx context.Context, runner Runner, binaryPath string, ip string, port int, scripts []string, extraArgs []string) ([]NSEScripResult, error) {
	results, _, err := RunNSEWithOutput(ctx, runner, binaryPath, ip, port, scripts, extraArgs)
	return results, err
}

func RunNSEWithOutput(ctx context.Context, runner Runner, binaryPath string, ip string, port int, scripts []string, extraArgs []string) ([]NSEScripResult, []byte, error) {
	args := []string{"-p", strconv.Itoa(port), "--script", strings.Join(scripts, ","), ip, "-oX", "-"}
	args = append(args, extraArgs...)

	out, err := runner.Run(ctx, binaryPath, args)
	if err != nil {
		return nil, out, withOutputError(err, out)
	}

	var parsed nseXML
	if err := xml.Unmarshal(out, &parsed); err != nil {
		return nil, out, err
	}

	var results []NSEScripResult
	for _, host := range parsed.Hosts {
		for _, port := range host.Ports {
			for _, script := range port.Scripts {
				results = append(results, NSEScripResult{
					ID:     script.ID,
					Output: script.Output,
				})
			}
		}
	}
	return results, out, nil
}
