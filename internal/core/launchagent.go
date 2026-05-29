package core

import (
	"bytes"
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
    <string>{{ .RunProxy }}</string>
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
    <string>ngrok</string>
    <string>http</string>
    <string>--url={{ .Domain }}</string>
    <string>http://localhost:{{ .Port }}</string>
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

func RenderLiteLLMPlist(paths ProfilePaths, port int) (string, error) {
	return renderPlist(litellmPlistTemplate, map[string]any{
		"Label":    paths.LaunchAgentLabel("litellm"),
		"RunProxy": filepath.Join(paths.ProfileDir, "run-proxy.sh"),
		"OutLog":   filepath.Join(paths.LogsDir, "litellm.out.log"),
		"ErrLog":   filepath.Join(paths.LogsDir, "litellm.err.log"),
		"Port":     port,
	})
}

func RenderNgrokPlist(paths ProfilePaths, domain string, port int) (string, error) {
	return renderPlist(ngrokPlistTemplate, map[string]any{
		"Label":  paths.LaunchAgentLabel("ngrok"),
		"Domain": domain,
		"Port":   port,
		"OutLog": filepath.Join(paths.LogsDir, "ngrok.out.log"),
		"ErrLog": filepath.Join(paths.LogsDir, "ngrok.err.log"),
	})
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
