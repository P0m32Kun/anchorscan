package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// SaveRunProvenance persists the run manifest JSON. The manifest is opaque to
// the store; callers are responsible for its schema and backward compatibility.
func (s *Store) SaveRunProvenance(runID string, manifest string) error {
	_, err := s.db.Exec(
		`INSERT INTO run_provenance (run_id, manifest) VALUES (?, ?)
		 ON CONFLICT(run_id) DO UPDATE SET manifest = excluded.manifest`,
		runID,
		manifest,
	)
	return err
}

// GetRunProvenance returns the stored manifest JSON for a run. If no manifest
// exists (legacy run), it returns an empty string and a nil error.
func (s *Store) GetRunProvenance(runID string) (string, error) {
	row := s.db.QueryRow(`SELECT manifest FROM run_provenance WHERE run_id = ?`, runID)
	var manifest string
	if err := row.Scan(&manifest); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return manifest, nil
}

// RunProvenanceArtifactHashes extracts the artifact_hashes field from a
// manifest JSON. It is a convenience for callers that do not want to parse the
// full manifest themselves.
func RunProvenanceArtifactHashes(manifest string) (map[string]string, error) {
	if manifest == "" {
		return map[string]string{}, nil
	}
	var parsed struct {
		ArtifactHashes map[string]string `json:"artifact_hashes"`
	}
	if err := json.Unmarshal([]byte(manifest), &parsed); err != nil {
		return nil, err
	}
	if parsed.ArtifactHashes == nil {
		return map[string]string{}, nil
	}
	return parsed.ArtifactHashes, nil
}
