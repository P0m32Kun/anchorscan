package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func TestBackupRoundTripRestoresDatabaseAndProjects(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "scans.sqlite")
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "nse.yaml"), []byte("nse: rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "service-tags.yaml"), []byte("tags: rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Lab", DefaultTargets: "127.0.0.1", DefaultPorts: "80", DefaultProfile: "normal", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	archive, err := Create(CreateOptions{
		Store:     s,
		DataRoot:  filepath.Dir(dbPath),
		DBPath:    dbPath,
		ConfigDir: configDir,
		OutputDir: filepath.Join(dir, "backups"),
		Now:       func() time.Time { return time.Unix(1, 0) },
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	restoreDir := filepath.Join(dir, "restore")
	restoreDB := filepath.Join(restoreDir, "data", "scans.sqlite")
	restoreConfig := filepath.Join(restoreDir, "config")
	if err := Restore(archive, RestoreOptions{
		DataRoot:  filepath.Dir(restoreDB),
		DBPath:    restoreDB,
		ConfigDir: restoreConfig,
	}); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	rs, err := store.Open(restoreDB)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	project, err := rs.GetProject("p1")
	if err != nil {
		t.Fatalf("GetProject returned error: %v", err)
	}
	if project.Name != "Lab" {
		t.Fatalf("project name = %q, want Lab", project.Name)
	}
	if _, err := os.Stat(filepath.Join(restoreConfig, "nse.yaml")); err != nil {
		t.Fatalf("config not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(restoreConfig, "service-tags.yaml")); err != nil {
		t.Fatalf("config not restored: %v", err)
	}
}

func TestBackupRejectsActiveRunLease(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "scans.sqlite")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.AcquireRunLease("run-1", "owner-1", time.Now(), time.Minute); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(dir, "config")
	_ = os.MkdirAll(configDir, 0o755)
	_, err = Create(CreateOptions{
		Store:     s,
		DataRoot:  filepath.Dir(dbPath),
		DBPath:    dbPath,
		ConfigDir: configDir,
		OutputDir: filepath.Join(dir, "backups"),
		Now:       func() time.Time { return time.Unix(1, 0) },
	})
	if err == nil {
		t.Fatal("expected backup to reject active run lease")
	}
}

func TestBackupRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "scans.sqlite")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "real.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(configDir, "real.yaml"), filepath.Join(configDir, "link.yaml")); err != nil {
		t.Fatal(err)
	}

	_, err = Create(CreateOptions{
		Store:     s,
		DataRoot:  filepath.Dir(dbPath),
		DBPath:    dbPath,
		ConfigDir: configDir,
		OutputDir: filepath.Join(dir, "backups"),
		Now:       func() time.Time { return time.Unix(1, 0) },
	})
	if err == nil {
		t.Fatal("expected backup to reject symlink in config dir")
	}
}

func TestRestoreRejectsArchivePathEscape(t *testing.T) {
	dir := t.TempDir()
	malicious := filepath.Join(dir, "evil.tar.gz")
	if err := os.MkdirAll(filepath.Dir(malicious), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeTarGzWithEntry(malicious, "../escape.txt", []byte("bad")); err != nil {
		t.Fatal(err)
	}
	restoreDir := filepath.Join(dir, "restore")
	if err := os.MkdirAll(filepath.Join(restoreDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Restore(malicious, RestoreOptions{
		DataRoot:  filepath.Join(restoreDir, "data"),
		DBPath:    filepath.Join(restoreDir, "data", "scans.sqlite"),
		ConfigDir: filepath.Join(restoreDir, "config"),
	}); err == nil {
		t.Fatal("expected restore to reject path-escaping archive entry")
	}
}

func writeTarGzWithEntry(path, name string, data []byte) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	hdr := &tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	return nil
}

func TestBackupRestoresEvidence(t *testing.T) {
	// Generate a minimal PNG image for evidence.
	var imgBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if err := png.Encode(&imgBuf, img); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "scans.sqlite")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatal(err)
	}
	zones, err := s.ListProjectZones("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) == 0 {
		t.Fatal("expected zones")
	}
	zoneID := zones[0].ZoneID
	v := store.Verification{
		ID: "v1", ProjectID: "p1", ZoneID: zoneID, VulnerabilityKey: "k1", Outcome: "confirmed",
		Title: "x", Severity: "high", Description: "d", Remediation: "r", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateEvidence("p1", store.CreateEvidenceInput{VerificationID: "v1", Data: imgBuf.Bytes(), Caption: "c", Position: 1})
	if err != nil {
		t.Fatalf("CreateEvidence returned error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.yaml"), []byte("tools:\n  nmap: /bin/nmap\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := Create(CreateOptions{
		Store:     s,
		DataRoot:  filepath.Dir(dbPath),
		DBPath:    dbPath,
		ConfigDir: configDir,
		OutputDir: filepath.Join(dir, "backups"),
		Now:       func() time.Time { return time.Unix(1, 0) },
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	restoreDir := filepath.Join(dir, "restore")
	restoreDB := filepath.Join(restoreDir, "data", "scans.sqlite")
	restoreConfig := filepath.Join(restoreDir, "config")
	if err := Restore(archive, RestoreOptions{
		DataRoot:  filepath.Dir(restoreDB),
		DBPath:    restoreDB,
		ConfigDir: restoreConfig,
	}); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	rs, err := store.Open(restoreDB)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	evidence, err := rs.ListVerificationEvidence("v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(evidence))
	}
	if _, err := os.Stat(rs.EvidenceFilePath(evidence[0], "p1")); err != nil {
		t.Fatalf("evidence file not restored: %v", err)
	}
}

func TestValidateManifestRejectsHashMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("wrong"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		Version: backupVersion,
		Entries: []ManifestEntry{{RelPath: "data.txt", Size: 5, SHA256: "0000000000000000000000000000000000000000000000000000000000000000"}},
	}
	if err := validateManifest(dir, manifest); err == nil {
		t.Fatal("expected hash mismatch error")
	}
}

func TestValidateManifestRejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		Version: backupVersion,
		Entries: []ManifestEntry{{RelPath: "missing.txt", Size: 1, SHA256: "0000000000000000000000000000000000000000000000000000000000000000"}},
	}
	if err := validateManifest(dir, manifest); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestValidateNoExtraFilesRejectsInjectedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.txt"), []byte("good"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evil.txt"), []byte("evil"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		Version: backupVersion,
		Entries: []ManifestEntry{{RelPath: "good.txt", Size: 4, SHA256: hashOf("good")}},
	}
	if err := validateManifest(dir, manifest); err != nil {
		t.Fatal(err)
	}
	if err := validateNoExtraFiles(dir, manifest); err == nil {
		t.Fatal("expected extra file error")
	}
}

func TestBackupIncludesArtifactsWhenRequested(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "scans.sqlite")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(store.Project{ID: "p1", Name: "Lab", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifactRoot := filepath.Join(dir, "data", "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "run-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "run-1", "report.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err = store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := Create(CreateOptions{
		Store:            s,
		DataRoot:         filepath.Dir(dbPath),
		DBPath:           dbPath,
		ConfigDir:        configDir,
		ArtifactRoot:     artifactRoot,
		IncludeArtifacts: true,
		OutputDir:        filepath.Join(dir, "backups"),
		Now:              func() time.Time { return time.Unix(1, 0) },
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	restoreDir := filepath.Join(dir, "restore")
	restoreDB := filepath.Join(restoreDir, "data", "scans.sqlite")
	if err := Restore(archive, RestoreOptions{
		DataRoot:     filepath.Dir(restoreDB),
		DBPath:       restoreDB,
		ConfigDir:    filepath.Join(restoreDir, "config"),
		ArtifactRoot: filepath.Join(restoreDir, "data", "artifacts"),
	}); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "data", "artifacts", "run-1", "report.json")); err != nil {
		t.Fatalf("artifact not restored: %v", err)
	}
}

func hashOf(s string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
