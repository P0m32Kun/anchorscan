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
