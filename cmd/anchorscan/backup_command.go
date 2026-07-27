package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/backup"
)

func runBackup(args []string, stdout io.Writer, stderr io.Writer, deps cliDeps) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	outputDir := fs.String("output-dir", filepath.Join("data", "backups"), "backup archive output directory")
	includeArtifacts := fs.Bool("include-artifacts", false, "include run artifacts in backup")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printBackupHelp(stdout)
			return nil
		}
		return err
	}
	if err := ensureParentDir(*dbPath); err != nil {
		return err
	}
	scanStore, err := deps.openStore(*dbPath)
	if err != nil {
		return err
	}
	defer scanStore.Close()

	archivePath, err := backup.Create(backup.CreateOptions{
		Store:            scanStore,
		DataRoot:         filepath.Dir(*dbPath),
		DBPath:           *dbPath,
		ConfigDir:        filepath.Dir(*configPath),
		ArtifactRoot:     filepath.Join(filepath.Dir(*dbPath), "artifacts"),
		IncludeArtifacts: *includeArtifacts,
		OutputDir:        *outputDir,
		Now:              deps.now,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "backup=%s\n", archivePath)
	return nil
}

func runRestore(args []string, stdout io.Writer, deps cliDeps) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	archivePath := fs.String("archive", "", "path to backup archive")
	configPath := fs.String("config", filepath.Join("config", "default.yaml"), "path to config file")
	dbPath := fs.String("db", filepath.Join("data", "scans.sqlite"), "path to sqlite database")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printRestoreHelp(stdout)
			return nil
		}
		return err
	}
	if *archivePath == "" {
		return errors.New("restore requires --archive")
	}
	dataRoot := filepath.Dir(*dbPath)
	// Reconcile and check for an active run lease before replacing data.
	scanStore, err := deps.openStore(*dbPath)
	if err != nil {
		return err
	}
	if err := scanStore.ReconcileInterruptedRuns(deps.now(), 30*time.Second); err != nil {
		_ = scanStore.Close()
		return err
	}
	lease, err := scanStore.ActiveRunLease()
	if err != nil {
		_ = scanStore.Close()
		return err
	}
	if lease.RunID != "" {
		_ = scanStore.Close()
		return fmt.Errorf("restore rejected: active run lease held by %s", lease.RunID)
	}
	if err := scanStore.Close(); err != nil {
		return err
	}

	if err := backup.Restore(*archivePath, backup.RestoreOptions{
		DataRoot:     dataRoot,
		DBPath:       *dbPath,
		ConfigDir:    filepath.Dir(*configPath),
		ArtifactRoot: filepath.Join(dataRoot, "artifacts"),
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "restored=%s\ndb=%s\n", *archivePath, *dbPath)
	return nil
}

func printBackupHelp(w io.Writer) {
	_, _ = fmt.Fprintf(w, "Usage: anchorscan backup [flags]\n\nCreate a tar.gz backup of the SQLite database, project evidence and required config.\nThe command fails if a scan run is currently active.\n\nFlags:\n  --config <path>        Config file path (default: config/default.yaml)\n  --db <path>            SQLite database path (default: data/scans.sqlite)\n  --output-dir <path>    Backup archive output directory (default: data/backups)\n  --include-artifacts    Also include run artifacts (default: false)\n")
}

func printRestoreHelp(w io.Writer) {
	_, _ = fmt.Fprintf(w, "Usage: anchorscan restore --archive <path> [flags]\n\nVerify and restore a backup archive. The current SQLite database, config and project evidence are replaced.\nThe command fails if a scan run is currently active.\n\nFlags:\n  --archive <path>    Backup archive to restore (required)\n  --config <path>     Config file path (default: config/default.yaml)\n  --db <path>         SQLite database path (default: data/scans.sqlite)\n")
}
