package store

func (s *Store) UpsertDetectionCheck(check DetectionCheck) error {
	_, err := s.db.Exec(`INSERT INTO detection_checks (
		run_id, ip, port, protocol, engine, status, reason_code, detail, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(run_id, ip, port, protocol, engine) DO UPDATE SET
		status = excluded.status,
		reason_code = excluded.reason_code,
		detail = excluded.detail,
		started_at = excluded.started_at,
		finished_at = excluded.finished_at
	WHERE detection_checks.status = 'running' OR excluded.status <> 'running'`,
		check.RunID, check.IP, check.Port, check.Protocol, check.Engine, check.Status,
		check.ReasonCode, check.Detail, formatTime(check.StartedAt), formatTime(check.FinishedAt),
	)
	return err
}

func (s *Store) ListDetectionChecks(runID string) ([]DetectionCheck, error) {
	rows, err := s.db.Query(`SELECT run_id, ip, port, protocol, engine, status, reason_code, detail, started_at, finished_at
		FROM detection_checks WHERE run_id = ? ORDER BY ip, port, protocol, engine`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var checks []DetectionCheck
	for rows.Next() {
		var check DetectionCheck
		var startedAt, finishedAt string
		if err := rows.Scan(&check.RunID, &check.IP, &check.Port, &check.Protocol, &check.Engine, &check.Status, &check.ReasonCode, &check.Detail, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		var err error
		if check.StartedAt, err = parseTime(startedAt); err != nil {
			return nil, err
		}
		if check.FinishedAt, err = parseTime(finishedAt); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, rows.Err()
}

func (s *Store) CountDetectionChecksByStatus(runID string) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM detection_checks WHERE run_id = ? GROUP BY status`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}
