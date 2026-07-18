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

func acquireRunLease(ctx context.Context, scanStore *store.Store, runID string) (context.Context, func(), error) {
	if scanStore == nil || runID == "" {
		return ctx, func() {}, nil
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, nil, err
	}
	ownerToken := hex.EncodeToString(bytes)
	lease, err := scanStore.AcquireRunLease(runID, ownerToken, time.Now(), runLeaseTTL)
	if err != nil {
		if errors.Is(err, store.ErrRunLeaseHeld) {
			return nil, nil, fmt.Errorf("scan already running: %s", lease.RunID)
		}
		return nil, nil, err
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

	return runCtx, func() {
		close(stop)
		<-done
		cancel()
		_, _ = scanStore.ReleaseRunLease(runID, ownerToken)
	}, nil
}
