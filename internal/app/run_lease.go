package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

const (
	runLeaseHeartbeat = 5 * time.Second
	runLeaseTTL       = 30 * time.Second
)

func newRunLeaseToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func reserveRunLease(scanStore *store.Store, runID string) (string, error) {
	if scanStore == nil || runID == "" {
		return "", nil
	}
	token, err := newRunLeaseToken()
	if err != nil {
		return "", err
	}
	lease, err := scanStore.AcquireRunLease(runID, token, time.Now(), runLeaseTTL)
	if errors.Is(err, store.ErrRunLeaseHeld) {
		return "", fmt.Errorf("scan already running: %s", lease.RunID)
	}
	return token, err
}

func acquireRunLease(ctx context.Context, scanStore *store.Store, runID, ownerToken string) (context.Context, func(string, string, time.Time), func(), error) {
	if scanStore == nil || runID == "" {
		return ctx, func(string, string, time.Time) {}, func() {}, nil
	}
	if ownerToken == "" {
		var err error
		ownerToken, err = newRunLeaseToken()
		if err != nil {
			return nil, nil, nil, err
		}
		lease, err := scanStore.AcquireRunLease(runID, ownerToken, time.Now(), runLeaseTTL)
		if err != nil {
			if errors.Is(err, store.ErrRunLeaseHeld) {
				return nil, nil, nil, fmt.Errorf("scan already running: %s", lease.RunID)
			}
			return nil, nil, nil, err
		}
	} else {
		owned, err := scanStore.RenewRunLease(runID, ownerToken, time.Now())
		if err != nil {
			return nil, nil, nil, err
		}
		if !owned {
			return nil, nil, nil, errors.New("run lease was lost")
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(runLeaseHeartbeat)
		defer ticker.Stop()
		lastSuccess := time.Now()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				owned, renewErr := scanStore.RenewRunLease(runID, ownerToken, now)
				if owned {
					lastSuccess = now
				}
				if !owned && (renewErr == nil || now.Sub(lastSuccess) >= runLeaseTTL) {
					cancel()
					return
				}
			}
		}
	}()

	stopHeartbeat := func() {
		close(stop)
		<-done
		cancel()
	}
	return runCtx, func(status, message string, finishedAt time.Time) {
			stopHeartbeat()
			_, _ = scanStore.FinishRunWithLease(runID, ownerToken, status, message, finishedAt)
		}, func() {
			stopHeartbeat()
			_, _ = scanStore.ReleaseRunLease(runID, ownerToken)
		}, nil
}
