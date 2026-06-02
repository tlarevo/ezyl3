# AGENTS.md

Operating guide for coding agents (and humans) working on `ezyl3`. Read this
first. It encodes how work flows from idea to merged code, the conventions to
follow, and the rules that exist because we learned them the hard way.

`ezyl3` is a macOS-only Go CLI + Bubble Tea TUI that manages a local LiteLLM
bridge for Cursor. Audience for all planning docs here is **the maintainer and
the coding agents that implement** — not external stakeholders. Keep specs
engineering-grade and decisive, not product-marketing prose.

## The work pipeline

Every non-trivial change flows through the same path. Do not skip stages for
substantial work; small fixes may start at the branch stage.

```text
PRD            docs/prd/YYYY-MM-DD-<slug>.md      ← the spec: problem + decisions
  ↓  (one PRD → one or more plans)
Plan           docs/plans/YYYY-MM-DD-<slug>.md    ← checkbox task breakdown
  ↓  (one plan → one branch per vertical slice)
Branch + PR    via the worktree flow below        ← implement, verify, review
  ↓
Merge + cleanup
```

- **PRD** (`docs/prd/`): the decision record. Use the format already established
  in `docs/prd/2026-05-30-release-and-lifecycle.md`:
  Problem Statement → Solution → Architecture (mermaid) → User Stories →
  **Implementation Decisions** → **Testing Decisions** → Out of Scope. The two
  Decisions sections are what make a PRD executable by an agent — be decisive
  there, not exploratory.
- **Plan** (`docs/plans/`): the task breakdown, with `- [ ]` checkbox steps an
  agent can execute one at a time. See the existing files in `docs/plans/` for
  the shape. All plans live in `docs/plans/` — there is no second location.
- **PR**: one PR per coherent vertical slice. Tracking lives in GitHub
  (PRs/issues) — do not add a parallel tracking system.

## Branch / commit / PR flow

Agents work in the harness-provided worktree they are spawned in
(`.claude/worktrees/<name>`) using plain `git`. One branch at a time, in place.
The canonical loop:

1. **Branch off the latest `main`**:
   `git fetch origin && git checkout -b <type>/<slug> origin/main`.
   Never branch off another feature branch unless you intend to stack.
2. **Implement** in small commits.
3. **Run the verification gate** (below) — *before every commit*.
4. **Push** and open a PR against `main` (`gh pr create`).
5. **Address review comments**, push fixes, reply on each thread, resolve it.
6. **After merge**, the branch is done; the next task starts from a fresh
   `origin/main`.

Branch naming: `feat/<slug>`, `fix/<slug>`, `chore/<slug>`, `docs/<slug>`.

The primary checkout (wherever the repo is cloned) stays on `main` — never commit
feature work there. The harness worktree is where the agent operates; the primary
is the human's `main` reference.

Commit messages: imperative subject, a body explaining *why* when non-obvious,
and end with:

```text
Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
```

## Verification gate (run before every commit)

This is non-negotiable. Run all four and confirm each is clean. **Never commit
on red** — a failing test or unformatted file must be fixed first, not committed
and fixed later.

```bash
go build ./...
go test ./...
gofmt -l .        # must print nothing under project source (internal/, scripts/, cmd/)
go vet ./...
```

CI runs `gofmt -l`, `go vet`, `go test`, and `goreleaser check` on every PR, so
red here means red there. Running the full suite locally first is faster than a
CI round-trip.

## Hard-won rules

These exist because skipping them caused real breakage:

- **Confirm a review fix is in the merge, not just pushed.** A PR can be merged
  at any commit. If you push a fix to a branch that then merges at an earlier
  commit, the fix is stranded on a dead branch and never reaches `main`. After a
  fix is "done", verify it with `git merge-base --is-ancestor <sha> origin/main`.
- **Know which worktree holds `main`.** `main` is checked out in the primary
  worktree; `git checkout main` from another worktree fails. Always branch off
  `origin/main` explicitly rather than assuming the current checkout.
- **Run the full suite before committing, then read the output.** Two premature
  commits in this project's history shipped on a failing test that the author
  glossed over. Read the gate output; do not assume green.
- **TUI rendering cannot be verified headlessly.** Model/state-layer logic is
  unit-testable, but the Bubble Tea alt-screen is not. After TUI changes, a human
  must eyeball `ezyl3 tui` (or the setup wizard) in a real terminal.
- **Build artifacts are not source.** `dist/` (GoReleaser snapshot output) and
  the `.go*cache/` dirs are gitignored; never commit them.

## Project conventions

- **macOS only.** Do not add Linux/Windows code paths or service managers.
- **Inject side effects for testability.** External commands, `launchctl`, log
  following, ngrok checks, and the Python installer all sit behind interfaces so
  tests use fakes instead of touching the system. Follow this pattern for any new
  side-effectful code (see `core.ServiceManager`, `core.NgrokChecker`,
  `core.UVToolchain`, `core.LaunchAgentWriter`).
- **Never print secrets.** Setup summaries, `doctor`, and `cursor settings`
  redact API keys to `set`/`empty` by default. Revealing a secret must be an
  explicit, opt-in action (e.g. `cursor settings --reveal-key`). Tests assert no
  secret leaks.
- **External commands get a timeout.** Use `exec.CommandContext` with a bounded
  context, matching `core.UVToolchain` and `core.DefaultNgrokChecker`.
- **Cursor needs a public HTTPS tunnel.** A local-only profile cannot serve
  Cursor (it rejects localhost). Keep local-only first-class but warn when it is
  used for Cursor.

## Quick reference

```bash
go run ./cmd/ezyl3 --help          # explore the CLI
go run ./cmd/ezyl3 version         # build version (dev from source)
go test ./...                      # full suite
goreleaser check                   # validate release config (if installed)
```
