# Cursor TUI View Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox (`- [ ]`)
> syntax for tracking. Run the AGENTS.md verification gate
> (`go build ./... && go test ./... && gofmt -l . && go vet ./...`) before every
> commit. Never commit on red.

**Goal:** Add a guided **Cursor** tab to the TUI that surfaces base URL, model
names, an opt-in revealed API key, the local-only warning, and ngrok readiness —
reusing `core` logic so TUI and CLI never diverge.

**Spec:** `docs/prd/2026-06-02-cursor-tui-view.md`.

**Locked decisions:** Cursor tab placed 2nd (after Overview); reveal key is `c`,
gated to the Cursor tab; a structured `core.CursorInfo` accessor is the single
source of truth (CLI `CursorSettings` formats from it); no clipboard copy.

---

## Task 1: Extract structured `core.CursorInfo`

**Files:**
- Modify: `internal/core/runtime.go`
- Test: `internal/core/runtime_test.go`

- [ ] **Step 1: Write failing tests**

Add a test for a new `CursorSettingsInfo(runtime) (CursorInfo, error)` asserting:
- tunneled profile → `BaseURL` is `https://<domain>/v1`, `LocalOnly` false,
  `MasterKey` is the clean unquoted key, `Models` contains the five names.
- local-only profile → `BaseURL` is `http://127.0.0.1:<port>/v1`, `LocalOnly`
  true.
Keep the existing `CursorSettings` (string) tests unchanged — they are the
guarantee that CLI output does not drift.

Run: `go test ./internal/core -run 'TestCursorSettings'`
Expected: FAIL (symbol missing).

- [ ] **Step 2: Implement `CursorInfo` + accessor, refactor `CursorSettings`**

Add `type CursorInfo struct { BaseURL string; MasterKey string; LocalOnly bool;
Models []string }` and `CursorSettingsInfo(runtime)`. Move the base-URL/domain/
secret-reading logic into it. Reimplement `CursorSettings(runtime, reveal)` to
call `CursorSettingsInfo` and format the exact same string it produces today
(redacted key unless `reveal`, same local-only warning text, same model line).
`Models` should be the canonical five Cursor picker names.

- [ ] **Step 3: Verify**

Run: `go test ./internal/core`
Expected: PASS, including the unchanged `CursorSettings` string tests (proves
byte-identical CLI output).

- [ ] **Step 4: Commit**

```bash
git add internal/core/runtime.go internal/core/runtime_test.go
git commit -m "refactor: extract core.CursorInfo as the source for cursor settings"
```

## Task 2: Add the Cursor tab (read-only render)

**Files:**
- Modify: `internal/tui/tui.go`
- Test: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing tests**

Using a fake runtime fixture (profile.json + `.env`), assert that with the active
tab set to the Cursor tab, the rendered view contains:
- the base URL,
- the five model names,
- `API key: set` (redacted) and NOT the raw key value,
- for a local-only fixture: the local-only warning text; for a tunneled fixture:
  no warning.

Run: `go test ./internal/tui -run 'TestCursor'`
Expected: FAIL.

- [ ] **Step 2: Implement the tab**

Add a `cursorTab` constant positioned second in the iota block and `"Cursor"` as
the second entry of `tabs`. Renumber subsequent constants. Add a `reveal bool`
field to `model`. Add `renderCursor()` that reads `core.CursorSettingsInfo(m.runtime)`
and renders base URL, models, redacted-or-revealed key (per `m.reveal`), and the
local-only warning when `info.LocalOnly`. Wire `cursorTab` into `renderMain`'s
switch. Refresh logic: `CursorInfo` can be read at render time or cached in
`refresh()`; do not store the cleartext key when redacted.

- [ ] **Step 3: Verify**

Run: `go test ./internal/tui`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat: add Cursor tab to the TUI"
```

## Task 3: Reveal-key toggle gated to the Cursor tab

**Files:**
- Modify: `internal/tui/tui.go`
- Test: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing tests**

Assert:
- pressing `c` on the Cursor tab flips reveal; the view then shows the clean
  unquoted key (`API key: sk-cursor-...`) and never wraps it in quotes.
- pressing `c` again re-redacts.
- pressing `c` on a non-Cursor tab does nothing (reveal stays false / no key
  leaks).

Run: `go test ./internal/tui -run 'TestCursorReveal'`
Expected: FAIL.

- [ ] **Step 2: Implement**

Add a `reveal` binding (`c`) to `keyMap`, enabled only when `activeTab ==
cursorTab` (mirror the services-tab gating of start/stop/restart). Handle it in
`Update` to toggle `m.reveal`. Surface it in `ShortHelp`/`FullHelp` when on the
Cursor tab.

- [ ] **Step 3: Verify**

Run: `go test ./internal/tui`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat: reveal Cursor API key with c on the Cursor tab"
```

## Task 4: ngrok readiness badge (tunneled profiles)

**Files:**
- Modify: `internal/tui/tui.go`
- Test: `internal/tui/tui_test.go`

- [ ] **Step 1: Write failing tests**

Extend the TUI `dependencies` with an injectable ngrok check. Assert:
- tunneled fixture + fake checker returning nil → Cursor tab shows a ready badge.
- tunneled fixture + fake checker returning an error → badge shows the first line
  of that error.
- local-only fixture → no ngrok badge (local-only does not need ngrok).

Run: `go test ./internal/tui -run 'TestCursorNgrok'`
Expected: FAIL.

- [ ] **Step 2: Implement**

Add `ngrok func() error` to the TUI `dependencies` struct, defaulting in
`newModelWithDeps` to `core.DefaultNgrokChecker{}.CheckNgrokReady`. In
`renderCursor`, when `!info.LocalOnly`, run the check and render a badge (ready,
or `firstLine(err)`), reusing the existing status-style helpers.

- [ ] **Step 3: Verify**

Run: `go test ./internal/tui`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat: show ngrok readiness on the Cursor tab"
```

## Task 5: Docs and final verification

**Files:**
- Modify: `README.md`
- All modified files

- [ ] **Step 1: Update README**

In the TUI section, note the Cursor tab shows base URL, model names, the API key
(press `c` to reveal), the local-only warning, and ngrok readiness.

- [ ] **Step 2: Full gate**

Run: `go build ./... && go test ./... && gofmt -l . && go vet ./...`
Expected: all clean.

- [ ] **Step 3: Manual verification (human, required)**

The alt-screen cannot be verified headlessly. Run `ezyl3 tui` against a real
managed profile and confirm: the Cursor tab renders cleanly; `c` reveals/redacts
the key; the local-only warning shows for a local-only profile; the ngrok badge
reflects reality.

- [ ] **Step 4: Commit and open PR**

```bash
git add README.md
git commit -m "docs: describe the Cursor TUI tab"
```

Open a PR against `main` via the worktree flow.

## Self-Review

- Spec coverage: structured `CursorInfo` source of truth, Cursor tab (base URL,
  models, redacted/revealed key, local-only warning, ngrok badge), reveal gated
  to the tab, README, tests, and a required manual TUI check.
- Placeholder scan: no TBD/fill-in-later remain; the four locked decisions are
  baked in.
- Type consistency: new seams are `core.CursorInfo`, `core.CursorSettingsInfo`,
  the TUI `cursorTab` + `model.reveal` + `dependencies.ngrok`, all additive and
  reusing existing `core` logic.
- Secret handling: redacted by default everywhere; key revealed only on explicit
  `c`; cleartext never stored in model state when redacted — consistent with
  AGENTS.md.
