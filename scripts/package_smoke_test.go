//go:build packageintegration

package scripts

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/knowledgebase"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/web"
)

// Single-source design (2026-08-05 rework): the release archive must NOT
// ship a catalog copy. The knowledge base is configured by pointing
// knowledge_base.path at a clone of the knowledge-base repo (Pentest-Playbook,
// handbook-v3/dist/catalog.json); the frozen producer artifact checksum lives
// in internal/knowledgebase/testdata/README.md and is locked by
// internal/knowledgebase/catalog_drift_test.go (fixture only, never packaged).

func TestPackageArchiveIncludesRuntimeResources(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(testFileDir(t), ".."))
	version := "v9.8.7-package-test"
	packageName := os.Getenv("ANCHORSCAN_PACKAGE_NAME")
	archivePath := os.Getenv("ANCHORSCAN_PACKAGE_ARCHIVE")
	if packagedVersion := os.Getenv("ANCHORSCAN_PACKAGE_VERSION"); packagedVersion != "" {
		version = packagedVersion
	}
	if packageName == "" || archivePath == "" {
		distDir := t.TempDir()
		packageName = "anchorscan-" + version + "-" + runtime.GOOS + "-" + runtime.GOARCH
		archivePath = filepath.Join(distDir, packageName+".tar.gz")
		cmd := exec.Command("make", "package",
			"DIST_DIR="+distDir,
			"VERSION="+version,
			"GOOS="+runtime.GOOS,
			"GOARCH="+runtime.GOARCH,
		)
		cmd.Dir = repoRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("make package returned error: %v\n%s", err, output)
		}
	}

	extractDir := t.TempDir()
	extractTarGz(t, archivePath, extractDir)
	packageDir := filepath.Join(extractDir, packageName)
	binaryName := "anchorscan"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	assertBinaryVersion(t, filepath.Join(packageDir, binaryName), strings.TrimPrefix(version, "v"))
	assertBinaryStarts(t, filepath.Join(packageDir, binaryName))

	for _, relativePath := range []string{
		"config/default.yaml.example",
		"config/nse.yaml",
		"config/service-tags.yaml",
		"config/ports-highrisk.txt",
		"config/ports-top1000.txt",
		"docs/README.md",
		"docs/deploy.md",
		"tools/docx-render/.python-version",
		"tools/docx-render/pyproject.toml",
		"tools/docx-render/uv.lock",
		"tools/docx-render/render_docx.py",
		"tools/docx-render/templates/project-report.docx",
	} {
		if _, err := os.Stat(filepath.Join(packageDir, filepath.FromSlash(relativePath))); err != nil {
			t.Errorf("packaged runtime resource %q: %v", relativePath, err)
		}
	}
	for _, relativePath := range []string{"config/default.yaml", "config/nuclei-templates", "nuclei-templates", "data", "reports"} {
		if _, err := os.Stat(filepath.Join(packageDir, filepath.FromSlash(relativePath))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("package must not include local path %q: %v", relativePath, err)
		}
	}

	configPath := filepath.Join(packageDir, "config", "default.yaml.example")
	nseRules, err := config.LoadNSERulesForConfig(configPath)
	if err != nil {
		t.Fatalf("LoadNSERulesForConfig returned error: %v", err)
	}
	if len(nseRules) == 0 {
		t.Fatal("packaged NSE rules are empty")
	}
	tagRules, err := config.LoadTagRulesForConfig(configPath)
	if err != nil {
		t.Fatalf("LoadTagRulesForConfig returned error: %v", err)
	}
	if len(tagRules) == 0 {
		t.Fatal("packaged service tag rules are empty")
	}
	for _, preset := range []string{"highrisk", "top1000"} {
		value, err := ports.LoadPresetForConfig(preset, configPath)
		if err != nil {
			t.Fatalf("LoadPresetForConfig(%q) returned error: %v", preset, err)
		}
		if value == "" {
			t.Fatalf("packaged %s preset is empty", preset)
		}
	}

	assertArchiveIsClean(t, archivePath, packageDir)
	assertDefaultConfigDisablesKnowledgeBase(t, packageDir)
}

// assertArchiveIsClean verifies the release archive contains neither a
// knowledge-base catalog copy nor any forbidden runtime/state/customer
// artifact. It scans the tar member listing (see package_clean.go for the
// shared forbidden patterns) so entries that never extract are still caught.
func assertArchiveIsClean(t *testing.T, archivePath, packageDir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(packageDir, "config", "catalog.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("package must not ship config/catalog.json (single-source design): %v", err)
	}
	for _, hit := range findForbiddenMembers(archiveMembers(t, archivePath)) {
		t.Fatalf("archive must not contain forbidden member matching %q: found %q", hit.pattern, hit.member)
	}
}

// assertDefaultConfigDisablesKnowledgeBase boots a web server exactly as a
// fresh install would (default example config with an empty knowledge_base.path)
// and asserts the knowledge base reports disabled with a clear diagnostic and
// zero entries.
func assertDefaultConfigDisablesKnowledgeBase(t *testing.T, packageDir string) {
	t.Helper()
	configPath := filepath.Join(packageDir, "config", "default.yaml.example")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load packaged default config: %v", err)
	}
	if strings.TrimSpace(cfg.KnowledgeBase.Path) != "" {
		t.Fatalf("packaged default knowledge_base.path = %q, want %q (empty = disabled, single-source design)", cfg.KnowledgeBase.Path, "")
	}
	catalog := knowledgebase.Load(configPath, cfg.KnowledgeBase.Path)
	if catalog.Status() != knowledgebase.StatusDisabled || len(catalog.Search("")) != 0 {
		t.Fatalf("packaged default catalog status=%q entries=%d", catalog.Status(), len(catalog.Search("")))
	}
	if len(catalog.Diagnostics()) == 0 {
		t.Fatal("disabled knowledge base must carry a clear diagnostic")
	}

	handler, err := web.NewServer(web.ServerOptions{ConfigPath: configPath, DBPath: filepath.Join(t.TempDir(), "scan.db")})
	if err != nil {
		t.Fatalf("boot web server with packaged defaults: %v", err)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/kb", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET /kb with packaged defaults: status %d", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "knowledgebase-status\">disabled") || !strings.Contains(body, "未配置 knowledge_base.path") || strings.Contains(body, "knowledgebase-entry") {
		t.Fatalf("GET /kb with packaged defaults did not report disabled catalog with a clear diagnostic: %s", body)
	}
}

func TestBuildVersionCanBeInjected(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(testFileDir(t), ".."))
	injectedVersion := "v9.8.7-linker-test"
	// go build -o uses the exact name given; on Windows the binary must carry
	// the .exe extension to be executable.
	binaryName := "anchorscan"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), binaryName)

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-X github.com/P0m32Kun/anchorscan/internal/version.Version="+injectedVersion, "-o", binary, "./cmd/anchorscan")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build injected version: %v\n%s", err, output)
	}
	assertBinaryVersion(t, binary, injectedVersion)

	devBinary := filepath.Join(t.TempDir(), binaryName)
	cmd = exec.Command("go", "build", "-trimpath", "-o", devBinary, "./cmd/anchorscan")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build development version: %v\n%s", err, output)
	}
	assertBinaryVersion(t, devBinary, "dev")
}

func assertBinaryStarts(t *testing.T, binary string) {
	t.Helper()
	if output, err := exec.Command(binary, "--help").CombinedOutput(); err != nil {
		t.Fatalf("start %s: %v\n%s", binary, err, output)
	}
}

func assertBinaryVersion(t *testing.T, binary, version string) {
	t.Helper()
	output, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("run %s --version: %v\n%s", binary, err, output)
	}
	if got, want := strings.TrimSpace(string(output)), "anchorscan version "+version; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func testFileDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate package test")
	}
	return filepath.Dir(file)
}

func extractTarGz(t *testing.T, archivePath, destination string) {
	t.Helper()
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open gzip stream: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("read tar entry: %v", err)
		}
		target := filepath.Join(destination, filepath.Clean(header.Name))
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatalf("create archive directory: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create archive parent: %v", err)
		}
		data := make([]byte, header.Size)
		if _, err := io.ReadFull(tarReader, data); err != nil {
			t.Fatalf("read archive file: %v", err)
		}
		if err := os.WriteFile(target, data, os.FileMode(header.Mode)); err != nil {
			t.Fatalf("write archive file: %v", err)
		}
	}
}
