package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TargetSet is a Project's curated intake list of canonical scan targets. It is
// a prefill aid for Scan Create only; it never stores vulnerabilities, liveness,
// fingerprints, or run results, and it does not evolve into a CMDB.
type TargetSet struct {
	ID        string
	ProjectID string
	Name      string
	Targets   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TargetSetList returns the normalized target lines of a TargetSet.
func (t TargetSet) TargetList() []string {
	var out []string
	for _, line := range strings.Split(t.Targets, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// SaveTargetSet upserts the single Target Set of a project. The caller is
// responsible for passing already validated, normalized, deduped targets.
func (s *Store) SaveTargetSet(projectID, name, targetsText string) (TargetSet, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	createdAt := formatTime(now)
	updatedAt := createdAt

	var existingID string
	err := s.db.QueryRow(`SELECT id FROM project_target_sets WHERE project_id = ?`, projectID).Scan(&existingID)
	switch {
	case err == sql.ErrNoRows:
		_, err = s.db.Exec(`
			INSERT INTO project_target_sets (id, project_id, name, targets, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			id, projectID, name, targetsText, createdAt, updatedAt,
		)
		if err != nil {
			return TargetSet{}, err
		}
	case err != nil:
		return TargetSet{}, err
	default:
		if _, err := s.db.Exec(`
			UPDATE project_target_sets SET name = ?, targets = ?, updated_at = ?
			WHERE project_id = ?`,
			name, targetsText, updatedAt, projectID,
		); err != nil {
			return TargetSet{}, err
		}
		id = existingID
	}

	return s.GetTargetSet(projectID)
}

// GetTargetSet returns the project's Target Set or sql.ErrNoRows when absent.
func (s *Store) GetTargetSet(projectID string) (TargetSet, error) {
	var t TargetSet
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, project_id, name, targets, created_at, updated_at
		FROM project_target_sets
		WHERE project_id = ?`, projectID,
	).Scan(&t.ID, &t.ProjectID, &t.Name, &t.Targets, &createdAt, &updatedAt)
	if err != nil {
		return TargetSet{}, err
	}
	t.CreatedAt, _ = parseTime(createdAt)
	t.UpdatedAt, _ = parseTime(updatedAt)
	return t, nil
}

// DeleteProjectTargetSets removes a project's Target Set rows. It is part of
// the project cascade so deleting a project never leaves orphaned intake data.
func (s *Store) DeleteProjectTargetSets(projectID string) error {
	_, err := s.db.Exec(`DELETE FROM project_target_sets WHERE project_id = ?`, projectID)
	return err
}
