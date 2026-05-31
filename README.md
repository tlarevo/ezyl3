# ezyl3

`ezyl3` manages a local LiteLLM bridge for Cursor on macOS. It can create a managed runtime, import an existing LiteLLM runtime as an external profile, write LaunchAgents, show doctor checks, print Cursor settings, manage model tiers, and inspect logs.

## Install

```bash
brew install tlarevo/tap/ezyl3
```

Released binaries report their tag with:

```bash
ezyl3 version
```

## Build From Source

Requirements:

- macOS
- Go 1.24 or newer
- `ngrok`, only if you want a public tunnel

Build:

```bash
go build ./cmd/ezyl3
```

Run from source:

```bash
go run ./cmd/ezyl3 --help
```

Run tests:

```bash
go test ./...
```

## Releasing

Tagged releases use GoReleaser for GitHub artifacts and a separate formula
rendering step for `tlarevo/homebrew-tap`. The repository secret
`HOMEBREW_TAP_TOKEN` must have contents write access to the tap repository.

## Local Testing

Use direct Go commands for day-to-day development. Brew is for package and
release validation, not the normal development loop.

```bash
go test ./...
go run ./cmd/ezyl3 --help
go run ./cmd/ezyl3 version
go build -o /tmp/ezyl3 ./cmd/ezyl3
/tmp/ezyl3 version
```

Use a GoReleaser snapshot to test the release artifact shape before involving
Homebrew:

```bash
goreleaser release --snapshot --clean --skip=publish
find dist -type f -name ezyl3 -perm +111
```

Run setup smoke tests with isolated XDG paths so local checks do not touch your
real profile:

```bash
HOME=/tmp/ezyl3-smoke \
XDG_DATA_HOME=/tmp/ezyl3-smoke/data \
XDG_STATE_HOME=/tmp/ezyl3-smoke/state \
XDG_CONFIG_HOME=/tmp/ezyl3-smoke/config \
XDG_CACHE_HOME=/tmp/ezyl3-smoke/cache \
/tmp/ezyl3 setup --skip-python-deps --force
```

Use Homebrew only when validating packaging and release delivery:

```bash
brew install --formula ./Formula/ezyl3.rb
ezyl3 version
brew uninstall ezyl3
```

After a real release, validate the public tap path:

```bash
brew tap tlarevo/tap
brew install tlarevo/tap/ezyl3
ezyl3 version
brew uninstall ezyl3
```

## Managed Setup

Create the default managed profile:

```bash
./ezyl3 setup
```

`setup` opens a guided terminal wizard when it is attached to a terminal. The wizard uses masked inputs for secrets, lets you leave ngrok blank for local-only setup, shows a review step before writing files, runs setup with progress feedback, generates a LiteLLM master key when you do not provide one, and finishes with a redacted Cursor-ready summary.

Managed setup is self-contained: it uses `uv` to create the profile virtual environment and install LiteLLM without modifying Homebrew, pyenv, mise, shell profiles, or your global Python. If a usable `uv` is on `PATH`, ezyl3 uses it. Otherwise ezyl3 downloads a pinned `uv` release into its own cache after verifying the release checksum, then uses uv-managed Python 3.13 for the profile.

For scripted setup:

```bash
./ezyl3 setup \
  --skip-python-deps \
  --hf-token "$HF_TOKEN" \
  --ollama-api-key "$OLLAMA_API_KEY" \
  --domain "example.ngrok-free.dev"
```

Use `--force` to overwrite an existing managed profile:

```bash
./ezyl3 setup --force
```

Managed profile files are written under the XDG data/state directories, usually:

- Runtime: `~/.local/share/ezyl3/profiles/default`
- Profile descriptor: `~/.local/share/ezyl3/profiles/default/profile.json`
- Usage database: `~/.local/share/ezyl3/profiles/default/usage.sqlite`
- Logs: `~/.local/state/ezyl3/profiles/default/logs`
- Bootstrap tooling and Python cache: `~/.cache/ezyl3`
- LaunchAgents: `~/Library/LaunchAgents/com.ezyl3.default.*.plist`

Sample profile descriptors live in `docs/examples/`:

- `profile.managed-local.json`
- `profile.managed-ngrok.json`
- `profile.external.json`

`profile.json` supports these fields:

- `name`: profile name used with `--profile`
- `mode`: `managed` or `external`
- `runtime_dir`: LiteLLM runtime directory
- `logs_dir`: managed profile log directory, when logs live outside the runtime directory
- `port`: LiteLLM proxy port
- `tunnel_provider`: currently `ngrok`
- `domain`: optional ngrok domain for tunneled profiles

## External Runtime Import

If you already have a LiteLLM runtime, import it as an external profile:

```bash
./ezyl3 import ~/litellm-cursor
```

External profiles are read-only for installation steps. `ezyl3` records where the runtime lives, but does not rewrite its `config.yaml`, `.env`, scripts, or logs unless you run a command that explicitly targets that external path.

## Cursor Settings

Print the settings Cursor needs:

```bash
./ezyl3 cursor settings
```

Use the printed base URL and model names in Cursor. The API key line only reports whether the LiteLLM master key is set; it does not print the key.

Common model names:

- `litellm-auto`
- `litellm-simple`
- `litellm-medium`
- `litellm-complex`
- `litellm-reasoning`

`litellm-auto` is a LiteLLM complexity router: it scores each request and routes it
to the `simple`, `medium`, `complex`, or `reasoning` tier automatically, defaulting
to `medium`. Select `litellm-auto` in Cursor unless you want to pin a specific tier.

If you set a Hugging Face billing org during setup (`--hf-bill-to` or the wizard
bill-to field), managed setup adds an `X-HF-Bill-To` header to Hugging Face requests
so usage bills to that org. Leaving it blank omits the header entirely.

## Services and Logs

Manage LaunchAgents:

```bash
./ezyl3 service status
./ezyl3 service start
./ezyl3 service stop
./ezyl3 service restart
```

Local-only profiles do not need an ngrok LaunchAgent. Service commands report ngrok as not configured instead of treating it as a setup failure.

Run the proxy directly when debugging LaunchAgent issues:

```bash
./ezyl3 proxy run
```

The proxy runner loads `.env` from the selected runtime and starts `.venv/bin/litellm` with the managed `config.yaml`.

Show local usage recorded by the managed proxy:

```bash
./ezyl3 usage summary
./ezyl3 usage summary --days 7
./ezyl3 usage summary --json
```

Managed profiles record request counts, token counts, model/provider breakdowns, and best-effort estimated spend in `usage.sqlite`. Prompt and response bodies are not stored.

Read logs:

```bash
./ezyl3 logs litellm
./ezyl3 logs ngrok
./ezyl3 logs litellm --follow
```

Open the setup companion TUI:

```bash
./ezyl3 tui
```

The TUI shows setup overview, local usage, profile mode, services, model tiers, doctor checks, and recent LiteLLM log output. In the Services view, press `s` to start, `x` to stop, and `k` to restart services.

## Version

Print the build version:

```bash
./ezyl3 version
```

A binary built from source reports `dev`; released binaries report their tag.

## Uninstall

Remove a managed profile and everything `ezyl3` created for it:

```bash
./ezyl3 uninstall --dry-run
./ezyl3 uninstall
./ezyl3 uninstall --force
```

`uninstall` stops the profile's services, then removes the managed profile
directory, its logs directory, and the LaunchAgents `ezyl3` wrote. Use `--dry-run`
to print exactly what would be removed without changing anything. `uninstall`
prompts for confirmation when run interactively; pass `--force` to skip the prompt
for scripts.

External (imported) profiles are treated as read-only: `uninstall` removes only
`ezyl3`'s own profile metadata and LaunchAgents and never deletes the imported
runtime files. The shared `~/.cache/ezyl3` directory is also left in place because
other profiles may use it.

## Troubleshooting

Run doctor first:

```bash
./ezyl3 doctor
```

Use JSON for automation:

```bash
./ezyl3 doctor --json
```

Common fixes:

- Missing `config.yaml`, `.env`, or `.venv/bin/litellm`: run `./ezyl3 setup` without `--skip-python-deps`, or import an existing runtime.
- Proxy exits immediately: run `./ezyl3 proxy run` to see the LiteLLM startup error in the foreground.
- LiteLLM is not reachable: run `./ezyl3 service start`, then inspect `./ezyl3 logs litellm`.
- ngrok is not reachable: confirm you provided a valid `*.ngrok-free.dev` or `*.ngrok-free.app` domain. Ignore this for local-only profiles.
- Cursor cannot connect: re-run `./ezyl3 cursor settings` and confirm Cursor uses the printed base URL and model names.
- Model validation fails: run `./ezyl3 models validate`; the error identifies the missing or broken tier.

## Secrets

Prompt mode is the safest setup path because secrets are not echoed and do not enter shell history. Scripted flags remain available for automation, but prefer environment variables and avoid committing shell scripts that contain real tokens.

`ezyl3` writes secrets to `.env` with `0600` permissions. Product output, doctor output, Cursor settings, and setup summaries report secret presence as `set` or `empty`; they do not print API keys or generated LiteLLM master keys.
