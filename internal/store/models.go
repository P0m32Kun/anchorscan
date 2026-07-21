package store

import "time"

type Project struct {
	ID             string
	Name           string
	Description    string
	ClientUnit     string
	ReportTitle    string
	TestObject     string
	StartDate      string
	EndDate        string
	Testers        string
	DefaultTargets string
	DefaultPorts   string
	ExcludeTargets string
	ExcludePorts   string
	DefaultProfile string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProjectZone struct {
	ProjectID string
	ZoneID    string
	Name      string
	SortOrder int
}

type ScanRun struct {
	RunID          string
	ProjectID      string
	ZoneID         string
	Target         string
	Ports          string
	Profile        string
	Status         string
	StartedAt      time.Time
	FinishedAt     time.Time
	Error          string
	ConfigSnapshot string
	ArtifactDir    string
}

type ScanEvent struct {
	ID      int64     `json:"id"`
	RunID   string    `json:"run_id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Stage   string    `json:"stage"`
	Message string    `json:"message"`
}

type DetectionCheck struct {
	RunID      string
	IP         string
	Port       int
	Protocol   string
	Engine     string
	Status     string
	ReasonCode string
	Detail     string
	StartedAt  time.Time
	FinishedAt time.Time
}
