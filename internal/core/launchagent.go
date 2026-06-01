package core

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const litellmPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{{ .Label }}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{ .ExecutablePath }}</string>
    <string>proxy</string>
    <string>run</string>
    <string>--profile</string>
    <string>{{ .Profile }}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>{{ .OutLog }}</string>
  <key>StandardErrorPath</key>
  <string>{{ .ErrLog }}</string>
</dict>
</plist>
`

const ngrokPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{{ .Label }}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{ .NgrokPath }}</string>
    <string>http</string>
    <string>--url={{ .Domain }}</string>
    <string>http://localhost:{{ .Port }}</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>{{ .OutLog }}</string>
  <key>StandardErrorPath</key>
  <string>{{ .ErrLog }}</string>
</dict>
</plist>
`

func RenderLiteLLMPlist(paths ProfilePaths, port int, executablePath string) (string, error) {
	return renderPlist(litellmPlistTemplate, map[string]any{
		"Label":          paths.LaunchAgentLabel("litellm"),
		"ExecutablePath": executablePath,
		"Profile":        paths.Profile,
		"OutLog":         filepath.Join(paths.LogsDir, "litellm.out.log"),
		"ErrLog":         filepath.Join(paths.LogsDir, "litellm.err.log"),
		"Port":           port,
	})
}

func RenderNgrokPlist(paths ProfilePaths, domain string, port int, ngrokPath string) (string, error) {
	if ngrokPath == "" {
		ngrokPath = ResolveNgrokPath()
	}
	return renderPlist(ngrokPlistTemplate, map[string]any{
		"Label":     paths.LaunchAgentLabel("ngrok"),
		"Domain":    domain,
		"Port":      port,
		"NgrokPath": ngrokPath,
		"OutLog":    filepath.Join(paths.LogsDir, "ngrok.out.log"),
		"ErrLog":    filepath.Join(paths.LogsDir, "ngrok.err.log"),
	})
}

// ResolveNgrokPath returns an absolute path to the ngrok binary so the LaunchAgent
// does not depend on launchd's minimal PATH (which excludes Homebrew). It falls
// back to common install locations, then to the bare name as a last resort.
func ResolveNgrokPath() string {
	if path, err := exec.LookPath("ngrok"); err == nil {
		return path
	}
	for _, candidate := range []string{"/opt/homebrew/bin/ngrok", "/usr/local/bin/ngrok"} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "ngrok"
}

func renderPlist(tpl string, value any) (string, error) {
	parsed, err := template.New("plist").Parse(tpl)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, value); err != nil {
		return "", err
	}
	return out.String(), nil
}
