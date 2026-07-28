package scripts

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var actionUse = regexp.MustCompile(`(?m)^\s*uses:\s+[^@\s]+@([^\s]+)\s*$`)
var commitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

func TestWorkflowsPinActionsAndReleaseArtifacts(t *testing.T) {
	repoRoot := scriptRepoRoot(t)
	for _, name := range []string{"pr.yml", "lab.yml", "release.yml"} {
		contents, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range actionUse.FindAllStringSubmatch(string(contents), -1) {
			if !commitSHA.MatchString(match[1]) {
				t.Fatalf("%s uses unpinned action ref %q", name, match[1])
			}
		}
	}

	release, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"make package-smoke", "sha256sum anchorscan-*.tar.gz > checksums.txt", "dist/checksums.txt"} {
		if !strings.Contains(string(release), want) {
			t.Fatalf("release workflow does not contain %q", want)
		}
	}

	pr, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "pr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pr), "make security-check") {
		t.Fatal("PR workflow does not run dependency security checks")
	}

	makefile, err := os.ReadFile(filepath.Join(repoRoot, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"govulncheck@v1.1.4", "npm audit --audit-level=high", "uv lock --check"} {
		if !strings.Contains(string(makefile), want) {
			t.Fatalf("Makefile security check does not contain %q", want)
		}
	}
}

func TestLabWorkflowUsesPinnedImages(t *testing.T) {
	repoRoot := scriptRepoRoot(t)
	contents, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "lab.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if !strings.Contains(line, "image:") {
			continue
		}
		if !strings.Contains(line, "@sha256:") {
			t.Fatalf("lab image is not digest-pinned: %s", line)
		}
	}
}

func scriptRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate workflow test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}
