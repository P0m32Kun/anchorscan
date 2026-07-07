package store

import (
	"database/sql"

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
);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) SaveFingerprint(runID string, fp fingerprint.ServiceFingerprint) error {
	_, err := s.db.Exec(
		`INSERT INTO fingerprints (run_id, ip, port, service, product, normalized, is_web, url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, fp.IP, fp.Port, fp.Service, fp.Product, fp.Normalized, boolToInt(fp.IsWeb), fp.URL,
	)
	return err
}

func (s *Store) ListFingerprints(runID string) ([]fingerprint.ServiceFingerprint, error) {
	rows, err := s.db.Query(
		`SELECT ip, port, service, product, normalized, is_web, url
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
		if err := rows.Scan(&fp.IP, &fp.Port, &fp.Service, &fp.Product, &fp.Normalized, &isWeb, &fp.URL); err != nil {
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
