package app

import (
	"context"
	"errors"
	"sync"

	"github.com/P0m32Kun/anchorscan/internal/store"
	"github.com/P0m32Kun/anchorscan/internal/tools"
)

type Manager struct {
	mu       sync.Mutex
	runner   tools.Runner
	store    *store.Store
	activeID string
	cancel   context.CancelFunc
}

func NewManager(runner tools.Runner, scanStore *store.Store) *Manager {
	return &Manager{runner: runner, store: scanStore}
}

func (m *Manager) Start(ctx context.Context, opts ScanOptions) (string, error) {
	m.mu.Lock()
	if m.activeID != "" {
		m.mu.Unlock()
		return "", errors.New("scan already running")
	}
	ownerToken, err := reserveRunLease(m.store, opts.RunID)
	if err != nil {
		m.mu.Unlock()
		return "", err
	}
	runCtx, cancel := context.WithCancel(ctx)
	opts.LeaseOwnerToken = ownerToken
	m.activeID = opts.RunID
	m.cancel = cancel
	m.mu.Unlock()

	go func() {
		_ = RunScan(runCtx, m.runner, m.store, opts)
		m.mu.Lock()
		m.activeID = ""
		m.cancel = nil
		m.mu.Unlock()
	}()
	return opts.RunID, nil
}

func (m *Manager) StartTool(ctx context.Context, opts ToolRunOptions) (string, error) {
	m.mu.Lock()
	if m.activeID != "" {
		m.mu.Unlock()
		return "", errors.New("scan already running")
	}
	ownerToken, err := reserveRunLease(m.store, opts.RunID)
	if err != nil {
		m.mu.Unlock()
		return "", err
	}
	runCtx, cancel := context.WithCancel(ctx)
	opts.LeaseOwnerToken = ownerToken
	m.activeID = opts.RunID
	m.cancel = cancel
	m.mu.Unlock()

	go func() {
		_ = RunTool(runCtx, m.runner, m.store, opts)
		m.mu.Lock()
		m.activeID = ""
		m.cancel = nil
		m.mu.Unlock()
	}()
	return opts.RunID, nil
}

func (m *Manager) Cancel(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeID != runID || m.cancel == nil {
		return errors.New("scan is not running")
	}
	m.cancel()
	return nil
}

func (m *Manager) ActiveRunID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeID
}
