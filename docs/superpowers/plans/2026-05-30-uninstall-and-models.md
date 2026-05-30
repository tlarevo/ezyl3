# Uninstall and Model Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ezyl3` cleanly removable and restore the managed config to parity with the original `~/litellm-cursor` kit. Adds `ezyl3 uninstall` with a testable planning boundary, and fixes a feature regression where the managed `setup` config is missing the `litellm-auto` complexity router, the per-model Hugging Face org-billing header, and the extra Ollama fallback hops.

**Architecture:** Setup writes a managed profile, LaunchAgents, logs, and uv/Python caches. Uninstall reverses that for managed profiles only. A pure planner in `internal/core` computes the set of service actions and removal paths from `ProfilePaths` plus the loaded `Profile`, so the decision logic is unit-testable without deleting files or calling `launchctl`. The CLI executes the plan: it stops services via the existing `ServiceManager`, removes plists, and removes managed directories. External (imported) profiles never have their runtime files removed.

The managed LiteLLM config is currently a static `const DefaultLiteLLMConfig` ([install.go:10](../../internal/core/install.go)) that cannot reflect user input. This plan converts it into a render function so the org-billing header can be conditionally included from the user-supplied bill-to value.

**Tech Stack:** Go, Cobra CLI, existing `internal/core` `ProfilePaths`/`Profile`/`Secrets`/`ServiceManager` types, LiteLLM `auto_router/complexity_router`, standard library filesystem operations.

**Decisions locked for this plan:**
- `litellm-auto` is a **real LiteLLM complexity router** in the original kit, not phantom copy. It must be **restored** to the managed config, not removed. Router thresholds and default are copied verbatim from the original: `simple_medium: 0.15`, `medium_complex: 0.35`, `complex_reasoning: 0.60`, `default_model: litellm-medium`, routing `SIMPLE/MEDIUM/COMPLEX/REASONING → litellm-simple/medium/complex/reasoning`.
- The `X-HF-Bill-To` header is an **optional** per-user choice driven by the existing `Secrets.HFBillTo` (`--hf-bill-to` flag / wizard input). It is injected as `extra_headers` on each Hugging Face model entry **only when bill-to is non-empty**, and omitted entirely when blank. The render-time approach is used (not `os.environ/HF_BILL_TO`) so a blank value produces no header rather than an empty one.
- The original's extra fallback hops are restored: `litellm-auto` falls back across all four `-fb` tiers, and `medium-fb`/`complex-fb`/`reasoning-fb` each fall back to `simple-fb` as a final cheapest-tier downgrade.
- Uninstall is **managed-only** for runtime files. For an external profile it removes only `ezyl3`'s own metadata directory and any LaunchAgents `ezyl3` wrote, never the imported runtime path.

---

## Task 1: Restore `litellm-auto` Router, Optional Bill-To Header, and Fallbacks

**Files:**
- Modify: `internal/core/install.go`
- Modify: `internal/core/config.go`
- Modify: `internal/core/setup.go` (only if `CreateManagedProfile`/`RunSetup` need to pass secrets to the renderer; see Step 3)
- Modify: `README.md`
- Test: `internal/core/config_test.go`
- Test: `internal/core/install_test.go` (create if absent)
- Test: `internal/core/setup_test.go`

- [ ] **Step 1: Write failing tests**

Add tests asserting:
- A rendered managed config (with a non-empty bill-to) parses via `LoadLiteLLMConfig`, contains a `litellm-auto` model whose `litellm_params.model` is `auto_router/complexity_router`, and includes the four tier mappings and the `default_model: litellm-medium`.
- The rendered config passes `(*LiteLLMConfig).Validate()` — which now requires `litellm-auto` (see Step 4).
- With a **non-empty** bill-to, each Hugging Face model entry carries `extra_headers` with `X-HF-Bill-To` set to that value.
- With an **empty** bill-to, no `extra_headers`/`X-HF-Bill-To` appears anywhere in the rendered config.
- `litellm_settings.fallbacks` includes `litellm-auto → [all four -fb tiers]` and the `medium-fb/complex-fb/reasoning-fb → [simple-fb]` hops.
- `SetupResult.Summary()` ([setup.go:143](../../internal/core/setup.go)) and `CursorSettings` ([runtime.go:90](../../internal/core/runtime.go)) still advertise `litellm-auto` plus the four tiers (no copy change needed; assert they remain correct).

Run: `go test ./internal/core -run 'TestRenderLiteLLMConfig|TestValidate|TestCursorSettings|TestSetupResultSummary'`
Expected: FAIL because the renderer and validation entry do not exist yet.

- [ ] **Step 2: Convert the config const to a renderer**

Replace `const DefaultLiteLLMConfig` with `func RenderLiteLLMConfig(secrets Secrets) string` (or a small typed config builder marshaled via `gopkg.in/yaml.v3`, whichever keeps the diff readable). The renderer must:
- Define the four HF primary tiers and four Ollama `-fb` tiers exactly as today.
- Add the `litellm-auto` entry with `model: auto_router/complexity_router` and the `complexity_router_config` block using the locked thresholds and tier mapping.
- Inject `extra_headers: { X-HF-Bill-To: <secrets.HFBillTo> }` on each HF entry **only if** `strings.TrimSpace(secrets.HFBillTo) != ""`.
- Emit `litellm_settings.fallbacks` including the `litellm-auto` chain and the extra `-fb → simple-fb` hops, plus the existing per-tier `tier → tier-fb` entries.
- Keep `drop_params`, `num_retries`, `request_timeout`, the usage callback wiring, and `general_settings.master_key: os.environ/LITELLM_MASTER_KEY` intact.

- [ ] **Step 3: Pass secrets into config creation**

Update `CreateManagedProfile` ([install.go:72](../../internal/core/install.go)) to write `RenderLiteLLMConfig(secrets)` instead of the const. `CreateManagedProfile` already receives `secrets`, so this is a local change; confirm `RunSetup` still passes the resolved secrets (including a generated master key) before config creation.

- [ ] **Step 4: Require `litellm-auto` in validation**

Add `"litellm-auto"` to `RequiredModelNames` ([config.go:11](../../internal/core/config.go)) so the config that `setup` writes — and any user edit — must keep the router defined. Confirm `Validate()` does not choke on the router entry (its `litellm_params.model` is non-empty, satisfying the existing check); fallback-source/target validation should pass because all referenced names are defined.

- [ ] **Step 5: Update README**

Confirm the "Common model names" list keeps `litellm-auto` and document that it is a complexity router that auto-selects a tier. Add a one-line note that `--hf-bill-to` (or the wizard bill-to field) adds the `X-HF-Bill-To` org-billing header to Hugging Face requests, and that leaving it blank omits the header.

- [ ] **Step 6: Verify**

Run: `go test ./internal/core`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/core/install.go internal/core/config.go internal/core/setup.go README.md internal/core/config_test.go internal/core/install_test.go internal/core/setup_test.go
git commit -m "fix: restore litellm-auto router, optional bill-to header, and fallbacks in managed config"
```

## Task 2: Add Uninstall Planner

**Files:**
- Create: `internal/core/uninstall.go`
- Test: `internal/core/uninstall_test.go`

- [ ] **Step 1: Write failing tests for the planner**

Add tests that build a temp managed profile (`ProfileDir`, `LogsDir`, a `profile.json` with `mode: managed`, and stub LaunchAgent plist files) and assert `PlanUninstall(paths, profile)` returns:
- service stop actions for `litellm` and `ngrok` (ngrok marked optional/not-configured when its plist is absent)
- removal paths including `ProfileDir`, `LogsDir`, and the `litellm`/`ngrok` LaunchAgent plist paths
- for a `mode: external` profile, the imported `RuntimeDir` is **never** in the removal set; only the `ezyl3` `ProfileDir` metadata and LaunchAgents are planned

Run: `go test ./internal/core -run TestPlanUninstall`
Expected: FAIL because `PlanUninstall` does not exist.

- [ ] **Step 2: Implement the planner**

Add `UninstallPlan` (fields: `Profile string`, `Mode string`, `RemovePaths []string`, `Services []string`, `ExternalRuntimeDir string`) and `PlanUninstall(paths ProfilePaths, profile Profile) UninstallPlan`. The planner only inspects inputs and `os.Stat`s plist existence; it performs no deletions and runs no commands. For managed profiles include `ProfileDir`, `LogsDir`, and existing LaunchAgent plists. For external profiles include only `ProfileDir` and existing LaunchAgent plists, and record `ExternalRuntimeDir` for display without adding it to `RemovePaths`. Do not include the shared `CacheDir` root by default; if cache removal is desired, scope it to an `ezyl3`-owned subpath only and document it in the plan output.

- [ ] **Step 3: Verify**

Run: `go test ./internal/core -run TestPlanUninstall`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/core/uninstall.go internal/core/uninstall_test.go
git commit -m "feat: add uninstall planner"
```

## Task 3: Add CLI `uninstall`

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`

- [ ] **Step 1: Write failing CLI tests**

Add tests proving:
- root help exposes `uninstall`
- `ezyl3 uninstall --dry-run` prints the planned removals and service stops and deletes nothing (assert temp files still exist afterward)
- `ezyl3 uninstall --force` on a managed temp profile removes the planned paths
- for an external profile, the output names the imported runtime as preserved and the runtime files still exist after `--force`
- no secret values appear in output

Use an injectable service runner so tests do not call `launchctl`, consistent with `service` tests.

Run: `go test ./internal/cli -run TestUninstall`
Expected: FAIL because the command does not exist.

- [ ] **Step 2: Implement command wiring**

Register `uninstallCommand(opts)` in `NewRootCommand`. It resolves `ProfilePaths` via `core.ResolvePaths(envMap(), opts.profile)`, loads the profile (best-effort; default to managed-shaped paths if `profile.json` is absent), calls `core.PlanUninstall`, prints the plan, and then:
- if `--dry-run`: stop after printing
- else require interactive confirmation unless `--force`
- on confirmation: stop services through `core.ServiceManager` (ignoring missing/not-configured ngrok), then remove each path in `RemovePaths`
Flags: `--dry-run`, `--force`. Reuse `isInteractive(cmd)` for the confirmation gate; in non-interactive mode without `--force`, fail with a clear message instructing the user to pass `--force`.

- [ ] **Step 3: Verify**

Run: `go test ./internal/cli -run TestUninstall`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat: add ezyl3 uninstall command"
```

## Task 4: Documentation and Final Verification

**Files:**
- Modify: `README.md`
- All modified files

- [ ] **Step 1: Document uninstall**

Add a README section describing `ezyl3 uninstall`, `--dry-run`, `--force`, the managed-versus-external behavior (external runtime files are never deleted), and exactly which paths are removed.

- [ ] **Step 2: Run full tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 3: Check diff hygiene**

Run: `git diff --check`
Expected: no output and exit 0.

- [ ] **Step 4: Commit docs**

```bash
git add README.md
git commit -m "docs: describe ezyl3 uninstall"
```

## Self-Review

- Spec coverage: restores the `litellm-auto` complexity router and extra fallback hops to the managed config, makes the `X-HF-Bill-To` header an optional render-time choice driven by `Secrets.HFBillTo`, enforces `litellm-auto` in validation, adds a pure testable uninstall planner, wires a CLI command with dry-run/force/confirmation, preserves external runtimes, and documents the behavior.
- Placeholder scan: no TBD/TODO/fill-in-later placeholders remain; router thresholds and bill-to behavior are locked, not deferred.
- Type consistency: public seams are `core.RenderLiteLLMConfig(Secrets)`, `core.UninstallPlan`, `core.PlanUninstall`, and `uninstallCommand`, reusing existing `ProfilePaths`, `Profile`, `Secrets`, and `ServiceManager`.
- Regression note: converting `DefaultLiteLLMConfig` (const) to `RenderLiteLLMConfig(Secrets)` (func) is the one structural change; it is required because a static const can never reflect the user's optional bill-to value. Post-creation the file is a normal editable `config.yaml`, so `models set`/`LoadLiteLLMConfig` round-trips are unaffected.
