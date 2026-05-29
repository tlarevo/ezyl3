# ezyl3

`ezyl3` manages a local LiteLLM bridge for Cursor on macOS. It can create a managed runtime, import an existing LiteLLM runtime as an external profile, write LaunchAgents, show doctor checks, print Cursor settings, manage model tiers, and inspect logs.

## Build and Run From Source

Requirements:

- macOS
- Go 1.24 or newer
- Python 3.12 or `python3`
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

## Managed Setup

Create the default managed profile:

```bash
./ezyl3 setup
```

`setup` is interactive when it is attached to a terminal. It asks for secrets without echoing them, lets you leave ngrok blank for local-only setup, generates a LiteLLM master key when you do not provide one, writes runtime files, and prints a redacted summary.

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
- Logs: `~/.local/state/ezyl3/profiles/default/logs`
- LaunchAgents: `~/Library/LaunchAgents/com.ezyl3.default.*.plist`

## External Runtime Import

If you already have a LiteLLM runtime, import it as metadata:

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

## Services and Logs

Manage LaunchAgents:

```bash
./ezyl3 service status
./ezyl3 service start
./ezyl3 service stop
./ezyl3 service restart
```

Local-only profiles do not need an ngrok LaunchAgent. Service commands report ngrok as not configured instead of treating it as a setup failure.

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

The TUI shows setup overview, profile mode, services, model tiers, doctor checks, and recent LiteLLM log output. In the Services view, press `s` to start, `x` to stop, and `k` to restart services.

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

- Missing `config.yaml`, `.env`, or `run-proxy.sh`: run `./ezyl3 setup` or import an existing runtime.
- LiteLLM is not reachable: run `./ezyl3 service start`, then inspect `./ezyl3 logs litellm`.
- ngrok is not reachable: confirm you provided a valid `*.ngrok-free.dev` or `*.ngrok-free.app` domain. Ignore this for local-only profiles.
- Cursor cannot connect: re-run `./ezyl3 cursor settings` and confirm Cursor uses the printed base URL and model names.
- Model validation fails: run `./ezyl3 models validate`; the error identifies the missing or broken tier.

## Secrets

Prompt mode is the safest setup path because secrets are not echoed and do not enter shell history. Scripted flags remain available for automation, but prefer environment variables and avoid committing shell scripts that contain real tokens.

`ezyl3` writes secrets to `.env` with `0600` permissions. Product output, doctor output, Cursor settings, and setup summaries report secret presence as `set` or `empty`; they do not print API keys or generated LiteLLM master keys.
