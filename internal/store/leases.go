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
	cutoff := now.Add(-ttl).UTC().Format(time.RFC3339Nano)
	heartbeat := now.UTC().Format(time.RFC3339Nano)
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
	var recorded string
	if err := s.db.QueryRow(`SELECT run_id, owner_token, heartbeat_at FROM run_leases WHERE scope = ?`, globalRunLeaseScope).Scan(&lease.RunID, &lease.OwnerToken, &recorded); err != nil {
		return RunLease{}, err
	}
	lease.HeartbeatAt, err = time.Parse(time.RFC3339Nano, recorded)
	if err != nil {
		return RunLease{}, err
	}
	return lease, ErrRunLeaseHeld
}

func (s *Store) RenewRunLease(runID, ownerToken string, now time.Time) (bool, error) {
	result, err := s.db.Exec(`UPDATE run_leases SET heartbeat_at = ? WHERE scope = ? AND run_id = ? AND owner_token = ?`, now.UTC().Format(time.RFC3339Nano), globalRunLeaseScope, runID, ownerToken)
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
