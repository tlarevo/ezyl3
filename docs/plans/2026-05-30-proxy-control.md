# Proxy Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move LiteLLM proxy execution out of the generated `run-proxy.sh` artifact and into `ezyl3 proxy run`.

**Architecture:** Setup remains responsible for creating profiles, secrets, config, logs, Python dependencies, and LaunchAgents. A new core proxy runner resolves the runtime, loads `.env`, builds the LiteLLM command, and executes `.venv/bin/litellm` directly. LaunchAgents invoke the installed `ezyl3` binary with `proxy run --profile <name>`.

**Tech Stack:** Go, Cobra CLI, macOS LaunchAgent plists, existing `core` runtime/profile types, standard library process execution.

---

### Task 1: Add Core Proxy Runner

**Files:**
- Create: `internal/core/proxy.go`
- Test: `internal/core/proxy_test.go`
- Modify: `internal/core/secrets.go`

- [ ] **Step 1: Write failing tests for command spec and env loading**

Add tests that create a temp runtime with `config.yaml`, `.env`, and `.venv/bin/litellm`, then assert `BuildProxyCommand` returns:
- working directory set to the runtime path
- binary path set to `.venv/bin/litellm`
- args `--config <config.yaml> --host 127.0.0.1 --port 4400`
- env entries loaded from `.env`, including arbitrary custom keys

Run: `go test ./internal/core -run 'TestBuildProxyCommand|TestReadEnvFile'`
Expected: FAIL because `BuildProxyCommand` and `ReadEnvFile` do not exist.

- [ ] **Step 2: Implement minimal proxy command builder**

Add `ProxyCommandSpec`, `BuildProxyCommand(runtime Runtime)`, and `ReadEnvFile(path string)`. `ReadEnvFile` should reuse the same simple dotenv grammar as `ReadSecrets`: skip blanks/comments, split on first `=`, trim surrounding double quotes, and preserve arbitrary keys.

- [ ] **Step 3: Run targeted tests**

Run: `go test ./internal/core -run 'TestBuildProxyCommand|TestReadEnvFile'`
Expected: PASS.

### Task 2: Add CLI `proxy run`

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`

- [ ] **Step 1: Write failing CLI tests**

Add tests that verify root help exposes `proxy`, and `ezyl3 proxy run --path <runtime>` delegates to a core runner without leaking secrets. Use an injectable package-level runner if needed so tests do not start LiteLLM.

Run: `go test ./internal/cli -run TestProxy`
Expected: FAIL because the command does not exist.

- [ ] **Step 2: Implement command wiring**

Register `proxyCommand(opts)` in `NewRootCommand`. The command resolves `runtimeFromOptions(opts)` and calls `core.RunProxy` with command stdin/stdout/stderr.

- [ ] **Step 3: Run targeted tests**

Run: `go test ./internal/cli -run TestProxy`
Expected: PASS.

### Task 3: Point LaunchAgents At `ezyl3 proxy run`

**Files:**
- Modify: `internal/core/setup.go`
- Modify: `internal/core/install.go`
- Modify: `internal/core/launchagent.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/setupwizard/wizard.go`
- Test: `internal/core/setup_test.go`
- Test: `internal/core/launchagent_test.go`
- Test: `internal/setupwizard/wizard_test.go`

- [ ] **Step 1: Write failing tests**

Update LaunchAgent tests to expect ProgramArguments:
`<absolute ezyl3 path>`, `proxy`, `run`, `--profile`, `<profile>`.
Update setup tests/fakes to assert the executable path is passed to LaunchAgent writing.

Run: `go test ./internal/core ./internal/setupwizard`
Expected: FAIL until signatures and rendering are updated.

- [ ] **Step 2: Add executable path to setup options**

Add `ExecutablePath string` to `core.SetupOptions`. CLI setup and setup wizard should pass `os.Executable()` unless tests provide a value. `RunSetup` should reject an empty executable path only when the default LaunchAgent writer is used; injected test writers can inspect the value.

- [ ] **Step 3: Update LaunchAgent rendering**

Change `LaunchAgentWriter.WriteLaunchAgents` and `WriteLaunchAgents` to accept `executablePath`. Change `RenderLiteLLMPlist(paths, port, executablePath)` to render the direct `ezyl3 proxy run --profile <profile>` invocation.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/core ./internal/setupwizard`
Expected: PASS.

### Task 4: Retire `run-proxy.sh` As a Generated Artifact

**Files:**
- Modify: `internal/core/install.go`
- Modify: `internal/core/runtime.go`
- Modify: `internal/core/setup_test.go`
- Modify: `internal/cli/root_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write/update failing tests**

Update setup tests so a managed profile contains `config.yaml`, `.env`, `profile.json`, and logs, and explicitly assert `run-proxy.sh` is not created. Update doctor tests or add one that verifies missing `run-proxy.sh` is not reported.

Run: `go test ./internal/core ./internal/cli`
Expected: FAIL while the script is still created or checked.

- [ ] **Step 2: Remove script generation and doctor check**

Delete `runProxyScript`, stop writing `run-proxy.sh`, and remove the doctor file check. Add a doctor check for `.venv/bin/litellm` with a clear message when dependencies were skipped or missing.

- [ ] **Step 3: Update README**

Replace troubleshooting text that mentions `run-proxy.sh` with guidance for `config.yaml`, `.env`, `.venv/bin/litellm`, `ezyl3 proxy run`, and `ezyl3 service start`.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/core ./internal/cli`
Expected: PASS.

### Task 5: Final Verification and PR

**Files:**
- All modified files

- [ ] **Step 1: Run full tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 2: Inspect final diff**

Run: `git diff --stat` and `git diff --check`
Expected: no whitespace errors and only proxy-control scoped changes.

- [ ] **Step 3: Commit, push, and open PR**

Commit message: `feat: move proxy execution into ezyl3`
Push branch: `codex/proxy-control`
Open a PR against `main` summarizing `ezyl3 proxy run`, LaunchAgent update, `run-proxy.sh` retirement, and tests.
