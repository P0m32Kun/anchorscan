package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/report"
)

type Store struct {
	db       *sql.DB
	dataRoot string
}

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000; PRAGMA journal_mode = WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db, dataRoot: filepath.Dir(path)}, nil
}

func (s *Store) managedProjectDir(projectID string) string {
	return filepath.Join(s.dataRoot, "projects", projectID)
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveFingerprint(runID string, fp fingerprint.ServiceFingerprint) error {
	_, err := s.db.Exec(
		`INSERT INTO fingerprints (run_id, ip, port, protocol, service, product, version, extrainfo, tunnel, cpe, normalized, is_web, url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, fp.IP, fp.Port, fp.Protocol, fp.Service, fp.Product, fp.Version, fp.ExtraInfo, fp.Tunnel, fp.CPE, fp.Normalized, boolToInt(fp.IsWeb), fp.URL,
	)
	return err
}

func (s *Store) UpsertFingerprint(runID string, fp fingerprint.ServiceFingerprint) error {
	_, err := s.db.Exec(
		`INSERT INTO fingerprints (run_id, ip, port, protocol, service, product, version, extrainfo, tunnel, cpe, normalized, is_web, url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(run_id, ip, port, protocol) DO UPDATE SET
		 service = excluded.service, product = excluded.product, version = excluded.version, extrainfo = excluded.extrainfo,
		 tunnel = excluded.tunnel, cpe = excluded.cpe, normalized = excluded.normalized, is_web = excluded.is_web, url = excluded.url`,
		runID, fp.IP, fp.Port, fp.Protocol, fp.Service, fp.Product, fp.Version, fp.ExtraInfo, fp.Tunnel, fp.CPE, fp.Normalized, boolToInt(fp.IsWeb), fp.URL,
	)
	return err
}

func (s *Store) ListFingerprints(runID string) ([]fingerprint.ServiceFingerprint, error) {
	rows, err := s.db.Query(
		`SELECT ip, port, protocol, service, product, version, extrainfo, tunnel, cpe, normalized, is_web, url
		 FROM fingerprints
		 WHERE run_id = ?
		 ORDER BY ip, port, protocol`,
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
		if err := rows.Scan(&fp.IP, &fp.Port, &fp.Protocol, &fp.Service, &fp.Product, &fp.Version, &fp.ExtraInfo, &fp.Tunnel, &fp.CPE, &fp.Normalized, &isWeb, &fp.URL); err != nil {
			return nil, err
		}
		fp.IsWeb = isWeb == 1
		out = append(out, fp)
	}
	return out, rows.Err()
}

func (s *Store) SaveFinding(runID string, finding report.Finding) error {
	_, err := s.db.Exec(
		`INSERT INTO findings (run_id, ip, port, protocol, scope, source, finding_id, severity, summary, target, output)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, finding.IP, finding.Port, finding.Protocol, finding.Scope, finding.Source, finding.ID, finding.Severity, finding.Summary, finding.Target, finding.Output,
	)
	return err
}

func (s *Store) ListFindings(runID string) ([]report.Finding, error) {
	rows, err := s.db.Query(
		`SELECT ip, port, protocol, scope, source, finding_id, severity, summary, target, output
		 FROM findings
		 WHERE run_id = ?
		 ORDER BY ip, port, protocol, finding_id`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []report.Finding
	for rows.Next() {
		var finding report.Finding
		if err := rows.Scan(&finding.IP, &finding.Port, &finding.Protocol, &finding.Scope, &finding.Source, &finding.ID, &finding.Severity, &finding.Summary, &finding.Target, &finding.Output); err != nil {
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
