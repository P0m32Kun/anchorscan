package store

import (
	"errors"
	"time"
)

const globalRunLeaseScope = "global"

var ErrRunLeaseHeld = errors.New("run lease is held")

type RunLease struct {
	RunID       string
	OwnerToken  string
	HeartbeatAt time.Time
}

func (s *Store) AcquireRunLease(runID, ownerToken string, now time.Time, ttl time.Duration) (RunLease, error) {
	cutoff := now.Add(-ttl).UnixNano()
	heartbeat := now.UnixNano()
	result, err := s.db.Exec(`INSERT INTO run_leases (scope, run_id, owner_token, heartbeat_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(scope) DO UPDATE SET run_id = excluded.run_id, owner_token = excluded.owner_token, heartbeat_at = excluded.heartbeat_at
		WHERE run_leases.heartbeat_at < ?`, globalRunLeaseScope, runID, ownerToken, heartbeat, cutoff)
	if err != nil {
		return RunLease{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 0 {
		return RunLease{RunID: runID, OwnerToken: ownerToken, HeartbeatAt: now.UTC()}, nil
	}
	var lease RunLease
	var recorded int64
	if err := s.db.QueryRow(`SELECT run_id, owner_token, heartbeat_at FROM run_leases WHERE scope = ?`, globalRunLeaseScope).Scan(&lease.RunID, &lease.OwnerToken, &recorded); err != nil {
		return RunLease{}, err
	}
	lease.HeartbeatAt = time.Unix(0, recorded).UTC()
	return lease, ErrRunLeaseHeld
}

func (s *Store) RenewRunLease(runID, ownerToken string, now time.Time) (bool, error) {
	result, err := s.db.Exec(`UPDATE run_leases SET heartbeat_at = ? WHERE scope = ? AND run_id = ? AND owner_token = ?`, now.UnixNano(), globalRunLeaseScope, runID, ownerToken)
	if err != nil {
		return false, err
	}
	updated, err := result.RowsAffected()
	return updated == 1, err
}

func (s *Store) ReleaseRunLease(runID, ownerToken string) (bool, error) {
	result, err := s.db.Exec(`DELETE FROM run_leases WHERE scope = ? AND run_id = ? AND owner_token = ?`, globalRunLeaseScope, runID, ownerToken)
	if err != nil {
		return false, err
	}
	deleted, err := result.RowsAffected()
	return deleted == 1, err
}

func (s *Store) FinishRunWithLease(runID, ownerToken, status, message string, finishedAt time.Time) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE scan_runs SET status = ?, error = ?, finished_at = ?
		WHERE run_id = ? AND EXISTS (SELECT 1 FROM run_leases WHERE scope = ? AND run_id = ? AND owner_token = ?)`,
		status, message, formatTime(finishedAt), runID, globalRunLeaseScope, runID, ownerToken)
	if err != nil {
		return false, err
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return false, err
	}
	result, err = tx.Exec(`DELETE FROM run_leases WHERE scope = ? AND run_id = ? AND owner_token = ?`, globalRunLeaseScope, runID, ownerToken)
	if err != nil {
		return false, err
	}
	deleted, err := result.RowsAffected()
	if err != nil || deleted != 1 {
		return false, err
	}
	return true, tx.Commit()
}
