# Release Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ezyl3` installable via an unsigned Homebrew tap and protected by CI. Adds a pull-request CI workflow, an `ezyl3 version` command with build-time version injection, an MIT `LICENSE`, and a GoReleaser-driven tagged release workflow that publishes macOS binaries and updates the Homebrew tap formula.

**Architecture:** CI runs format/vet/test on every PR. Releases are cut by pushing a `vX.Y.Z` tag, which triggers a GitHub Actions workflow running GoReleaser. GoReleaser builds darwin arm64/amd64, injects the version via `-ldflags -X` into a package variable read by `ezyl3 version`, publishes a GitHub Release, and pushes an updated formula to a separate tap repo. Distribution is **unsigned**: Homebrew installs binaries without the browser quarantine attribute, so unsigned binaries run cleanly for our developer audience. Notarization and downloadable installers are out of scope.

**Tech Stack:** Go, Cobra CLI, GitHub Actions, GoReleaser, Homebrew tap repository.

**Decisions locked for this plan:**
- Unsigned distribution. No Apple Developer Program, `codesign`, notarization, or stapling.
- macOS only: build darwin arm64 and amd64; no Linux/Windows artifacts.
- Tap repo default: `tlarevo/homebrew-tap`, yielding `brew install tlarevo/tap/ezyl3`. Confirm the repo exists before Task 4; adjust the formula owner if the maintainer chooses a different name.

**External prerequisites (maintainer, not code):**
- Create the tap repository `tlarevo/homebrew-tap` if it does not exist.
- Provision a GitHub token (a classic PAT or fine-grained token) with write access to the tap repo, stored as a repository secret (for example `HOMEBREW_TAP_TOKEN`) on `tlarevo/ezyl3`. GoReleaser cannot bootstrap this itself.

---

## Task 1: Add CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

Create a workflow triggered on `pull_request` and on `push` to the default branch. One job on `macos-latest` (the tool is macOS-only) that:
- checks out the repo
- sets up Go using the version in `go.mod` (1.24.x)
- runs `test -z "$(gofmt -l .)"` (fails on any unformatted file, printing offenders)
- runs `go vet ./...`
- runs `go test ./...`

- [ ] **Step 2: Validate locally**

Run the same three commands locally to confirm the current tree passes:
`gofmt -l .` (expect no output), `go vet ./...`, `go test ./...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run gofmt, vet, and tests on pull requests"
```

## Task 2: Add `ezyl3 version`

**Files:**
- Create: `internal/version/version.go`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`
- Test: `internal/version/version_test.go`

- [ ] **Step 1: Write failing tests**

Add a CLI test asserting `ezyl3 version` prints the value returned by the version package, and that the default (no injected value) prints a clear development placeholder such as `dev`. Add a version-package test asserting the default value and that an injected override is reported.

Run: `go test ./internal/cli ./internal/version -run 'TestVersion'`
Expected: FAIL because the package and command do not exist.

- [ ] **Step 2: Implement version package and command**

Create `internal/version/version.go` with an exported package variable `Version = "dev"` (overridable via `-ldflags -X ezyl3/internal/version.Version=...`) and an accessor if helpful for testing. Register `versionCommand(opts)` in `NewRootCommand` that prints the version to stdout.

- [ ] **Step 3: Verify**

Run: `go test ./internal/cli ./internal/version -run 'TestVersion'`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/version/version.go internal/version/version_test.go internal/cli/root.go internal/cli/root_test.go
git commit -m "feat: add ezyl3 version command"
```

## Task 3: Add LICENSE

**Files:**
- Create: `LICENSE`

- [ ] **Step 1: Add MIT license**

Add a standard MIT `LICENSE` with the correct copyright holder and year (confirm holder name with the maintainer; default to the repository owner). Use MIT unless the maintainer specifies otherwise.

- [ ] **Step 2: Commit**

```bash
git add LICENSE
git commit -m "chore: add MIT license"
```

## Task 4: Add GoReleaser Config and Release Workflow

**Files:**
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yml`
- Modify: `README.md`

- [ ] **Step 1: Write GoReleaser config**

Create `.goreleaser.yaml` that:
- builds `./cmd/ezyl3` for `goos: [darwin]`, `goarch: [arm64, amd64]`
- injects version with `ldflags: -s -w -X ezyl3/internal/version.Version={{.Version}}`
- produces tar.gz archives
- configures a `brews:` block targeting `tlarevo/homebrew-tap` with the formula name `ezyl3`, using the `HOMEBREW_TAP_TOKEN` secret

- [ ] **Step 2: Validate config**

Run: `goreleaser check`
Expected: config is valid. If `goreleaser` is not installed locally, note that CI will run `goreleaser check` instead.

- [ ] **Step 3: Write release workflow**

Create `.github/workflows/release.yml` triggered on tags matching `v*`. It checks out with full history, sets up Go from `go.mod`, and runs GoReleaser (release on tag). Provide `GITHUB_TOKEN` for the GitHub Release and `HOMEBREW_TAP_TOKEN` for the formula push. Optionally add a `goreleaser check` plus snapshot build step gated to pull requests so config breakage is caught without cutting a tag.

- [ ] **Step 4: Update README install section**

Make `brew install tlarevo/tap/ezyl3` the primary install instruction. Keep "Build and Run From Source" as the developer path. Add a one-line note that binaries are unsigned and delivered through Homebrew. Document `ezyl3 version`.

- [ ] **Step 5: Verify build still works**

Run: `go build ./...` and `go test ./...`
Expected: PASS. (A real release is cut later by pushing a tag, not during this task.)

- [ ] **Step 6: Commit**

```bash
git add .goreleaser.yaml .github/workflows/release.yml README.md
git commit -m "ci: add goreleaser release workflow and homebrew tap"
```

## Task 5: Final Verification

**Files:**
- All modified files

- [ ] **Step 1: Run full tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 2: Check formatting and diff hygiene**

Run: `gofmt -l .` (expect no output) and `git diff --check` (expect clean).

- [ ] **Step 3: Confirm external prerequisites with maintainer**

Confirm the tap repo exists and `HOMEBREW_TAP_TOKEN` is set before the first tag is pushed. The first release is cut by pushing `v0.1.0` after this plan merges; that push is a maintainer action outside this plan.

## Self-Review

- Spec coverage: adds PR CI (fmt/vet/test), `ezyl3 version` with ldflags injection, MIT LICENSE, GoReleaser config and tag-triggered release workflow targeting an unsigned Homebrew tap, and README install updates.
- Placeholder scan: no TBD/TODO/fill-in-later placeholders remain; tap repo name and license holder are explicit confirm-points, not silent gaps.
- Type consistency: new seam is `internal/version.Version` consumed by `versionCommand`; release tooling is config/workflow only and does not change core APIs.
- External dependency isolation: only Task 4 depends on the tap repo and token; Tasks 1–3 are fully self-contained.
