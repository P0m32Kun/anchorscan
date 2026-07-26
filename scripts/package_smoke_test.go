//go:build packageintegration

package scripts

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
)

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
	assertBinaryVersion(t, filepath.Join(packageDir, binaryName), version)

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
	if _, err := os.Stat(filepath.Join(packageDir, "config", "default.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("package must not include local config/default.yaml: %v", err)
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
}

func TestBuildVersionCanBeInjected(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(testFileDir(t), ".."))
	injectedVersion := "v9.8.7-linker-test"
	binary := filepath.Join(t.TempDir(), "anchorscan")

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-X github.com/P0m32Kun/anchorscan/internal/version.Version="+injectedVersion, "-o", binary, "./cmd/anchorscan")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build injected version: %v\n%s", err, output)
	}
	assertBinaryVersion(t, binary, injectedVersion)

	devBinary := filepath.Join(t.TempDir(), "anchorscan")
	cmd = exec.Command("go", "build", "-trimpath", "-o", devBinary, "./cmd/anchorscan")
	cmd.Dir = repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build development version: %v\n%s", err, output)
	}
	assertBinaryVersion(t, devBinary, "dev")
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
