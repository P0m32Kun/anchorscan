package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

const (
	backupVersion = "v1"
	leaseTTL      = 30 * time.Second
)

// Manifest describes the contents of a backup archive.
type Manifest struct {
	Version          string          `json:"version"`
	CreatedAt        string          `json:"created_at"`
	DBPath           string          `json:"db_path"`
	ConfigDir        string          `json:"config_dir"`
	IncludeArtifacts bool            `json:"include_artifacts"`
	Entries          []ManifestEntry `json:"entries"`
}

// ManifestEntry records a single file in the archive.
type ManifestEntry struct {
	RelPath string `json:"rel_path"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
}

// CreateOptions configures a backup creation.
type CreateOptions struct {
	Store            *store.Store
	DataRoot         string
	DBPath           string
	ConfigDir        string
	ArtifactRoot     string
	IncludeArtifacts bool
	OutputDir        string
	Now              func() time.Time
}

// Create produces a tar.gz backup archive and returns its path.
// It fails if an active run lease exists, to avoid inconsistent snapshots.
func Create(opts CreateOptions) (string, error) {
	if opts.Store == nil {
		return "", errors.New("backup requires a store")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if err := opts.Store.ReconcileInterruptedRuns(now(), leaseTTL); err != nil {
		return "", fmt.Errorf("reconcile leases: %w", err)
	}
	lease, err := opts.Store.ActiveRunLease()
	if err != nil {
		return "", fmt.Errorf("check run lease: %w", err)
	}
	if lease.RunID != "" {
		return "", fmt.Errorf("backup rejected: active run lease held by %s", lease.RunID)
	}

	if opts.DataRoot == "" {
		return "", errors.New("backup requires a data root")
	}
	if opts.ConfigDir == "" {
		return "", errors.New("backup requires a config dir")
	}
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join(opts.DataRoot, "backups")
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("create backup output dir: %w", err)
	}

	staging, err := os.MkdirTemp("", "anchorscan-backup-staging-*")
	if err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	// 1. Consistent SQLite snapshot via VACUUM INTO.
	dbSnapshot := filepath.Join(staging, "db.sqlite")
	if err := opts.Store.VacuumInto(dbSnapshot); err != nil {
		return "", fmt.Errorf("vacuum database: %w", err)
	}

	// 2. Copy runtime config.
	if err := copyDirectory(opts.ConfigDir, filepath.Join(staging, "config"), false); err != nil {
		return "", fmt.Errorf("copy config: %w", err)
	}

	// 3. Copy project evidence.
	projectsDst := filepath.Join(staging, "projects")
	projectsSrc := filepath.Join(opts.DataRoot, "projects")
	if _, statErr := os.Stat(projectsSrc); statErr == nil {
		if err := copyDirectory(projectsSrc, projectsDst, false); err != nil {
			return "", fmt.Errorf("copy projects: %w", err)
		}
	} else if os.IsNotExist(statErr) {
		_ = os.MkdirAll(projectsDst, 0o755)
	}

	// 4. Optionally copy artifacts.
	artifactSrc := opts.ArtifactRoot
	if artifactSrc == "" {
		artifactSrc = filepath.Join(opts.DataRoot, "artifacts")
	}
	if opts.IncludeArtifacts {
		if _, statErr := os.Stat(artifactSrc); statErr == nil {
			if err := copyDirectory(artifactSrc, filepath.Join(staging, "artifacts"), false); err != nil {
				return "", fmt.Errorf("copy artifacts: %w", err)
			}
		}
	}

	// 5. Build manifest with hashes.
	manifest, err := buildManifest(staging)
	if err != nil {
		return "", fmt.Errorf("build manifest: %w", err)
	}
	manifest.Version = backupVersion
	manifest.CreatedAt = now().UTC().Format(time.RFC3339Nano)
	manifest.DBPath = opts.DBPath
	manifest.ConfigDir = opts.ConfigDir
	manifest.IncludeArtifacts = opts.IncludeArtifacts
	if err := writeManifest(staging, manifest); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	archiveName := fmt.Sprintf("anchorscan-backup-%s.tar.gz", now().Format("20060102-150405"))
	archivePath := filepath.Join(opts.OutputDir, archiveName)
	if err := tarGz(staging, archivePath); err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}
	return archivePath, nil
}

// RestoreOptions configures a restore.
type RestoreOptions struct {
	DataRoot     string
	DBPath       string
	ConfigDir    string
	ArtifactRoot string
}

// Restore extracts a backup archive, validates its manifest, and replaces the
// target data root contents. It must be called with the target database closed.
func Restore(archivePath string, opts RestoreOptions) error {
	if opts.DataRoot == "" {
		return errors.New("restore requires a data root")
	}
	if opts.DBPath == "" {
		return errors.New("restore requires a db path")
	}
	if opts.ConfigDir == "" {
		return errors.New("restore requires a config dir")
	}
	if opts.ArtifactRoot == "" {
		opts.ArtifactRoot = filepath.Join(opts.DataRoot, "artifacts")
	}

	extractDir, err := os.MkdirTemp("", "anchorscan-restore-*")
	if err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err := extractTarGz(archivePath, extractDir); err != nil {
		return fmt.Errorf("extract archive: %w", err)
	}

	manifest, err := readManifest(extractDir)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if manifest.Version != backupVersion {
		return fmt.Errorf("unsupported backup version: %s", manifest.Version)
	}
	if err := validateManifest(extractDir, manifest); err != nil {
		return fmt.Errorf("validate manifest: %w", err)
	}

	// Replace target directories. Fail fast if any replacement errors.
	replacements := []struct {
		src string
		dst string
	}{
		{filepath.Join(extractDir, "db.sqlite"), opts.DBPath},
		{filepath.Join(extractDir, "config"), opts.ConfigDir},
		{filepath.Join(extractDir, "projects"), filepath.Join(opts.DataRoot, "projects")},
	}
	if manifest.IncludeArtifacts {
		replacements = append(replacements, struct{ src, dst string }{filepath.Join(extractDir, "artifacts"), opts.ArtifactRoot})
	}

	// Pre-validate sources exist before mutating targets.
	for _, r := range replacements {
		if _, err := os.Stat(r.src); err != nil {
			return fmt.Errorf("backup source missing: %s: %w", r.src, err)
		}
	}

	for _, r := range replacements {
		if err := replaceDirectory(r.src, r.dst); err != nil {
			return fmt.Errorf("replace %s: %w", r.dst, err)
		}
	}
	return nil
}

func buildManifest(root string) (Manifest, error) {
	manifest := Manifest{Entries: []ManifestEntry{}}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		h, err := hashFile(path)
		if err != nil {
			return err
		}
		manifest.Entries = append(manifest.Entries, ManifestEntry{
			RelPath: rel,
			Size:    info.Size(),
			SHA256:  h,
		})
		return nil
	})
	return manifest, err
}

func writeManifest(root string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "manifest.json"), data, 0o644)
}

func readManifest(root string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func validateManifest(root string, manifest Manifest) error {
	for _, entry := range manifest.Entries {
		path := filepath.Join(root, entry.RelPath)
		if !isWithin(root, path) {
			return fmt.Errorf("manifest entry escapes archive: %s", entry.RelPath)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("missing file %s: %w", entry.RelPath, err)
		}
		if info.Size() != entry.Size {
			return fmt.Errorf("size mismatch for %s: got %d, want %d", entry.RelPath, info.Size(), entry.Size)
		}
		h, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("hash file %s: %w", entry.RelPath, err)
		}
		if h != entry.SHA256 {
			return fmt.Errorf("hash mismatch for %s", entry.RelPath)
		}
	}
	return nil
}

func copyDirectory(src, dst string, allowSymlinks bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if !allowSymlinks {
				return fmt.Errorf("symlink not allowed: %s", path)
			}
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
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
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func replaceDirectory(src, dst string) error {
	// Remove existing target and rename source into place.
	if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	return nil
}

func tarGz(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
	return err
}

func extractTarGz(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	gr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return fmt.Errorf("symlink not allowed in archive: %s", hdr.Name)
		}
		path := filepath.Join(dst, hdr.Name)
		if !isWithin(dst, path) {
			return fmt.Errorf("archive entry escapes target dir: %s", hdr.Name)
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(path, hdr.FileInfo().Mode().Perm()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.Create(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func isWithin(base, target string) bool {
	ok, err := isWithinCheck(base, target)
	return err == nil && ok
}

func isWithinCheck(base, target string) (bool, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false, err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false, err
	}
	return !strings.HasPrefix(rel, ".."), nil
}
