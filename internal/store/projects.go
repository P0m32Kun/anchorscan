package store

import "database/sql"

func (s *Store) SaveProject(project Project) error {
	_, err := s.db.Exec(
		`INSERT INTO projects (
			id, name, description, client_unit, report_title, test_object, start_date, end_date, testers,
			default_targets, default_ports, exclude_targets, exclude_ports, default_profile, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			client_unit = excluded.client_unit,
			report_title = excluded.report_title,
			test_object = excluded.test_object,
			start_date = excluded.start_date,
			end_date = excluded.end_date,
			testers = excluded.testers,
			default_targets = excluded.default_targets,
			default_ports = excluded.default_ports,
			exclude_targets = excluded.exclude_targets,
			exclude_ports = excluded.exclude_ports,
			default_profile = excluded.default_profile,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at`,
		project.ID,
		project.Name,
		project.Description,
		project.ClientUnit,
		project.ReportTitle,
		project.TestObject,
		project.StartDate,
		project.EndDate,
		project.Testers,
		project.DefaultTargets,
		project.DefaultPorts,
		project.ExcludeTargets,
		project.ExcludePorts,
		project.DefaultProfile,
		formatTime(project.CreatedAt),
		formatTime(project.UpdatedAt),
	)
	return err
}

func (s *Store) GetProject(id string) (Project, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, client_unit, report_title, test_object, start_date, end_date, testers,
			default_targets, default_ports, exclude_targets, exclude_ports, default_profile, created_at, updated_at
		 FROM projects
		 WHERE id = ?`,
		id,
	)

	var project Project
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&project.ID,
		&project.Name,
		&project.Description,
		&project.ClientUnit,
		&project.ReportTitle,
		&project.TestObject,
		&project.StartDate,
		&project.EndDate,
		&project.Testers,
		&project.DefaultTargets,
		&project.DefaultPorts,
		&project.ExcludeTargets,
		&project.ExcludePorts,
		&project.DefaultProfile,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Project{}, err
	}

	var err error
	project.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Project{}, err
	}
	project.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return Project{}, err
	}

	return project, nil
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, client_unit, report_title, test_object, start_date, end_date, testers,
			default_targets, default_ports, exclude_targets, exclude_ports, default_profile, created_at, updated_at
		 FROM projects
		 ORDER BY created_at ASC, id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var project Project
		var createdAt string
		var updatedAt string
		if err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.ClientUnit,
			&project.ReportTitle,
			&project.TestObject,
			&project.StartDate,
			&project.EndDate,
			&project.Testers,
			&project.DefaultTargets,
			&project.DefaultPorts,
			&project.ExcludeTargets,
			&project.ExcludePorts,
			&project.DefaultProfile,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}

		project.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		project.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, err
		}

		projects = append(projects, project)
	}

	return projects, rows.Err()
}

func (s *Store) DeleteProject(id string) error {
	_, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteProjectCascade(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	for _, stmt := range []string{
		`DELETE FROM findings WHERE run_id IN (SELECT run_id FROM scan_runs WHERE project_id = ?)`,
		`DELETE FROM fingerprints WHERE run_id IN (SELECT run_id FROM scan_runs WHERE project_id = ?)`,
		`DELETE FROM scan_events WHERE run_id IN (SELECT run_id FROM scan_runs WHERE project_id = ?)`,
		`DELETE FROM scan_runs WHERE project_id = ?`,
		`DELETE FROM projects WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
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

func (s *Store) ProjectHasRunningRuns(id string) (bool, error) {
	row := s.db.QueryRow(`SELECT COUNT(1) FROM scan_runs WHERE project_id = ? AND status = ?`, id, "running")
	var count int
	if err := row.Scan(&count); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return count > 0, nil
}

// CreateProjectZone persists a single zone for a project.
func (s *Store) CreateProjectZone(zone ProjectZone) error {
	_, err := s.db.Exec(
		`INSERT INTO project_zones (project_id, zone_id, name, sort_order) VALUES (?, ?, ?, ?)`,
		zone.ProjectID, zone.ZoneID, zone.Name, zone.SortOrder,
	)
	return err
}

// ListProjectZones returns zones for a project ordered by sort_order and zone_id.
func (s *Store) ListProjectZones(projectID string) ([]ProjectZone, error) {
	rows, err := s.db.Query(
		`SELECT project_id, zone_id, name, sort_order
		 FROM project_zones
		 WHERE project_id = ?
		 ORDER BY sort_order ASC, zone_id ASC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []ProjectZone
	for rows.Next() {
		var zone ProjectZone
		if err := rows.Scan(&zone.ProjectID, &zone.ZoneID, &zone.Name, &zone.SortOrder); err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}
	return zones, rows.Err()
}

// DeleteProjectZone removes a zone. Callers must verify that the zone has no
// runs or verifications before invoking this method.
func (s *Store) DeleteProjectZone(projectID, zoneID string) error {
	_, err := s.db.Exec(
		`DELETE FROM project_zones WHERE project_id = ? AND zone_id = ?`,
		projectID, zoneID,
	)
	return err
}

// ZoneHasRuns reports whether any scan run is associated with the given zone.
func (s *Store) ZoneHasRuns(projectID, zoneID string) (bool, error) {
	row := s.db.QueryRow(
		`SELECT COUNT(1) FROM scan_runs WHERE project_id = ? AND zone_id = ?`,
		projectID, zoneID,
	)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateDefaultProjectZones creates the standard I, II, III zones for a project.
func (s *Store) CreateDefaultProjectZones(projectID string) error {
	for i, zone := range []struct {
		id   string
		name string
	}{
		{"I", "I区"},
		{"II", "II区"},
		{"III", "III区"},
	} {
		if err := s.CreateProjectZone(ProjectZone{
			ProjectID: projectID,
			ZoneID:    zone.id,
			Name:      zone.name,
			SortOrder: i,
		}); err != nil {
			return err
		}
	}
	return nil
}

// NextProjectZoneSortOrder returns the next available sort order for a custom zone.
func (s *Store) NextProjectZoneSortOrder(projectID string) (int, error) {
	row := s.db.QueryRow(
		`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM project_zones WHERE project_id = ?`,
		projectID,
	)
	var next int
	if err := row.Scan(&next); err != nil {
		return 0, err
	}
	return next, nil
}
