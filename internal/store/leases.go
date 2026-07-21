package store

import (
	"database/sql"
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
	result, err := s.db.Exec(`INSERT INTO run_leases (scope, run_id, owner_token, heartbeat_at, heartbeat_at_ns) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(scope) DO UPDATE SET run_id = excluded.run_id, owner_token = excluded.owner_token, heartbeat_at = excluded.heartbeat_at, heartbeat_at_ns = excluded.heartbeat_at_ns
		WHERE run_leases.heartbeat_at_ns < ? AND julianday(run_leases.heartbeat_at) < julianday(?)`, globalRunLeaseScope, runID, ownerToken, time.Unix(0, heartbeat).UTC().Format(time.RFC3339Nano), heartbeat, cutoff, time.Unix(0, cutoff).UTC().Format(time.RFC3339Nano))
	if err != nil {
		return RunLease{}, err
	}
	if changed, _ := result.RowsAffected(); changed != 0 {
		return RunLease{RunID: runID, OwnerToken: ownerToken, HeartbeatAt: now.UTC()}, nil
	}
	var lease RunLease
	var recorded int64
	if err := s.db.QueryRow(`SELECT run_id, owner_token, heartbeat_at_ns FROM run_leases WHERE scope = ?`, globalRunLeaseScope).Scan(&lease.RunID, &lease.OwnerToken, &recorded); err != nil {
		return RunLease{}, err
	}
	lease.HeartbeatAt = time.Unix(0, recorded).UTC()
	return lease, ErrRunLeaseHeld
}

func (s *Store) RenewRunLease(runID, ownerToken string, now time.Time) (bool, error) {
	result, err := s.db.Exec(`UPDATE run_leases SET heartbeat_at = ?, heartbeat_at_ns = ? WHERE scope = ? AND run_id = ? AND owner_token = ?`, now.UTC().Format(time.RFC3339Nano), now.UnixNano(), globalRunLeaseScope, runID, ownerToken)
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
	result, err := tx.Exec(`UPDATE scan_runs SET status = ?, error = ?, finished_at = ?,
		include_in_report = CASE WHEN kind = 'scan' AND ? IN ('completed', 'completed_with_errors') THEN 1 ELSE include_in_report END
		WHERE run_id = ? AND EXISTS (SELECT 1 FROM run_leases WHERE scope = ? AND run_id = ? AND owner_token = ?)`,
		status, message, formatTime(finishedAt), status, runID, globalRunLeaseScope, runID, ownerToken)
	if err != nil {
		return false, err
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return false, err
	}
	if status == "canceled" {
		if err := cancelDetectionChecks(tx, runID, finishedAt); err != nil {
			return false, err
		}
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

// ReconcileInterruptedRuns closes runs whose owner can no longer renew its lease.
// It is safe to call on startup and before a new lease acquisition.
func (s *Store) ReconcileInterruptedRuns(now time.Time, ttl time.Duration) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var runID, heartbeat string
	var heartbeatNS int64
	err = tx.QueryRow(`SELECT run_id, heartbeat_at, heartbeat_at_ns FROM run_leases WHERE scope = ?`, globalRunLeaseScope).Scan(&runID, &heartbeat, &heartbeatNS)
	if err == nil && !runLeaseFresh(heartbeat, heartbeatNS, now, ttl) {
		result, err := tx.Exec(`DELETE FROM run_leases WHERE scope = ? AND run_id = ? AND heartbeat_at_ns < ? AND julianday(heartbeat_at) < julianday(?)`, globalRunLeaseScope, runID, now.Add(-ttl).UnixNano(), now.Add(-ttl).UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		if deleted, _ := result.RowsAffected(); deleted == 1 {
			updated, err := tx.Exec(`UPDATE scan_runs SET status = 'interrupted', error = 'run lease expired', finished_at = ? WHERE run_id = ? AND status = 'running'`, formatTime(now), runID)
			if err != nil {
				return err
			}
			if changed, _ := updated.RowsAffected(); changed == 1 {
				if err := interruptDetectionChecks(tx, runID, now); err != nil {
					return err
				}
			}
		}
	} else if err != nil && err != sql.ErrNoRows {
		return err
	}

	_, err = tx.Exec(`UPDATE scan_runs SET status = 'interrupted', error = 'run lease missing', finished_at = ?
		WHERE status = 'running' AND NOT EXISTS (SELECT 1 FROM run_leases WHERE scope = ? AND run_id = scan_runs.run_id)`, formatTime(now), globalRunLeaseScope)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE detection_checks SET status = 'interrupted', reason_code = 'lease_expired', detail = 'run lease missing', finished_at = ?
		WHERE status = 'running' AND run_id IN (
			SELECT run_id FROM scan_runs WHERE status = 'interrupted' AND error = 'run lease missing' AND finished_at = ?
		)`, formatTime(now), formatTime(now)); err != nil {
		return err
	}
	return tx.Commit()
}

func interruptDetectionChecks(tx *sql.Tx, runID string, now time.Time) error {
	_, err := tx.Exec(`UPDATE detection_checks SET status = 'interrupted', reason_code = 'lease_expired', detail = 'run lease expired', finished_at = ? WHERE run_id = ? AND status = 'running'`, formatTime(now), runID)
	return err
}

func cancelDetectionChecks(tx *sql.Tx, runID string, now time.Time) error {
	_, err := tx.Exec(`UPDATE detection_checks SET status = 'canceled', reason_code = 'run_canceled', detail = 'run canceled', finished_at = ? WHERE run_id = ? AND status = 'running'`, formatTime(now), runID)
	return err
}

func runLeaseFresh(heartbeat string, heartbeatNS int64, now time.Time, ttl time.Duration) bool {
	cutoff := now.Add(-ttl)
	if heartbeatNS >= cutoff.UnixNano() {
		return true
	}
	at, err := time.Parse(time.RFC3339Nano, heartbeat)
	return err != nil || !at.Before(cutoff)
}
