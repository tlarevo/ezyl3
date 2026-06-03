#!/usr/bin/env bash
#
# Pre-release smoke test for ezyl3.
#
# Exercises the *built binary* against real XDG/HOME paths end-to-end — the
# integration surface that `go test ./...` deliberately fakes. Lifecycle:
#
#   setup  ->  automated CLI smoke checks  ->  interactive TUI (you)  ->  cleanup
#
# Cleanup runs on exit (including when you quit the TUI), so a sandbox HOME under
# a temp dir is always removed. This complements, not replaces, `go test ./...`.
#
# Usage:
#   scripts/smoke-test.sh            # full run, launches the TUI at the end
#   scripts/smoke-test.sh --no-tui   # automated checks only (CI / headless)
#
# Exits non-zero if any automated check fails.

set -uo pipefail   # NOT -e: several checks intentionally expect non-zero exits.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT" || { echo "cannot cd to repo root: $REPO_ROOT" >&2; exit 1; }

LAUNCH_TUI=1
[[ "${1:-}" == "--no-tui" ]] && LAUNCH_TUI=0

SANDBOX="$(mktemp -d "${TMPDIR:-/tmp}/ezyl3-smoke.XXXXXX")"
BIN="$SANDBOX/ezyl3"
FAILURES=0

cleanup() {
  # Defensive guard on a destructive op: only remove a real sandbox path under a
  # temp dir. Never run rm -rf on an empty/unexpected value.
  case "$SANDBOX" in
    */ezyl3-smoke.*)
      echo
      echo "==> Cleaning up sandbox: $SANDBOX"
      rm -rf "$SANDBOX"
      ;;
    *)
      echo "WARNING: refusing to clean unexpected SANDBOX path: '${SANDBOX:-<unset>}'" >&2
      ;;
  esac
}
trap cleanup EXIT

# --- tiny assertion helpers -------------------------------------------------

pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; FAILURES=$((FAILURES + 1)); }

# assert_contains <label> <haystack> <needle>
assert_contains() {
  if [[ "$2" == *"$3"* ]]; then pass "$1"; else
    fail "$1 (missing: $3)"
    printf '       output: %s\n' "$2"
  fi
}

# assert_not_contains <label> <haystack> <needle>
assert_not_contains() {
  if [[ "$2" != *"$3"* ]]; then pass "$1"; else fail "$1 (unexpected: $3)"; fi
}

# fresh isolated HOME for a check; echoes the path
fresh_home() {
  local h="$SANDBOX/home-$1"
  rm -rf "$h"
  mkdir -p "$h"
  echo "$h"
}

section() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }

# --- 0. build + unit tests --------------------------------------------------

section "Build and unit tests"
if go build -o "$BIN" ./cmd/ezyl3; then
  pass "go build"
else
  fail "go build"
  echo "Cannot proceed without a working binary." >&2
  exit 1
fi
if go test ./... >/dev/null 2>&1; then pass "go test ./..."; else fail "go test ./..."; fi

# --- 1. direct exposure (--public-url) --------------------------------------

section "Direct exposure (--public-url)"
H="$(fresh_home direct)"
out="$(HOME="$H" "$BIN" setup --skip-python-deps --public-url https://llm.example.com 2>&1)"
assert_contains "setup succeeds" "$out" "Created managed profile"
assert_contains "summary shows direct exposure" "$out" "Exposure: direct: https://llm.example.com"
assert_contains "summary base URL has single /v1" "$out" "Base URL: https://llm.example.com/v1"

cur="$(HOME="$H" "$BIN" cursor settings 2>&1)"
assert_contains "cursor settings shows public URL" "$cur" "https://llm.example.com/v1"
assert_not_contains "cursor settings has no local-only warning" "$cur" "WARNING"

if [[ -e "$H/Library/LaunchAgents/com.ezyl3.default.ngrok.plist" ]]; then
  fail "direct profile must NOT write an ngrok LaunchAgent"
else
  pass "no ngrok LaunchAgent for direct profile"
fi

# --- 2. public-URL validation (each must fail) ------------------------------

section "Public-URL validation"
H="$(fresh_home validate)"
err="$(HOME="$H" "$BIN" setup --skip-python-deps --public-url http://insecure.com 2>&1)"
assert_contains "rejects http://" "$err" "must be https"
err="$(HOME="$H" "$BIN" setup --skip-python-deps --public-url https://llm.example.com/v1 2>&1)"
assert_contains "rejects path-bearing URL (origin only)" "$err" "origin with no path"
err="$(HOME="$H" "$BIN" setup --skip-python-deps --domain x.ngrok-free.dev --public-url https://y.com 2>&1)"
assert_contains "rejects --domain + --public-url" "$err" "both"

# --- 3. local-only still warns ----------------------------------------------

section "Local-only exposure"
H="$(fresh_home local)"
out="$(HOME="$H" "$BIN" setup --skip-python-deps 2>&1)"
assert_contains "local-only base URL" "$out" "Base URL: http://127.0.0.1:4400/v1"
cur="$(HOME="$H" "$BIN" cursor settings 2>&1)"
assert_contains "local-only warns Cursor cannot use it" "$cur" "Cursor cannot use it"

# --- summary of automated checks --------------------------------------------

section "Automated result"
if [[ "$FAILURES" -gt 0 ]]; then
  printf '\033[31m%d automated check(s) failed.\033[0m\n' "$FAILURES"
  exit 1
fi
printf '\033[32mAll automated checks passed.\033[0m\n'

# --- 4. interactive TUI (human verification) --------------------------------

if [[ "$LAUNCH_TUI" -eq 0 ]]; then
  echo "(skipping interactive TUI: --no-tui)"
  exit 0
fi

section "Interactive TUI — human verification"
cat <<'EOF'
A direct-exposure profile is set up in a sandbox HOME. The TUI will launch next.

On the Cursor tab (press `tab` to reach it, 2nd after Overview), confirm:
  - Base URL is the public URL, model names listed one per line
  - API key shows `set`; press `c` to reveal the clean key, `c` again to hide
  - NO local-only warning, NO ngrok badge (this is a direct profile)
  - keypresses feel instant (no stall)
  - layout/colors look right at your terminal width

Quit the TUI (`q`) when done — the sandbox is cleaned up automatically on exit.
EOF
read -r -p "Press Enter to launch the TUI..." _

H="$(fresh_home tui)"
if ! HOME="$H" "$BIN" setup --skip-python-deps --public-url https://llm.example.com >/dev/null 2>&1; then
  echo "ERROR: setup failed for the TUI profile; not launching the TUI." >&2
  exit 1
fi
HOME="$H" "$BIN" tui
# cleanup() runs on EXIT.
