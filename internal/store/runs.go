package store

import "time"

func (s *Store) SaveScanRun(run ScanRun) error {
	_, err := s.db.Exec(
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
	)
	return err
}

func (s *Store) GetScanRun(runID string) (ScanRun, error) {
	row := s.db.QueryRow(
		`SELECT run_id, project_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir
		 FROM scan_runs
		 WHERE run_id = ?`,
		runID,
	)

	return scanRunFromRow(row.Scan)
}

func (s *Store) ListScanRuns(limit int) ([]ScanRun, error) {
	rows, err := s.db.Query(
		`SELECT run_id, project_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir
		 FROM scan_runs
		 ORDER BY started_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRunsFromRows(rows)
}

func (s *Store) ListProjectScanRuns(projectID string, limit int) ([]ScanRun, error) {
	rows, err := s.db.Query(
		`SELECT run_id, project_id, target, ports, profile, status, started_at, finished_at, error, config_snapshot, artifact_dir
		 FROM scan_runs
		 WHERE project_id = ?
		 ORDER BY started_at DESC
		 LIMIT ?`,
		projectID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRunsFromRows(rows)
}

func (s *Store) DeleteScanRunCascade(runID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	for _, stmt := range []string{
		`DELETE FROM findings WHERE run_id = ?`,
		`DELETE FROM fingerprints WHERE run_id = ?`,
		`DELETE FROM scan_events WHERE run_id = ?`,
		`DELETE FROM scan_runs WHERE run_id = ?`,
	} {
		if _, err := tx.Exec(stmt, runID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return nil
}

func (s *Store) ListProjectArtifactDirs(projectID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT artifact_dir
		 FROM scan_runs
		 WHERE project_id = ? AND artifact_dir != ''
		 ORDER BY started_at ASC, run_id ASC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []string
	for rows.Next() {
		var dir string
		if err := rows.Scan(&dir); err != nil {
			return nil, err
		}
		dirs = append(dirs, dir)
	}
	return dirs, rows.Err()
}

func (s *Store) UpdateScanRunStatus(runID string, status string, message string, finishedAt time.Time) error {
	_, err := s.db.Exec(
		`UPDATE scan_runs
		 SET status = ?, error = ?, finished_at = ?
		 WHERE run_id = ?`,
		status,
		message,
		formatTime(finishedAt),
		runID,
	)
	return err
}

func (s *Store) AppendScanEvent(event ScanEvent) error {
	_, err := s.db.Exec(
		`INSERT INTO scan_events (run_id, time, level, stage, message)
		 VALUES (?, ?, ?, ?, ?)`,
		event.RunID,
		formatTime(event.Time),
		event.Level,
		event.Stage,
		event.Message,
	)
	return err
}

func (s *Store) ListScanEvents(runID string, limit int) ([]ScanEvent, error) {
	rows, err := s.db.Query(
		`SELECT id, run_id, time, level, stage, message
		 FROM scan_events
		 WHERE run_id = ?
		 ORDER BY id ASC
		 LIMIT ?`,
		runID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]ScanEvent, 0)
	for rows.Next() {
		var event ScanEvent
		var at string
		if err := rows.Scan(&event.ID, &event.RunID, &at, &event.Level, &event.Stage, &event.Message); err != nil {
			return nil, err
		}

		event.Time, err = parseTime(at)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

func scanRunFromRow(scan func(dest ...any) error) (ScanRun, error) {
	var run ScanRun
	var startedAt string
	var finishedAt string
	if err := scan(
		&run.RunID,
		&run.ProjectID,
		&run.Target,
		&run.Ports,
		&run.Profile,
		&run.Status,
		&startedAt,
		&finishedAt,
		&run.Error,
		&run.ConfigSnapshot,
		&run.ArtifactDir,
	); err != nil {
		return ScanRun{}, err
	}

	var err error
	run.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return ScanRun{}, err
	}
	run.FinishedAt, err = parseTime(finishedAt)
	if err != nil {
		return ScanRun{}, err
	}

	return run, nil
}

func scanRunsFromRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]ScanRun, error) {
	var runs []ScanRun
	for rows.Next() {
		run, err := scanRunFromRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
