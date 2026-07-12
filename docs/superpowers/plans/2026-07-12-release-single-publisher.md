# Single-Publisher Release Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish every tagged release once, after all three platform binaries build successfully.

**Architecture:** The `build` matrix produces one artifact per target without touching GitHub Releases. A dependent `release` job downloads every artifact and invokes the release action exactly once.

**Tech Stack:** GitHub Actions, Bash, Go cross-compilation.

## Global Constraints

- Keep the three existing target pairs: linux/amd64, darwin/arm64, windows/amd64.
- Add no third-party dependency beyond the Actions already used by the workflow.
- Preserve prerelease detection and generated release notes.

---

### Task 1: Assert the single-publisher workflow contract

**Files:**
- Create: `e2e/check-release-workflow.sh`

**Interfaces:**
- Consumes: `.github/workflows/release.yml`
- Produces: exit status 0 only when the workflow uploads build artifacts and has one dependent release publisher.

- [ ] **Step 1: Write the failing check**

```bash
#!/usr/bin/env bash
set -euo pipefail

workflow=.github/workflows/release.yml
grep -q 'actions/upload-artifact@' "$workflow"
grep -q 'actions/download-artifact@' "$workflow"
grep -q 'needs: build' "$workflow"
test "$(grep -c 'softprops/action-gh-release@' "$workflow")" -eq 1
```

- [ ] **Step 2: Run the check to verify it fails**

Run: `bash e2e/check-release-workflow.sh`

Expected: non-zero because v1.7.0's workflow does not use build artifacts and has no dependent release job.

### Task 2: Publish from one post-build job

**Files:**
- Modify: `.github/workflows/release.yml:13-59`
- Test: `e2e/check-release-workflow.sh`

**Interfaces:**
- Consumes: matrix output files named `anchorscan-<os>-<arch>[.exe]`.
- Produces: a single `softprops/action-gh-release@v2` invocation after all matrix jobs finish.

- [ ] **Step 1: Upload the binary from each build matrix entry**

Replace the current prerelease/release steps with:

```yaml
      - name: Upload build artifact
        uses: actions/upload-artifact@v4
        with:
          name: anchorscan-${{ matrix.goos }}-${{ matrix.goarch }}
          path: anchorscan-*
          if-no-files-found: error
```

- [ ] **Step 2: Add one dependent release job**

```yaml
  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Download build artifacts
        uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true

      - name: Detect prerelease
        id: pre
        run: |
          if [[ "${GITHUB_REF_NAME}" =~ -(rc|beta|alpha|pre)[0-9]*$ ]]; then
            echo "is_prerelease=true" >> "$GITHUB_OUTPUT"
          else
            echo "is_prerelease=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Upload release assets
        uses: softprops/action-gh-release@v2
        with:
          files: dist/anchorscan-*
          prerelease: ${{ steps.pre.outputs.is_prerelease }}
          generate_release_notes: true
```

- [ ] **Step 3: Run the contract check**

Run: `bash e2e/check-release-workflow.sh`

Expected: exit status 0.

- [ ] **Step 4: Compile and test the project**

Run: `go test ./... && go build ./cmd/anchorscan`

Expected: exit status 0.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/release.yml e2e/check-release-workflow.sh
git commit -m "ci: publish release assets from one job"
```
