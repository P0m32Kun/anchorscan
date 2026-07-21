package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var validOutcomes = map[string]struct{}{
	"confirmed":    {},
	"not_observed": {},
	"inconclusive": {},
}

func validateOutcome(outcome string) error {
	outcome = strings.ToLower(strings.TrimSpace(outcome))
	if _, ok := validOutcomes[outcome]; !ok {
		return fmt.Errorf("invalid outcome %q: must be confirmed, not_observed or inconclusive", outcome)
	}
	return nil
}

// CreateVerification persists a verification together with its associated
// assets and sources. The caller supplies a stable ID and the outcome is
// restricted to confirmed/not_observed/inconclusive.
func (s *Store) CreateVerification(v Verification, assets []VerificationAsset, sources []VerificationSource) error {
	if err := validateOutcome(v.Outcome); err != nil {
		return err
	}
	if strings.TrimSpace(v.ID) == "" {
		v.ID = uuid.New().String()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO report_verifications (
			id, project_id, zone_id, vulnerability_key, outcome, title, severity,
			description, remediation, notes, included, position, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.ProjectID, v.ZoneID, v.VulnerabilityKey, v.Outcome, v.Title, v.Severity,
		v.Description, v.Remediation, v.Notes, boolToInt(v.Included), v.Position,
		formatTime(v.CreatedAt), formatTime(v.UpdatedAt),
	); err != nil {
		return err
	}

	for _, a := range assets {
		a.VerificationID = v.ID
		if _, err := tx.Exec(`
			INSERT INTO verification_assets (verification_id, ip, port, protocol, asset_name, position)
			VALUES (?, ?, ?, ?, ?, ?)`,
			a.VerificationID, a.IP, a.Port, a.Protocol, a.AssetName, a.Position,
		); err != nil {
			return err
		}
	}

	for _, src := range sources {
		src.VerificationID = v.ID
		if _, err := tx.Exec(`
			INSERT INTO verification_sources (verification_id, run_id, source, finding_id, ip, port, protocol)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			src.VerificationID, src.RunID, src.Source, src.FindingID, src.IP, src.Port, src.Protocol,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateVerification updates the mutable fields of a verification.
func (s *Store) UpdateVerification(v Verification) error {
	if err := validateOutcome(v.Outcome); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE report_verifications SET
			zone_id = ?,
			vulnerability_key = ?,
			outcome = ?,
			title = ?,
			severity = ?,
			description = ?,
			remediation = ?,
			notes = ?,
			included = ?,
			position = ?,
			updated_at = ?
		WHERE id = ?`,
		v.ZoneID, v.VulnerabilityKey, v.Outcome, v.Title, v.Severity,
		v.Description, v.Remediation, v.Notes, boolToInt(v.Included), v.Position,
		formatTime(v.UpdatedAt), v.ID,
	)
	return err
}

// VerificationWithAssociations is the full view of a verification.
type VerificationWithAssociations struct {
	Verification Verification
	Assets       []VerificationAsset
	Sources      []VerificationSource
	Evidence     []VerificationEvidence
}

// GetVerification returns a verification together with its assets, sources
// and evidence ordered by position.
func (s *Store) GetVerification(id string) (VerificationWithAssociations, error) {
	var result VerificationWithAssociations

	row := s.db.QueryRow(`
		SELECT id, project_id, zone_id, vulnerability_key, outcome, title, severity,
			description, remediation, notes, included, position, created_at, updated_at
		FROM report_verifications
		WHERE id = ?`, id)
	var createdAt, updatedAt string
	var include int
	if err := row.Scan(
		&result.Verification.ID, &result.Verification.ProjectID, &result.Verification.ZoneID,
		&result.Verification.VulnerabilityKey, &result.Verification.Outcome, &result.Verification.Title,
		&result.Verification.Severity, &result.Verification.Description, &result.Verification.Remediation,
		&result.Verification.Notes, &include, &result.Verification.Position, &createdAt, &updatedAt,
	); err != nil {
		return result, err
	}
	result.Verification.Included = include == 1
	result.Verification.CreatedAt, _ = parseTime(createdAt)
	result.Verification.UpdatedAt, _ = parseTime(updatedAt)

	assets, err := s.ListVerificationAssets(id)
	if err != nil {
		return result, err
	}
	result.Assets = assets

	sources, err := s.ListVerificationSources(id)
	if err != nil {
		return result, err
	}
	result.Sources = sources

	evidence, err := s.ListVerificationEvidence(id)
	if err != nil {
		return result, err
	}
	result.Evidence = evidence

	return result, nil
}

// ListVerificationAssets returns the assets associated with a verification.
func (s *Store) ListVerificationAssets(verificationID string) ([]VerificationAsset, error) {
	rows, err := s.db.Query(`
		SELECT verification_id, ip, port, protocol, asset_name, position
		FROM verification_assets
		WHERE verification_id = ?
		ORDER BY position ASC, ip ASC, port ASC`, verificationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []VerificationAsset
	for rows.Next() {
		var a VerificationAsset
		if err := rows.Scan(&a.VerificationID, &a.IP, &a.Port, &a.Protocol, &a.AssetName, &a.Position); err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

// ListVerificationSources returns the source facts associated with a verification.
func (s *Store) ListVerificationSources(verificationID string) ([]VerificationSource, error) {
	rows, err := s.db.Query(`
		SELECT verification_id, run_id, source, finding_id, ip, port, protocol
		FROM verification_sources
		WHERE verification_id = ?
		ORDER BY run_id ASC, source ASC, finding_id ASC`, verificationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []VerificationSource
	for rows.Next() {
		var src VerificationSource
		if err := rows.Scan(&src.VerificationID, &src.RunID, &src.Source, &src.FindingID, &src.IP, &src.Port, &src.Protocol); err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

// ListProjectVerifications returns all verifications for a project, ordered
// by position and ID. Evidence is not loaded; use GetVerification for the full view.
func (s *Store) ListProjectVerifications(projectID string) ([]Verification, error) {
	rows, err := s.db.Query(`
		SELECT id, project_id, zone_id, vulnerability_key, outcome, title, severity,
			description, remediation, notes, included, position, created_at, updated_at
		FROM report_verifications
		WHERE project_id = ?
		ORDER BY position ASC, id ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return verificationsFromRows(rows)
}

// ListZoneVerifications returns all verifications for a project zone.
func (s *Store) ListZoneVerifications(projectID, zoneID string) ([]Verification, error) {
	rows, err := s.db.Query(`
		SELECT id, project_id, zone_id, vulnerability_key, outcome, title, severity,
			description, remediation, notes, included, position, created_at, updated_at
		FROM report_verifications
		WHERE project_id = ? AND zone_id = ?
		ORDER BY position ASC, id ASC`, projectID, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return verificationsFromRows(rows)
}

func verificationsFromRows(rows *sql.Rows) ([]Verification, error) {
	var out []Verification
	for rows.Next() {
		var v Verification
		var createdAt, updatedAt string
		var include int
		if err := rows.Scan(
			&v.ID, &v.ProjectID, &v.ZoneID, &v.VulnerabilityKey, &v.Outcome, &v.Title, &v.Severity,
			&v.Description, &v.Remediation, &v.Notes, &include, &v.Position, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		v.Included = include == 1
		v.CreatedAt, _ = parseTime(createdAt)
		v.UpdatedAt, _ = parseTime(updatedAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

// DeleteVerificationCascade removes a verification, its associated rows, and all
// evidence files. It does not touch run artifacts.
func (s *Store) DeleteVerificationCascade(id string) error {
	v, err := s.GetVerification(id)
	if err != nil {
		return err
	}

	for _, e := range v.Evidence {
		if err := s.DeleteEvidence(e.ID); err != nil {
			return err
		}
	}

	if _, err := s.db.Exec(`DELETE FROM verification_assets WHERE verification_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM verification_sources WHERE verification_id = ?`, id); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM report_verifications WHERE id = ?`, id)
	return err
}

// DeleteProjectVerifications removes all verifications and evidence for a project.
func (s *Store) DeleteProjectVerifications(projectID string) error {
	verifications, err := s.ListProjectVerifications(projectID)
	if err != nil {
		return err
	}
	for _, v := range verifications {
		if err := s.DeleteVerificationCascade(v.ID); err != nil {
			return err
		}
	}
	return nil
}

// ZoneHasVerifications reports whether any verification is associated with the given zone.
func (s *Store) ZoneHasVerifications(projectID, zoneID string) (bool, error) {
	row := s.db.QueryRow(`
		SELECT COUNT(1) FROM report_verifications
		WHERE project_id = ? AND zone_id = ?`, projectID, zoneID)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// VerificationHasEvidence reports whether a verification has at least one evidence file.
func (s *Store) VerificationHasEvidence(verificationID string) (bool, error) {
	row := s.db.QueryRow(`SELECT COUNT(1) FROM verification_evidence WHERE verification_id = ?`, verificationID)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountVerificationEvidence returns the number of evidence items for a verification.
func (s *Store) CountVerificationEvidence(verificationID string) (int, error) {
	row := s.db.QueryRow(`SELECT COUNT(1) FROM verification_evidence WHERE verification_id = ?`, verificationID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// SetVerificationAssets replaces the assets of a verification.
func (s *Store) SetVerificationAssets(verificationID string, assets []VerificationAsset) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM verification_assets WHERE verification_id = ?`, verificationID); err != nil {
		return err
	}
	for _, a := range assets {
		a.VerificationID = verificationID
		if _, err := tx.Exec(`
			INSERT INTO verification_assets (verification_id, ip, port, protocol, asset_name, position)
			VALUES (?, ?, ?, ?, ?, ?)`,
			verificationID, a.IP, a.Port, a.Protocol, a.AssetName, a.Position,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SetVerificationSources replaces the source facts of a verification.
func (s *Store) SetVerificationSources(verificationID string, sources []VerificationSource) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM verification_sources WHERE verification_id = ?`, verificationID); err != nil {
		return err
	}
	for _, src := range sources {
		src.VerificationID = verificationID
		if _, err := tx.Exec(`
			INSERT INTO verification_sources (verification_id, run_id, source, finding_id, ip, port, protocol)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			verificationID, src.RunID, src.Source, src.FindingID, src.IP, src.Port, src.Protocol,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
