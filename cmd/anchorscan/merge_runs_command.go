package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

// runMergeRuns reassigns historical runs into a single task project with
// explicit per-run zones. It is a one-time operational command: dry-run by
// default, --apply requires a forced backup of the database and the involved
// project data directories. It never merges by name and never fabricates
// negative verifications.
func runMergeRuns(args []string, stdout, stderr io.Writer, deps cliDeps) error {
	fs := newMergeRunsFlags()
	if err := fs.parse(args); err != nil {
		return err
	}
	if fs.toProject == "" || len(fs.runAssignments) == 0 {
		printMergeRunsHelp(stdout)
		return errors.New("--to-project and at least one --run are required")
	}

	openStore := deps.openStore
	if openStore == nil {
		openStore = store.Open
	}
	scanStore, err := openStore(fs.dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer scanStore.Close()

	if _, err := scanStore.GetProject(fs.toProject); err != nil {
		return fmt.Errorf("target project %q not found: %w", fs.toProject, err)
	}
	targetZones, err := scanStore.ListProjectZones(fs.toProject)
	if err != nil {
		return err
	}
	zoneSet := map[string]struct{}{}
	for _, z := range targetZones {
		zoneSet[z.ZoneID] = struct{}{}
	}

	dataRoot := filepath.Dir(fs.dbPath)

	type plan struct {
		runID, fromProject, fromZone, toZone string
		oldReportDir, newReportDir           string
		artifactDir                          string
	}
	var plans []plan
	sourceProjects := map[string]struct{}{}
	for _, assignment := range fs.runAssignments {
		run, err := scanStore.GetScanRun(assignment.runID)
		if err != nil {
			return fmt.Errorf("run %q not found: %w", assignment.runID, err)
		}
		if run.ProjectID == fs.toProject {
			return fmt.Errorf("run %q already belongs to target project %q", assignment.runID, fs.toProject)
		}
		if _, ok := zoneSet[assignment.zoneID]; !ok {
			return fmt.Errorf("zone %q does not belong to target project %q", assignment.zoneID, fs.toProject)
		}
		oldReportDir := filepath.Join(dataRoot, "projects", run.ProjectID, "runs", assignment.runID)
		newReportDir := filepath.Join(dataRoot, "projects", fs.toProject, "runs", assignment.runID)
		if run.ProjectID != "" {
			sourceProjects[run.ProjectID] = struct{}{}
		}
		plans = append(plans, plan{
			runID:        assignment.runID,
			fromProject:  run.ProjectID,
			fromZone:     run.ZoneID,
			toZone:       assignment.zoneID,
			oldReportDir: oldReportDir,
			newReportDir: newReportDir,
			artifactDir:  run.ArtifactDir,
		})
	}

	fmt.Fprintln(stdout, "=== merge-runs preview ===")
	fmt.Fprintf(stdout, "target project: %s\n", fs.toProject)
	fmt.Fprintf(stdout, "mode: %s\n", modeLabel(fs.apply))
	for i, p := range plans {
		fmt.Fprintf(stdout, "\n[%d] run %s\n", i+1, p.runID)
		fmt.Fprintf(stdout, "    project: %s -> %s\n", p.fromProject, fs.toProject)
		fmt.Fprintf(stdout, "    zone:    %s -> %s\n", p.fromZone, p.toZone)
		fmt.Fprintf(stdout, "    include_in_report: %v\n", fs.include)
		fmt.Fprintf(stdout, "    artifact_dir:      %s (unchanged)\n", p.artifactDir)
		fmt.Fprintf(stdout, "    managed report:    %s -> %s\n", p.oldReportDir, p.newReportDir)
	}

	if !fs.apply {
		fmt.Fprintln(stdout, "\ndry-run only; pass --apply to execute (a --backup-dir is required).")
		return nil
	}

	if fs.backupDir == "" {
		return errors.New("--apply requires --backup-dir")
	}
	if err := backupMergeInputs(fs.backupDir, fs.dbPath, dataRoot, fs.toProject, sourceProjects); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	fmt.Fprintf(stdout, "\nbackup written to %s\n", fs.backupDir)

	for _, p := range plans {
		if err := scanStore.ReassignScanRun(p.runID, fs.toProject, p.toZone, fs.include); err != nil {
			return fmt.Errorf("reassign run %q: %w", p.runID, err)
		}
		if err := relocateManagedRunDir(p.oldReportDir, p.newReportDir); err != nil {
			fmt.Fprintf(stderr, "warning: could not relocate managed report dir for run %q: %v\n", p.runID, err)
		}
		fmt.Fprintf(stdout, "reassigned run %s\n", p.runID)
	}

	if fs.deleteEmptySourceProjects {
		for projectID := range sourceProjects {
			runs, err := scanStore.ListProjectScanRuns(projectID, 1)
			if err != nil {
				fmt.Fprintf(stderr, "warning: could not check source project %q: %v\n", projectID, err)
				continue
			}
			if len(runs) == 0 {
				if err := scanStore.DeleteProjectCascade(projectID); err != nil {
					fmt.Fprintf(stderr, "warning: could not delete empty source project %q: %v\n", projectID, err)
					continue
				}
				fmt.Fprintf(stdout, "deleted empty source project %s\n", projectID)
			} else {
				fmt.Fprintf(stdout, "source project %s still has runs; not deleted\n", projectID)
			}
		}
	}

	fmt.Fprintln(stdout, "\nmerge-runs complete.")
	return nil
}

func modeLabel(apply bool) string {
	if apply {
		return "apply"
	}
	return "dry-run"
}

func backupMergeInputs(backupDir, dbPath, dataRoot, toProject string, sourceProjects map[string]struct{}) error {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(dbPath, filepath.Join(backupDir, filepath.Base(dbPath))); err != nil {
		return err
	}
	for projectID := range sourceProjects {
		src := filepath.Join(dataRoot, "projects", projectID)
		if _, err := os.Stat(src); err == nil {
			if err := copyDir(src, filepath.Join(backupDir, "projects", projectID)); err != nil {
				return err
			}
		}
	}
	src := filepath.Join(dataRoot, "projects", toProject)
	if _, err := os.Stat(src); err == nil {
		if err := copyDir(src, filepath.Join(backupDir, "projects", toProject)); err != nil {
			return err
		}
	}
	return nil
}

func relocateManagedRunDir(oldDir, newDir string) error {
	if _, err := os.Stat(oldDir); err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
		return err
	}
	return os.Rename(oldDir, newDir)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

type runAssignment struct {
	runID  string
	zoneID string
}

type mergeRunsFlags struct {
	dbPath                    string
	toProject                 string
	runAssignments            []runAssignment
	include                   bool
	apply                     bool
	backupDir                 string
	deleteEmptySourceProjects bool
}

func newMergeRunsFlags() *mergeRunsFlags {
	return &mergeRunsFlags{include: true}
}

func (f *mergeRunsFlags) parse(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--db":
			i++
			if i >= len(args) {
				return errors.New("--db requires a value")
			}
			f.dbPath = args[i]
		case "--to-project":
			i++
			if i >= len(args) {
				return errors.New("--to-project requires a value")
			}
			f.toProject = args[i]
		case "--run":
			i++
			if i >= len(args) {
				return errors.New("--run requires a value runID@zoneID")
			}
			parts := strings.SplitN(args[i], "@", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("invalid --run %q, expected runID@zoneID", args[i])
			}
			f.runAssignments = append(f.runAssignments, runAssignment{runID: parts[0], zoneID: parts[1]})
		case "--no-include":
			f.include = false
		case "--apply":
			f.apply = true
		case "--backup-dir":
			i++
			if i >= len(args) {
				return errors.New("--backup-dir requires a value")
			}
			f.backupDir = args[i]
		case "--delete-empty-source-projects":
			f.deleteEmptySourceProjects = true
		case "-h", "--help":
			return errMergeRunsHelp
		default:
			return fmt.Errorf("unknown flag: %s", arg)
		}
	}
	if f.dbPath == "" {
		f.dbPath = "data/scans.sqlite"
	}
	return nil
}

var errMergeRunsHelp = errors.New("merge-runs help")

func printMergeRunsHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anchorscan merge-runs --to-project <id> --run <runID>@<zoneID> [...] [flags]

Reassign historical runs into one task project with explicit zones.
Dry-run by default; --apply requires --backup-dir and forces a backup of the
database and the involved project data directories.

Flags:
  --db <path>                        SQLite path (default data/scans.sqlite)
  --to-project <id>                  target task project (must already exist)
  --run <runID>@<zoneID>             one assignment per source run (repeatable)
  --no-include                       reassigned runs are NOT included in report
  --apply                            execute the migration (default is preview)
  --backup-dir <path>                required with --apply
  --delete-empty-source-projects     delete a source project only if it is empty

This command never merges by name and never fabricates negative verifications.`)
}
