package report

import (
	"fmt"
	"net"
	"strconv"

	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
)

// CandidateCommand is a generated command for a project-level vulnerability
// candidate. TargetFile is only set for batch commands that need a file.
type CandidateCommand struct {
	FullCommand string `json:"full_command"`
	ToolArgs    string `json:"tool_args"`
	TargetFile  string `json:"target_file"`
}

// BuildCandidateCommands generates commands for the selected candidate assets.
// It returns one command per entry in the result slice; for Nuclei and Nmap
// batch commands this may be a single command covering multiple assets.
func BuildCandidateCommands(cand ProjectVulnerabilityCandidate, tool string, assets []ProjectAsset, catalog *knowledgebase.Catalog) ([]CandidateCommand, string, error) {
	if cand.IsPending {
		return nil, "", fmt.Errorf("知识库未匹配，无可用命令")
	}
	entry, ok := catalog.Entry(cand.GroupKey)
	if !ok {
		return nil, "", fmt.Errorf("知识库条目不存在")
	}
	if len(assets) == 0 {
		return nil, "", fmt.Errorf("没有选中资产")
	}

	findings := make([]Finding, 0, len(assets))
	for _, asset := range assets {
		findings = append(findings, candidateFinding(asset, entry))
	}

	switch tool {
	case "nuclei":
		return buildCandidateNuclei(findings, entry, catalog)
	case "nmap":
		return buildCandidateNmap(findings, entry, catalog)
	case "msf":
		return buildCandidateMSF(findings, entry, catalog)
	default:
		return nil, "", fmt.Errorf("unsupported tool %q", tool)
	}
}

func buildCandidateNuclei(findings []Finding, entry knowledgebase.Entry, catalog *knowledgebase.Catalog) ([]CandidateCommand, string, error) {
	if len(findings) == 1 {
		cmd, err := BuildNucleiCommand(findings[0], catalog)
		if err != nil {
			return nil, "", err
		}
		return []CandidateCommand{{FullCommand: cmd.FullCommand, ToolArgs: cmd.ToolArgs}}, "", nil
	}
	batch, err := BuildBatchNucleiCommand(findings, catalog, VulnerabilityGroupKey(entry.ID))
	if err != nil {
		return nil, "", err
	}
	return []CandidateCommand{{FullCommand: displayArgs(batch.Args), ToolArgs: displayArgs(batch.Args[1:]), TargetFile: batch.Args[len(batch.Args)-1]}}, "", nil
}

func buildCandidateNmap(findings []Finding, entry knowledgebase.Entry, catalog *knowledgebase.Catalog) ([]CandidateCommand, string, error) {
	if len(findings) == 1 {
		cmd, err := BuildNmapCommand(findings[0], catalog)
		if err != nil {
			return nil, "", err
		}
		return []CandidateCommand{{FullCommand: cmd.FullCommand, ToolArgs: cmd.ToolArgs}}, "", nil
	}
	batches, err := BuildBatchNmapCommands(findings, catalog, VulnerabilityGroupKey(entry.ID))
	if err != nil {
		return nil, "", err
	}
	commands := make([]CandidateCommand, len(batches))
	for i, batch := range batches {
		commands[i] = CandidateCommand{
			FullCommand: displayArgs(batch.Args),
			ToolArgs:    displayArgs(batch.Args[1:]),
			TargetFile:  batch.Args[len(batch.Args)-1],
		}
	}
	return commands, "", nil
}

func buildCandidateMSF(findings []Finding, entry knowledgebase.Entry, catalog *knowledgebase.Catalog) ([]CandidateCommand, string, error) {
	commands, err := BuildBatchMSFCommands(findings, catalog, VulnerabilityGroupKey(entry.ID))
	if err != nil {
		return nil, "", err
	}
	result := make([]CandidateCommand, len(commands))
	for i, cmd := range commands {
		result[i] = CandidateCommand{FullCommand: cmd}
	}
	return result, "MSF 命令在外部环境逐条执行，不使用服务端目标文件", nil
}

func candidateFinding(asset ProjectAsset, entry knowledgebase.Entry) Finding {
	target := asset.Target
	if target == "" && (asset.Protocol == "http" || asset.Protocol == "https") {
		target = asset.Protocol + "://" + net.JoinHostPort(asset.IP, strconv.Itoa(asset.Port))
	}
	return Finding{
		Source:   "nuclei",
		ID:       entry.ID,
		IP:       asset.IP,
		Port:     asset.Port,
		Protocol: asset.Protocol,
		Severity: string(entry.Severity),
		Summary:  entry.Name,
		Target:   target,
	}
}
