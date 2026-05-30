# Setup Bubbles Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make interactive `ezyl3 setup` a Bubble Tea/Bubbles guided wizard while preserving the existing flag-driven non-interactive setup path.

**Architecture:** Keep `core.RunSetup` as the write/install orchestrator. Add a focused `internal/setupwizard` package that owns terminal interaction, builds `core.SetupOptions`, runs setup asynchronously, and returns `core.SetupResult`. Cobra only decides whether to run the wizard or the existing scripted path.

**Tech Stack:** Go, Cobra, Bubble Tea, Charmbracelet Bubbles (`textinput`, `progress`, `spinner`, `help`, `key`), Lip Gloss, existing `internal/core` setup APIs.

---

## File Structure

- Create `internal/setupwizard/wizard.go`: Bubble Tea model, step state, key handling, input rendering, setup execution command, and public `Run(options)` entrypoint.
- Create `internal/setupwizard/wizard_test.go`: model/update tests for step navigation, secret masking, review rendering, setup execution, and cancellation.
- Modify `internal/cli/root.go`: delegate interactive `setup` to `setupwizard.Run`; keep scripted setup behavior and flags intact.
- Modify `internal/cli/root_test.go`: assert non-interactive/scripted setup still works and interactive detection does not affect test command execution.
- No core setup behavior changes unless tests expose a missing seam.

## Task 1: Add Wizard Model Skeleton

**Files:**
- Create: `internal/setupwizard/wizard.go`
- Test: `internal/setupwizard/wizard_test.go`

- [ ] **Step 1: Write failing tests for initial wizard state**

Add tests that construct a model with profile `default`, a temp `core.ProfilePaths`, and `SkipPythonDeps: true`. Assert the first view includes `ezyl3 setup`, `Profile`, `Tunnel`, `Secrets`, `Review`, and `enter next`.

- [ ] **Step 2: Run focused test**

Run: `go test ./internal/setupwizard`

Expected: package or symbols missing.

- [ ] **Step 3: Implement model skeleton**

Create `setupwizard.Options`, `model`, `step` constants, `newModel`, `Init`, `Update`, and `View`. Use Bubbles `progress`, `help`, and `key` for shell structure. Keep the first implementation read-only and deterministic.

- [ ] **Step 4: Verify focused test passes**

Run: `go test ./internal/setupwizard`

Expected: pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/setupwizard/wizard.go internal/setupwizard/wizard_test.go
git commit -m "feat: add setup wizard shell"
```

## Task 2: Add Guided Inputs

**Files:**
- Modify: `internal/setupwizard/wizard.go`
- Test: `internal/setupwizard/wizard_test.go`

- [ ] **Step 1: Write failing tests for fields**

Add tests that:
- enter an ngrok domain and confirm it is saved on the tunnel step
- leave the domain blank and confirm local-only copy appears
- enter secret values and confirm the rendered view contains `******` but not the raw secret
- navigate back and forward without losing input values

- [ ] **Step 2: Run focused test**

Run: `go test ./internal/setupwizard`

Expected: failures showing inputs are not implemented.

- [ ] **Step 3: Implement text inputs**

Use `textinput.Model` for domain, Hugging Face token, Ollama key, Hugging Face billing org, and LiteLLM master key. Set secret inputs to `textinput.EchoPassword`. Use `enter` for next, `shift+tab`/`esc` or `b` for back, and `q`/`ctrl+c` for cancel.

- [ ] **Step 4: Verify focused test passes**

Run: `go test ./internal/setupwizard`

Expected: pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/setupwizard/wizard.go internal/setupwizard/wizard_test.go
git commit -m "feat: add setup wizard inputs"
```

## Task 3: Run Setup From Review Step

**Files:**
- Modify: `internal/setupwizard/wizard.go`
- Test: `internal/setupwizard/wizard_test.go`

- [ ] **Step 1: Write failing tests for execution**

Add fake setup runner tests that:
- on the Review step, pressing `enter` sends `core.SetupOptions` with the captured domain, secrets, force, and skip-python-deps values
- the Running step renders a spinner/progress copy
- success renders the redacted `core.SetupResult.Summary()`
- errors render a clear failure message and keep raw secret values out of the view

- [ ] **Step 2: Run focused test**

Run: `go test ./internal/setupwizard`

Expected: failures showing execution is not wired.

- [ ] **Step 3: Implement async setup command**

Add a `setupRunner` dependency type and a `setupDoneMsg`. In the Review step, `enter` moves to Running and returns a Bubble Tea command that calls the runner. On success, store `core.SetupResult`; on failure, store the error. Keep secrets only in state needed to call setup and never print raw values.

- [ ] **Step 4: Verify focused test passes**

Run: `go test ./internal/setupwizard`

Expected: pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/setupwizard/wizard.go internal/setupwizard/wizard_test.go
git commit -m "feat: run setup from wizard"
```

## Task 4: Wire Cobra Interactive Setup

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`

- [ ] **Step 1: Write failing CLI tests**

Add or update tests proving:
- non-interactive `setup` with flags still calls `core.RunSetup` and prints the existing summary
- when `isInteractive(cmd)` is false, the Bubble Tea wizard is not invoked
- existing redaction expectations still hold

- [ ] **Step 2: Run CLI tests**

Run: `go test ./internal/cli`

Expected: pass for unchanged behavior before wiring if tests only assert current scripted path, or fail if a seam is needed.

- [ ] **Step 3: Wire interactive branch**

In `setupCommand`, if `isInteractive(cmd)` is true, resolve paths and call `setupwizard.Run(setupwizard.Options{...})`. Pass any explicitly supplied flag values into the wizard as initial values. If not interactive, skip all prompt reads and keep current scripted `core.RunSetup` behavior.

- [ ] **Step 4: Verify CLI tests**

Run: `go test ./internal/cli`

Expected: pass.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/cli/root.go internal/cli/root_test.go internal/setupwizard/wizard.go internal/setupwizard/wizard_test.go
git commit -m "feat: launch setup wizard interactively"
```

## Task 5: Final Verification And Polish

**Files:**
- Modify as needed: `README.md`
- Modify as needed: `internal/setupwizard/wizard.go`
- Modify as needed: `internal/setupwizard/wizard_test.go`

- [ ] **Step 1: Update docs**

Update `README.md` setup section to say interactive `ezyl3 setup` opens a guided terminal wizard using masked inputs, while flags remain available for scripts.

- [ ] **Step 2: Run all tests**

Run: `go test ./...`

Expected: all packages pass.

- [ ] **Step 3: Check diff hygiene**

Run: `git diff --check`

Expected: no output and exit 0.

- [ ] **Step 4: Commit docs/polish**

Run:

```bash
git add README.md internal/setupwizard/wizard.go internal/setupwizard/wizard_test.go
git commit -m "docs: describe setup wizard"
```

## Self-Review

- Spec coverage: the plan covers a Bubble Tea/Bubbles interactive setup path, secret masking, review/run/success states, Cobra wiring, scripted compatibility, tests, and docs.
- Placeholder scan: no TBD/TODO/fill-in-later placeholders remain.
- Type consistency: public seams are `setupwizard.Options`, `setupwizard.Run`, a package-local setup runner, and existing `core.SetupOptions`/`core.SetupResult`.
