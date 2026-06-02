package core

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const ngrokCheckTimeout = 10 * time.Second

var ngrokDomainPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.ngrok-free\.(dev|app)$`)

// ngrokSetupHelp is the actionable guidance shown when ngrok is not ready. It is
// shared by setup preflight and doctor so the instructions stay consistent.
const ngrokSetupHelp = `ngrok is required for a tunneled (Cursor-ready) profile. Set it up first:
  brew install --cask ngrok
  ngrok config add-authtoken <token>   (from https://dashboard.ngrok.com/get-started/your-authtoken)
  then claim a free domain at https://dashboard.ngrok.com/domains`

// NgrokChecker verifies that ngrok is installed and authenticated. It is injected
// into setup so the readiness check can be faked in tests without invoking ngrok.
type NgrokChecker interface {
	// CheckNgrokReady returns nil when ngrok is usable for a tunnel, or an error
	// whose message explains what the user must do.
	CheckNgrokReady() error
}

// DefaultNgrokChecker checks for the ngrok binary on PATH and a valid ngrok
// configuration (which is created by `ngrok config add-authtoken`).
type DefaultNgrokChecker struct{}

func (DefaultNgrokChecker) CheckNgrokReady() error {
	if _, err := exec.LookPath("ngrok"); err != nil {
		return fmt.Errorf("ngrok binary not found on PATH.\n%s", ngrokSetupHelp)
	}
	ctx, cancel := context.WithTimeout(context.Background(), ngrokCheckTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ngrok", "config", "check").CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("ngrok config check timed out after %s; ensure ngrok is responsive.\n%s", ngrokCheckTimeout, ngrokSetupHelp)
		}
		return fmt.Errorf("ngrok is installed but not configured (%s).\n%s", strings.TrimSpace(string(out)), ngrokSetupHelp)
	}
	return nil
}

func NormalizeNgrokDomain(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", fmt.Errorf("ngrok domain is required")
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		value = parsed.Host
	}
	value = strings.TrimSuffix(value, "/")
	value = strings.ToLower(value)
	if !ngrokDomainPattern.MatchString(value) {
		return "", fmt.Errorf("invalid ngrok free domain: %s", input)
	}
	return value, nil
}
