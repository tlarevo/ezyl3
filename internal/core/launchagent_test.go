package core

import (
	"strings"
	"testing"
)

func TestLaunchAgentPlistsUseProfileSpecificLabelsAndPaths(t *testing.T) {
	paths := ProfilePaths{
		Profile:    "default",
		ProfileDir: "/tmp/ezyl3/profile",
		LogsDir:    "/tmp/ezyl3/state/logs",
	}

	litellm, err := RenderLiteLLMPlist(paths, 4400, "/usr/local/bin/ezyl3")
	if err != nil {
		t.Fatalf("RenderLiteLLMPlist returned error: %v", err)
	}
	ngrok, err := RenderNgrokPlist(paths, "example.ngrok-free.dev", 4400)
	if err != nil {
		t.Fatalf("RenderNgrokPlist returned error: %v", err)
	}

	for _, pair := range []struct {
		name string
		text string
		want []string
	}{
		{"litellm", litellm, []string{"com.ezyl3.default.litellm", "/usr/local/bin/ezyl3", "proxy", "run", "--profile", "default", "/tmp/ezyl3/state/logs/litellm.out.log"}},
		{"ngrok", ngrok, []string{"com.ezyl3.default.ngrok", "--url=example.ngrok-free.dev", "http://localhost:4400", "/tmp/ezyl3/state/logs/ngrok.out.log"}},
	} {
		for _, want := range pair.want {
			if !strings.Contains(pair.text, want) {
				t.Fatalf("%s plist missing %q:\n%s", pair.name, want, pair.text)
			}
		}
	}
}
