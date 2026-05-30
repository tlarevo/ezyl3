package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSecretsUses0600AndRedactsValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	secrets := Secrets{
		HFToken:          "hf_secret",
		HFBillTo:         "eventinc-gmbh",
		OllamaAPIKey:     "ollama.secret",
		LiteLLMMasterKey: "sk-cursor-secret",
	}

	if err := WriteSecrets(path, secrets); err != nil {
		t.Fatalf("WriteSecrets returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 0600", got)
	}

	read, err := ReadSecrets(path)
	if err != nil {
		t.Fatalf("ReadSecrets returned error: %v", err)
	}
	if read.LiteLLMMasterKey != secrets.LiteLLMMasterKey {
		t.Fatalf("master key was not round-tripped")
	}

	display := read.Redacted()
	for _, secret := range []string{secrets.HFToken, secrets.OllamaAPIKey, secrets.LiteLLMMasterKey} {
		if strings.Contains(display, secret) {
			t.Fatalf("redacted display leaked secret %q in %q", secret, display)
		}
	}
	if !strings.Contains(display, "HF_TOKEN=set") || !strings.Contains(display, "HF_BILL_TO=eventinc-gmbh") {
		t.Fatalf("redacted display is not useful: %q", display)
	}
}

func TestReadEnvFilePreservesArbitraryKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("\n# ignored\nHF_TOKEN=\"hf_secret\"\nCUSTOM_FLAG=enabled\nEMPTY=\"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	values, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile returned error: %v", err)
	}

	for _, want := range []string{"HF_TOKEN=hf_secret", "CUSTOM_FLAG=enabled", "EMPTY="} {
		if !containsString(values, want) {
			t.Fatalf("ReadEnvFile missing %q in %#v", want, values)
		}
	}
}
