package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type migration struct {
	version int
	name    string
	sql     string
	up      func(*sql.Tx) error
}

var migrations = []migration{
	{
		version: 1,
		name:    "create_base_schema",
		sql: `
CREATE TABLE IF NOT EXISTS fingerprints (
  run_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  service TEXT NOT NULL,
  product TEXT NOT NULL,
  version TEXT NOT NULL DEFAULT '',
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
  exclude_targets TEXT NOT NULL DEFAULT '',
  exclude_ports TEXT NOT NULL DEFAULT '',
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
);`,
		up: func(tx *sql.Tx) error {
			for _, stmt := range []string{
				`ALTER TABLE projects ADD COLUMN exclude_targets TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE projects ADD COLUMN exclude_ports TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE fingerprints ADD COLUMN version TEXT NOT NULL DEFAULT ''`,
			} {
				if _, err := tx.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 2,
		name:    "add_scan_run_artifact_dir",
		up: func(tx *sql.Tx) error {
			if _, err := tx.Exec(`ALTER TABLE scan_runs ADD COLUMN artifact_dir TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
			return nil
		},
	},
	{
		version: 3,
		name:    "add_nmap_import_fields",
		up: func(tx *sql.Tx) error {
			for _, stmt := range []string{
				`ALTER TABLE fingerprints ADD COLUMN protocol TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE fingerprints ADD COLUMN cpe TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE fingerprints ADD COLUMN extrainfo TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE fingerprints ADD COLUMN tunnel TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE findings ADD COLUMN protocol TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE findings ADD COLUMN scope TEXT NOT NULL DEFAULT ''`,
			} {
				if _, err := tx.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 4,
		name:    "create_run_leases",
		sql: `
CREATE TABLE run_leases (
  scope TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  owner_token TEXT NOT NULL,
  heartbeat_at TEXT NOT NULL
);`,
	},
	{
		version: 5,
		name:    "add_run_lease_heartbeat_nanoseconds",
		up: func(tx *sql.Tx) error {
			if _, err := tx.Exec(`ALTER TABLE run_leases ADD COLUMN heartbeat_at_ns INTEGER NOT NULL DEFAULT 0`); err != nil {
				return err
			}
			rows, err := tx.Query(`SELECT scope, heartbeat_at FROM run_leases`)
			if err != nil {
				return err
			}
			heartbeats := map[string]int64{}
			for rows.Next() {
				var scope, heartbeat string
				if err := rows.Scan(&scope, &heartbeat); err != nil {
					return err
				}
				at, err := time.Parse(time.RFC3339Nano, heartbeat)
				if err != nil {
					return err
				}
				heartbeats[scope] = at.UnixNano()
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
			for scope, heartbeat := range heartbeats {
				if _, err := tx.Exec(`UPDATE run_leases SET heartbeat_at_ns = ? WHERE scope = ?`, heartbeat, scope); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 6,
		name:    "create_detection_checks",
		sql: `
CREATE TABLE detection_checks (
  run_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  protocol TEXT NOT NULL,
  engine TEXT NOT NULL,
  status TEXT NOT NULL,
  reason_code TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL DEFAULT '',
  finished_at TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (run_id, ip, port, protocol, engine)
);`,
	},
	{
		version: 7,
		name:    "add_fingerprint_natural_key",
		up: func(tx *sql.Tx) error {
			if _, err := tx.Exec(`DELETE FROM fingerprints WHERE rowid NOT IN (
				SELECT MAX(rowid) FROM fingerprints GROUP BY run_id, ip, port, protocol
			)`); err != nil {
				return err
			}
			_, err := tx.Exec(`CREATE UNIQUE INDEX fingerprints_run_key ON fingerprints (run_id, ip, port, protocol)`)
			return err
		},
	},
	{
		version: 8,
		name:    "reserved_builtin_probe_identity",
		up:      func(*sql.Tx) error { return nil },
	},
	{
		version: 9,
		name:    "remove_builtin_probe_identity",
		up: func(tx *sql.Tx) error {
			for _, stmt := range []string{
				`ALTER TABLE detection_checks RENAME TO detection_checks_legacy`,
				`CREATE TABLE detection_checks (
  run_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  protocol TEXT NOT NULL,
  engine TEXT NOT NULL,
  status TEXT NOT NULL,
  reason_code TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL DEFAULT '',
  finished_at TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (run_id, ip, port, protocol, engine)
)`,
				`INSERT INTO detection_checks (run_id, ip, port, protocol, engine, status, reason_code, detail, started_at, finished_at)
SELECT run_id, ip, port, protocol, engine, status, reason_code, detail, started_at, finished_at
FROM detection_checks_legacy WHERE engine <> 'builtin'`,
				`DROP TABLE detection_checks_legacy`,
			} {
				if _, err := tx.Exec(stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		version: 10,
		name:    "add_project_report_metadata_and_zones",
		up: func(tx *sql.Tx) error {
			columns := []struct {
				table string
				name  string
				def   string
			}{
				{"projects", "client_unit", "TEXT NOT NULL DEFAULT ''"},
				{"projects", "report_title", "TEXT NOT NULL DEFAULT ''"},
				{"projects", "test_object", "TEXT NOT NULL DEFAULT ''"},
				{"projects", "start_date", "TEXT NOT NULL DEFAULT ''"},
				{"projects", "end_date", "TEXT NOT NULL DEFAULT ''"},
				{"projects", "testers", "TEXT NOT NULL DEFAULT ''"},
				{"scan_runs", "zone_id", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, col := range columns {
				if !hasTable(tx, col.table) {
					continue
				}
				if hasColumn(tx, col.table, col.name) {
					continue
				}
				if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, col.table, col.name, col.def)); err != nil {
					return err
				}
			}
			if !hasTable(tx, "project_zones") {
				for _, stmt := range []string{
					`CREATE TABLE IF NOT EXISTS project_zones (
  project_id TEXT NOT NULL,
  zone_id TEXT NOT NULL,
  name TEXT NOT NULL,
  sort_order INTEGER NOT NULL,
  PRIMARY KEY (project_id, zone_id)
)`,
					`CREATE INDEX IF NOT EXISTS idx_project_zones_project ON project_zones (project_id)`,
				} {
					if _, err := tx.Exec(stmt); err != nil {
						return err
					}
				}
			}
			return nil
		},
	},
	{
		version: 11,
		name:    "add_scan_run_zone_fields",
		up: func(tx *sql.Tx) error {
			if !hasTable(tx, "scan_runs") {
				return nil
			}
			for _, col := range []struct {
				name string
				def  string
			}{
				{"kind", "TEXT NOT NULL DEFAULT 'scan'"},
				{"label", "TEXT NOT NULL DEFAULT ''"},
				{"access_point", "TEXT NOT NULL DEFAULT ''"},
				{"tester_ip", "TEXT NOT NULL DEFAULT ''"},
				{"notes", "TEXT NOT NULL DEFAULT ''"},
				{"include_in_report", "INTEGER NOT NULL DEFAULT 0"},
			} {
				if hasColumn(tx, "scan_runs", col.name) {
					continue
				}
				if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE scan_runs ADD COLUMN %s %s`, col.name, col.def)); err != nil {
					return err
				}
			}
			// Existing completed scans default into the report; tool/failed/running do not.
			if _, err := tx.Exec(`UPDATE scan_runs SET include_in_report = 1 WHERE status IN ('completed', 'completed_with_errors') AND profile NOT LIKE 'tool:%'`); err != nil {
				return err
			}
			return nil
		},
	},
	{
		version: 12,
		name:    "create_report_verifications_and_evidence",
		up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS report_verifications (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  zone_id TEXT NOT NULL,
  vulnerability_key TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK(outcome IN ('confirmed','not_observed','inconclusive')),
  title TEXT NOT NULL,
  severity TEXT NOT NULL,
  description TEXT NOT NULL,
  remediation TEXT NOT NULL,
  notes TEXT NOT NULL,
  included INTEGER NOT NULL,
  position INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_report_verifications_project ON report_verifications(project_id);
CREATE INDEX IF NOT EXISTS idx_report_verifications_zone ON report_verifications(project_id, zone_id);

CREATE TABLE IF NOT EXISTS verification_assets (
  verification_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  protocol TEXT NOT NULL,
  asset_name TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL,
  PRIMARY KEY (verification_id, ip, port, protocol),
  FOREIGN KEY (verification_id) REFERENCES report_verifications(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS verification_sources (
  verification_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  source TEXT NOT NULL,
  finding_id TEXT NOT NULL,
  ip TEXT NOT NULL,
  port INTEGER NOT NULL,
  protocol TEXT NOT NULL,
  PRIMARY KEY (verification_id, run_id, source, finding_id, ip, port, protocol),
  FOREIGN KEY (verification_id) REFERENCES report_verifications(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS verification_evidence (
  id TEXT PRIMARY KEY,
  verification_id TEXT NOT NULL,
  relative_path TEXT NOT NULL,
  media_type TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  width INTEGER NOT NULL,
  height INTEGER NOT NULL,
  caption TEXT NOT NULL,
  position INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (verification_id) REFERENCES report_verifications(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_verification_evidence_verification ON verification_evidence(verification_id);`)
			return err
		},
	},
}

func hasTable(tx *sql.Tx, table string) bool {
	row := tx.QueryRow(`SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?`, table)
	var one int
	if err := row.Scan(&one); err != nil {
		return false
	}
	return true
}

func hasColumn(tx *sql.Tx, table, column string) bool {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func runMigrations(db *sql.DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL
);`); err != nil {
		return err
	}

	applied, err := appliedMigrations(db)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if applied[migration.version] {
			continue
		}
		if err := applyMigration(db, migration); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", migration.version, migration.name, err)
		}
	}

	return nil
}

func appliedMigrations(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func applyMigration(db *sql.DB, migration migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if migration.sql != "" {
		if _, err := tx.Exec(migration.sql); err != nil {
			return err
		}
	}
	if migration.up != nil {
		if err := migration.up(tx); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		migration.version,
		migration.name,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return err
	}

	return tx.Commit()
}
