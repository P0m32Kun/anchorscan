package store

import "time"

type Project struct {
	ID             string
	Name           string
	Description    string
	DefaultTargets string
	DefaultPorts   string
	ExcludeTargets string
	ExcludePorts   string
	DefaultProfile string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ScanRun struct {
	RunID          string
	ProjectID      string
	Target         string
	Ports          string
	Profile        string
	Status         string
	StartedAt      time.Time
	FinishedAt     time.Time
	Error          string
	ConfigSnapshot string
}

type ScanEvent struct {
	ID      int64     `json:"id"`
	RunID   string    `json:"run_id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
}
