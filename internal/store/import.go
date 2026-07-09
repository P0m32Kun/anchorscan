package store

import (
	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

// SaveImportRun 在单个事务内创建扫描 run 并批量写入 fingerprints 与 findings。
// 任一步失败都会回滚，保证导入失败时不留下半截 run 数据。
func (s *Store) SaveImportRun(run ScanRun, fps []fingerprint.ServiceFingerprint, findings []report.Finding) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO scan_runs (
			run_id, project_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			project_id = excluded.project_id,
			target = excluded.target,
			ports = excluded.ports,
			profile = excluded.profile,
			status = excluded.status,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			error = excluded.error,
			config_snapshot = excluded.config_snapshot,
			artifact_dir = excluded.artifact_dir`,
		run.RunID,
		run.ProjectID,
		run.Target,
		run.Ports,
		run.Profile,
		run.Status,
		formatTime(run.StartedAt),
		formatTime(run.FinishedAt),
		run.Error,
		run.ConfigSnapshot,
		run.ArtifactDir,
	); err != nil {
		return err
	}

	const fpStmt = `INSERT INTO fingerprints (run_id, ip, port, protocol, service, product, version, extrainfo, tunnel, cpe, normalized, is_web, url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for _, fp := range fps {
		if _, err := tx.Exec(fpStmt,
			run.RunID, fp.IP, fp.Port, fp.Protocol, fp.Service, fp.Product, fp.Version, fp.ExtraInfo, fp.Tunnel, fp.CPE, fp.Normalized, boolToInt(fp.IsWeb), fp.URL,
		); err != nil {
			return err
		}
	}

	const findingStmt = `INSERT INTO findings (run_id, ip, port, protocol, scope, source, finding_id, severity, summary, target, output)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	for _, finding := range findings {
		if _, err := tx.Exec(findingStmt,
			run.RunID, finding.IP, finding.Port, finding.Protocol, finding.Scope, finding.Source, finding.ID, finding.Severity, finding.Summary, finding.Target, finding.Output,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
