# Optional Tunnel / Direct Public HTTPS Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox (`- [ ]`)
> syntax. Run the AGENTS.md verification gate
> (`go build ./... && go test ./... && gofmt -l . && go vet ./...`) before every
> commit. Never commit on red.

**Goal:** Make the tunnel optional. Model exposure as `local | tunnel | direct`;
let a `direct` profile carry a user-supplied public HTTPS URL that ezyl3 records
and uses for Cursor settings, with ngrok preflight firing only on the ngrok
tunnel path.

**Spec:** `docs/prd/2026-06-02-optional-tunnel.md`.

**Locked decisions:** `direct` stays a managed macOS profile (ezyl3 writes the
litellm LaunchAgent; user owns their reverse proxy). Strict URL validation
(`https://`, real host, reject http/localhost/loopback, fail fast). Doctor
HTTP-probes the direct public URL's `/health/liveliness`. Schema fields:
`ExposureMode` + `PublicURL` alongside existing `Domain`/`TunnelProvider`, with
backward-compat inference. macOS-only; no provider abstraction, no Linux, no
extra providers built.

---

## Task 1: Exposure schema + public-URL validation

**Files:**
- Modify: `internal/core/profile.go`
- Modify: `internal/core/ngrok.go` (or a small new validator location)
- Test: `internal/core/profile_test.go`
- Test: `internal/core/ngrok_test.go`

- [ ] **Step 1: Write failing tests**

  - Profile: add constants `ExposureLocal`/`ExposureTunnel`/`ExposureDirect` and
    fields `ExposureMode` (`json:"exposure_mode,omitempty"`) and `PublicURL`
    (`json:"public_url,omitempty"`). Test a helper `(Profile).Exposure()` that
    returns the stored mode, or infers it for legacy profiles: non-empty `Domain`
    → tunnel, else local.
  - Validation: `NormalizePublicURL(input) (string, error)` accepts
    `https://host/v1` (and `https://host` → normalized), rejects `http://...`,
    bare host, `localhost`, `127.0.0.1`, and `::1` with clear errors.

  Run: `go test ./internal/core -run 'TestProfileExposure|TestNormalizePublicURL'`
  Expected: FAIL (symbols missing).

- [ ] **Step 2: Implement**

  Add the constants, fields, `Exposure()` inference helper, and
  `NormalizePublicURL`. `NormalizePublicURL` parses with `net/url`, requires
  scheme `https` and a host that is not loopback/localhost, and returns the URL
  without a trailing slash so callers can append/standardize `/v1`.

- [ ] **Step 3: Verify** — `go test ./internal/core -run 'TestProfileExposure|TestNormalizePublicURL'` → PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/core/profile.go internal/core/profile_test.go internal/core/ngrok.go internal/core/ngrok_test.go
  git commit -m "feat: model exposure mode and validate direct public URLs"
  ```

## Task 2: Base-URL precedence in CursorSettingsInfo

**Files:**
- Modify: `internal/core/runtime.go`
- Test: `internal/core/runtime_test.go`

- [ ] **Step 1: Write failing tests**

  Extend `CursorSettingsInfo` precedence: a profile with `PublicURL` →
  `BaseURL = <publicURL>/v1`, `LocalOnly = false`; ngrok `Domain` → `https://<domain>/v1`;
  neither → `http://127.0.0.1:<port>/v1`, `LocalOnly = true`. Assert the direct
  case is not local-only and emits no warning via `CursorSettings`.

  Run: `go test ./internal/core -run 'TestCursorSettings'`
  Expected: FAIL.

- [ ] **Step 2: Implement**

  In `CursorSettingsInfo`, resolve base URL by precedence: load the profile; if it
  has a `PublicURL`, use it (append `/v1` consistently); else fall back to the
  existing `DetectNgrokDomain` path; else local. Keep `CursorSettings` formatting
  unchanged so existing string tests still pass.

- [ ] **Step 3: Verify** — `go test ./internal/core` → PASS (including unchanged CursorSettings string tests).

- [ ] **Step 4: Commit**

  ```bash
  git add internal/core/runtime.go internal/core/runtime_test.go
  git commit -m "feat: prefer a direct public URL over ngrok in cursor settings"
  ```

## Task 3: Setup honors --public-url; ngrok preflight gated to tunnel

**Files:**
- Modify: `internal/core/setup.go`
- Modify: `internal/core/install.go` (profile creation records exposure/public URL)
- Modify: `internal/cli/root.go` (flag)
- Test: `internal/core/setup_test.go`
- Test: `internal/cli/root_test.go`

- [ ] **Step 1: Write failing tests**

  - `RunSetup` with a `PublicURL` option creates a `direct` profile, writes NO
    ngrok LaunchAgent, and does NOT call the `NgrokChecker`.
  - `RunSetup` with a `Domain` keeps today's behavior: tunnel profile, ngrok
    LaunchAgent, preflight runs.
  - Supplying both `Domain` and `PublicURL` errors before writing anything.
  - Neither → `local` profile (no preflight).
  - CLI: `--public-url` is wired; `--public-url` + `--domain` errors.

  Run: `go test ./internal/core ./internal/cli -run 'Setup|PublicURL|Tunnel'`
  Expected: FAIL.

- [ ] **Step 2: Implement**

  Add `PublicURL string` to `core.SetupOptions`. In `RunSetup`: reject
  `Domain`+`PublicURL` together; if `PublicURL` set, `NormalizePublicURL` it,
  mark exposure `direct`, skip the ngrok preflight and ngrok LaunchAgent; if
  `Domain` set, unchanged tunnel path (preflight + ngrok agent); else `local`.
  `CreateManagedProfile` records `ExposureMode` and `PublicURL` on the profile.
  Add the `--public-url` flag in `root.go` and pass it through (CLI and wizard
  paths; wizard may keep `--public-url` CLI-only for now if cleaner — note it).

- [ ] **Step 3: Verify** — `go test ./internal/core ./internal/cli` → PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/core/setup.go internal/core/install.go internal/cli/root.go internal/core/setup_test.go internal/cli/root_test.go
  git commit -m "feat: add --public-url for direct exposure; gate ngrok preflight to tunnel"
  ```

## Task 4: Doctor reframing + direct reachability probe

**Files:**
- Modify: `internal/core/runtime.go` (Doctor)
- Test: `internal/core/runtime_test.go`

- [ ] **Step 1: Write failing tests**

  - A `direct` profile: doctor includes a check that HTTP-probes
    `<publicURL>/health/liveliness` (reusing `httpCheck`) and no ngrok-ready
    check.
  - A `tunnel` profile: ngrok-ready + tunnel-liveliness checks as today.
  - Warning/“not reachable” copy references both `--domain` and `--public-url`
    rather than ngrok only.

  Run: `go test ./internal/core -run 'TestDoctor'`
  Expected: FAIL.

- [ ] **Step 2: Implement**

  In `Doctor`, branch on exposure: `direct` → `httpCheck` against the public URL's
  liveliness endpoint, skip ngrok checks; `tunnel` → existing ngrok-ready +
  tunnel-liveliness; `local` → unchanged. Reframe shared copy to mention both
  tunnel and public-URL options.

- [ ] **Step 3: Verify** — `go test ./internal/core` → PASS.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/core/runtime.go internal/core/runtime_test.go
  git commit -m "feat: doctor probes direct public URL and reframes exposure guidance"
  ```

## Task 5: TUI direct mode + docs + final verification

**Files:**
- Modify: `internal/tui/tui_test.go` (assert direct flows through)
- Modify: `README.md`
- All modified files

- [ ] **Step 1: TUI test**

  Add a TUI test: a `direct` profile fixture shows its public URL on the Cursor
  tab and no local-only warning (it already renders from `CursorSettingsInfo`, so
  this should pass once Task 2 lands; assert it explicitly to lock it in).

- [ ] **Step 2: README**

  Document the three exposure scenarios (local / tunnel / direct), the
  `--public-url` flag, that ezyl3 records but does not manage TLS, and that ngrok
  is required only for the tunnel path.

- [ ] **Step 3: Full gate** — `go build ./... && go test ./... && gofmt -l . && go vet ./...` → clean.

- [ ] **Step 4: Manual verification (human)**

  Run `ezyl3 setup --skip-python-deps --public-url https://example.com` against an
  isolated HOME; confirm a direct profile is created with no ngrok agent, and
  `ezyl3 cursor settings` shows the public URL with no warning and no ngrok
  requirement.

- [ ] **Step 5: Commit and open PR**

  ```bash
  git add internal/tui/tui_test.go README.md
  git commit -m "docs: describe optional tunnel and direct public HTTPS exposure"
  ```

  Open a PR against `main`.

## Self-Review

- Spec coverage: exposure modeled as `local|tunnel|direct`; `--public-url` for
  direct; strict HTTPS validation; ngrok preflight gated to tunnel; base-URL
  precedence in `CursorSettingsInfo` (CLI + TUI for free); doctor probes direct;
  README scenarios; tests at each layer; backward-compat inference.
- Placeholder scan: none; the four confirmed decisions are baked in.
- Scope discipline: no provider abstraction, no Linux/systemd, no extra tunnel
  providers, no TLS/server management — only exposure is built; `tunnel_provider`
  stays a documented single-implementation string.
- Backward compatibility: profiles without `ExposureMode` infer from `Domain`;
  existing ngrok/tunnel tests must pass unchanged.
