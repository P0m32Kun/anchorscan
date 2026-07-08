package store

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	schema := `
CREATE TABLE IF NOT EXISTS fingerprints (
  run_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  service TEXT NOT NULL,
  product TEXT NOT NULL,
  version TEXT NOT NULL,
  normalized TEXT NOT NULL,
  is_web INTEGER NOT NULL,
  url TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS findings (
  run_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  source TEXT NOT NULL,
  finding_id TEXT NOT NULL,
  severity TEXT NOT NULL,
  summary TEXT NOT NULL,
  target TEXT NOT NULL,
  output TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  default_targets TEXT NOT NULL,
  default_ports TEXT NOT NULL,
  exclude_targets TEXT NOT NULL,
  exclude_ports TEXT NOT NULL,
  default_profile TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS scan_runs (
  run_id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  target TEXT NOT NULL,
  ports TEXT NOT NULL,
  profile TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  error TEXT NOT NULL,
  config_snapshot TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS scan_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  time TEXT NOT NULL,
  level TEXT NOT NULL,
  stage TEXT NOT NULL,
  message TEXT NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	for _, stmt := range []string{
		`ALTER TABLE projects ADD COLUMN exclude_targets TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN exclude_ports TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fingerprints ADD COLUMN version TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			_ = db.Close()
			return nil, err
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) SaveFingerprint(runID string, fp fingerprint.ServiceFingerprint) error {
	_, err := s.db.Exec(
		`INSERT INTO fingerprints (run_id, ip, port, service, product, version, normalized, is_web, url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, fp.IP, fp.Port, fp.Service, fp.Product, fp.Version, fp.Normalized, boolToInt(fp.IsWeb), fp.URL,
	)
	return err
}

func (s *Store) ListFingerprints(runID string) ([]fingerprint.ServiceFingerprint, error) {
	rows, err := s.db.Query(
		`SELECT ip, port, service, product, version, normalized, is_web, url
		 FROM fingerprints
		 WHERE run_id = ?
		 ORDER BY ip, port`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []fingerprint.ServiceFingerprint
	for rows.Next() {
		var fp fingerprint.ServiceFingerprint
		var isWeb int
		if err := rows.Scan(&fp.IP, &fp.Port, &fp.Service, &fp.Product, &fp.Version, &fp.Normalized, &isWeb, &fp.URL); err != nil {
			return nil, err
		}
		fp.IsWeb = isWeb == 1
		out = append(out, fp)
	}
	return out, rows.Err()
}

func (s *Store) SaveFinding(runID string, finding report.Finding) error {
	_, err := s.db.Exec(
		`INSERT INTO findings (run_id, ip, port, source, finding_id, severity, summary, target, output)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, finding.IP, finding.Port, finding.Source, finding.ID, finding.Severity, finding.Summary, finding.Target, finding.Output,
	)
	return err
}

func (s *Store) ListFindings(runID string) ([]report.Finding, error) {
	rows, err := s.db.Query(
		`SELECT ip, port, source, finding_id, severity, summary, target, output
		 FROM findings
		 WHERE run_id = ?
		 ORDER BY ip, port, finding_id`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []report.Finding
	for rows.Next() {
		var finding report.Finding
		if err := rows.Scan(&finding.IP, &finding.Port, &finding.Source, &finding.ID, &finding.Severity, &finding.Summary, &finding.Target, &finding.Output); err != nil {
			return nil, err
		}
		out = append(out, finding)
	}
	return out, rows.Err()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

const sortableTimestampLayout = "2006-01-02T15:04:05.000000000Z07:00"

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(sortableTimestampLayout)
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}
