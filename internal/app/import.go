package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

// ImportNmapOptions 配置 import-nmap 命令。
type ImportNmapOptions struct {
	XMLPath   string
	XMLData   []byte
	RunID     string
	ProjectID string
	JSONPath  string
	HTMLPath  string
	Now       func() time.Time
}

// ImportNmap 把已有 Nmap XML 导入为一条完成态 AnchorScan run。
// 校验和解析在事务前完成；落库失败回滚，不会留下半截 run。
func ImportNmap(ctx context.Context, scanStore *store.Store, opts ImportNmapOptions) (string, error) {
	if opts.XMLPath == "" && len(opts.XMLData) == 0 {
		return "", errors.New("--xml is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	var data []byte
	if opts.XMLPath != "" {
		var err error
		data, err = os.ReadFile(opts.XMLPath)
		if err != nil {
			return "", err
		}
	} else {
		data = opts.XMLData
	}

	fps, scripts, err := fingerprint.ParseNmapXML(data)
	if err != nil {
		return "", err
	}

	// 复用现有 Classify 推断 IsWeb/URL/Normalized。
	classified := make([]fingerprint.ServiceFingerprint, 0, len(fps))
	for _, fp := range fps {
		classified = append(classified, fingerprint.Classify(fp))
	}
	findings := scriptsToFindings(scripts)

	now := opts.Now()
	runID := opts.RunID
	if runID == "" {
		runID = "import-" + now.Format("20060102-150405")
	}

	configSnapshot, _ := json.Marshal(map[string]any{
		"source":      "nmap-import",
		"xml":         opts.XMLPath,
		"imported_at": now.UTC().Format(time.RFC3339Nano),
	})

	run := store.ScanRun{
		RunID:          runID,
		ProjectID:      opts.ProjectID,
		Target:         "nmap-import",
		Profile:        "import",
		Status:         "completed",
		StartedAt:      now,
		FinishedAt:     now,
		ConfigSnapshot: string(configSnapshot),
	}

	if err := scanStore.SaveImportRun(run, classified, findings); err != nil {
		return "", err
	}

	if opts.JSONPath != "" || opts.HTMLPath != "" {
		storedFps, err := scanStore.ListFingerprints(runID)
		if err != nil {
			return runID, err
		}
		storedFindings, err := scanStore.ListFindings(runID)
		if err != nil {
			return runID, err
		}
		built := report.Build(storedFps, storedFindings)
		if opts.JSONPath != "" {
			if err := os.MkdirAll(filepath.Dir(opts.JSONPath), 0o755); err != nil {
				return runID, err
			}
			if err := report.WriteJSON(opts.JSONPath, built); err != nil {
				return runID, err
			}
		}
		if opts.HTMLPath != "" {
			if err := os.MkdirAll(filepath.Dir(opts.HTMLPath), 0o755); err != nil {
				return runID, err
			}
			if err := report.WriteHTML(opts.HTMLPath, built); err != nil {
				return runID, err
			}
		}
	}

	return runID, nil
}

// scriptsToFindings 把导入的 NSE 脚本转为可报告的 finding。
// port-level script 关联到对应端口；host/pre/post script 不伪造端口归属，
// 并通过 Source 编码 scope（如 nmap-import:hostscript:ssh-hostkey）。
func scriptsToFindings(scripts []fingerprint.ImportedScript) []report.Finding {
	out := make([]report.Finding, 0, len(scripts))
	for _, s := range scripts {
		finding := report.Finding{
			IP:       s.IP,
			Port:     s.Port,
			Protocol: s.Protocol,
			Source:   fmt.Sprintf("nmap-import:%s:%s", scriptScopeLabel(s.Scope), s.ID),
			ID:       s.ID,
			Severity: "info",
			Summary:  s.ID,
			Output:   s.Output,
		}
		if s.IP != "" {
			finding.Target = s.IP
		} else {
			finding.Target = "nmap-import"
		}
		out = append(out, finding)
	}
	return out
}

func scriptScopeLabel(scope string) string {
	switch scope {
	case "port":
		return "port"
	case "host":
		return "hostscript"
	case "pre":
		return "prescript"
	case "post":
		return "postscript"
	default:
		return strings.ToLower(scope)
	}
}
