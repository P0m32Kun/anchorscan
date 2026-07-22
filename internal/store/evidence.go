package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	mediaTypePNG  = "image/png"
	mediaTypeJPEG = "image/jpeg"
)

var (
	errInvalidImageFormat = fmt.Errorf("only PNG and JPEG images are accepted")
)

func detectImageMediaType(data []byte) (string, error) {
	if len(data) < 2 {
		return "", errInvalidImageFormat
	}
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return mediaTypePNG, nil
	}
	if data[0] == 0xFF && data[1] == 0xD8 {
		return mediaTypeJPEG, nil
	}
	return "", errInvalidImageFormat
}

func imageExtension(mediaType string) (string, error) {
	switch mediaType {
	case mediaTypePNG:
		return ".png", nil
	case mediaTypeJPEG:
		return ".jpg", nil
	}
	return "", errInvalidImageFormat
}

// CreateEvidenceInput is the request to upload an evidence image.
type CreateEvidenceInput struct {
	VerificationID string
	Data           []byte
	Caption        string
	Position       int
}

// CreateEvidence validates an image, persists it in the managed project
// directory, and records the evidence metadata. It verifies that the
// verification belongs to the supplied project before writing anything.
func (s *Store) CreateEvidence(projectID string, input CreateEvidenceInput) (VerificationEvidence, error) {
	var empty VerificationEvidence

	mediaType, err := detectImageMediaType(input.Data)
	if err != nil {
		return empty, err
	}
	ext, err := imageExtension(mediaType)
	if err != nil {
		return empty, err
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil {
		return empty, fmt.Errorf("could not decode image dimensions: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return empty, fmt.Errorf("invalid image dimensions %dx%d", config.Width, config.Height)
	}

	row := s.db.QueryRow(`SELECT project_id FROM report_verifications WHERE id = ?`, input.VerificationID)
	var actualProjectID string
	if err := row.Scan(&actualProjectID); err != nil {
		return empty, err
	}
	if actualProjectID != projectID {
		return empty, fmt.Errorf("verification %s does not belong to project %s", input.VerificationID, projectID)
	}

	evidenceID := uuid.New().String()
	relativePath := filepath.Join("evidence", input.VerificationID, evidenceID+ext)
	absPath := filepath.Join(s.managedProjectDir(projectID), relativePath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return empty, err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(absPath), "evidence-*.tmp")
	if err != nil {
		return empty, err
	}
	tmpPath := tmpFile.Name()
	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := io.Copy(tmpFile, bytes.NewReader(input.Data)); err != nil {
		cleanup()
		return empty, err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return empty, err
	}

	hash := sha256.Sum256(input.Data)
	sha256hex := hex.EncodeToString(hash[:])

	createdAt := time.Now().UTC()
	_, err = s.db.Exec(`
		INSERT INTO verification_evidence (
			id, verification_id, relative_path, media_type, sha256, width, height, caption, position, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evidenceID, input.VerificationID, relativePath, mediaType, sha256hex,
		config.Width, config.Height, input.Caption, input.Position, formatTime(createdAt),
	)
	if err != nil {
		cleanup()
		return empty, err
	}

	if err := os.Rename(tmpPath, absPath); err != nil {
		_ = os.Remove(absPath)
		_, _ = s.db.Exec(`DELETE FROM verification_evidence WHERE id = ?`, evidenceID)
		cleanup()
		return empty, err
	}

	return VerificationEvidence{
		ID:             evidenceID,
		VerificationID: input.VerificationID,
		RelativePath:   relativePath,
		MediaType:      mediaType,
		SHA256:         sha256hex,
		Width:          config.Width,
		Height:         config.Height,
		Caption:        input.Caption,
		Position:       input.Position,
		CreatedAt:      createdAt,
	}, nil
}

// GetEvidence returns a single evidence row by ID.
func (s *Store) GetEvidence(id string) (VerificationEvidence, error) {
	var e VerificationEvidence
	var createdAt string
	if err := s.db.QueryRow(`
		SELECT id, verification_id, relative_path, media_type, sha256, width, height, caption, position, created_at
		FROM verification_evidence
		WHERE id = ?`, id).Scan(
		&e.ID, &e.VerificationID, &e.RelativePath, &e.MediaType, &e.SHA256,
		&e.Width, &e.Height, &e.Caption, &e.Position, &createdAt,
	); err != nil {
		return e, err
	}
	e.CreatedAt, _ = parseTime(createdAt)
	return e, nil
}

// ListVerificationEvidence returns evidence for a verification ordered by position and ID.
func (s *Store) ListVerificationEvidence(verificationID string) ([]VerificationEvidence, error) {
	rows, err := s.db.Query(`
		SELECT id, verification_id, relative_path, media_type, sha256, width, height, caption, position, created_at
		FROM verification_evidence
		WHERE verification_id = ?
		ORDER BY position ASC, id ASC`, verificationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evidence []VerificationEvidence
	for rows.Next() {
		var e VerificationEvidence
		var createdAt string
		if err := rows.Scan(
			&e.ID, &e.VerificationID, &e.RelativePath, &e.MediaType, &e.SHA256,
			&e.Width, &e.Height, &e.Caption, &e.Position, &createdAt,
		); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = parseTime(createdAt)
		evidence = append(evidence, e)
	}
	return evidence, rows.Err()
}

// EvidenceFilePath returns the absolute filesystem path for an evidence item.
func (s *Store) EvidenceFilePath(e VerificationEvidence, projectID string) string {
	return filepath.Join(s.managedProjectDir(projectID), e.RelativePath)
}

// DeleteEvidence removes an evidence file and its database row.
func (s *Store) DeleteEvidence(id string) error {
	var relativePath string
	var verificationID string
	if err := s.db.QueryRow(`SELECT relative_path, verification_id FROM verification_evidence WHERE id = ?`, id).Scan(&relativePath, &verificationID); err != nil {
		return err
	}

	row := s.db.QueryRow(`SELECT project_id FROM report_verifications WHERE id = ?`, verificationID)
	var projectID string
	if err := row.Scan(&projectID); err != nil {
		return err
	}

	absPath := filepath.Join(s.managedProjectDir(projectID), relativePath)
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if dir := filepath.Dir(absPath); dir != "" {
		_ = os.Remove(dir)
	}

	_, err := s.db.Exec(`DELETE FROM verification_evidence WHERE id = ?`, id)
	return err
}

// UpdateEvidenceCaption updates the caption of an evidence item.
func (s *Store) UpdateEvidenceCaption(id, caption string) error {
	_, err := s.db.Exec(`UPDATE verification_evidence SET caption = ? WHERE id = ?`, caption, id)
	return err
}

// UpdateEvidenceOrder updates the position of a single evidence item within its verification.
func (s *Store) UpdateEvidenceOrder(id string, position int) error {
	_, err := s.db.Exec(`UPDATE verification_evidence SET position = ? WHERE id = ?`, position, id)
	return err
}

// ReorderEvidence sets the positions of all evidence items for a verification.
// The IDs are expected in the desired order; each item gets a 0-based position.
func (s *Store) ReorderEvidence(verificationID string, ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`
			UPDATE verification_evidence SET position = ?
			WHERE id = ? AND verification_id = ?`, i, id, verificationID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
