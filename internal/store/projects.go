package store

func (s *Store) SaveProject(project Project) error {
	_, err := s.db.Exec(
		`INSERT INTO projects (
			id, name, description, default_targets, default_ports, exclude_targets, exclude_ports, default_profile, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
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
		`SELECT id, name, description, default_targets, default_ports, exclude_targets, exclude_ports, default_profile, created_at, updated_at
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
		`SELECT id, name, description, default_targets, default_ports, exclude_targets, exclude_ports, default_profile, created_at, updated_at
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
