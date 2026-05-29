package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Secrets struct {
	HFToken          string
	HFBillTo         string
	OllamaAPIKey     string
	LiteLLMMasterKey string
}

func WriteSecrets(path string, secrets Secrets) error {
	var b strings.Builder
	writeEnvLine(&b, "HF_TOKEN", secrets.HFToken)
	writeEnvLine(&b, "HF_BILL_TO", secrets.HFBillTo)
	writeEnvLine(&b, "OLLAMA_API_KEY", secrets.OllamaAPIKey)
	writeEnvLine(&b, "LITELLM_MASTER_KEY", secrets.LiteLLMMasterKey)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func ReadSecrets(path string) (Secrets, error) {
	file, err := os.Open(path)
	if err != nil {
		return Secrets{}, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		values[parts[0]] = strings.Trim(parts[1], `"`)
	}
	if err := scanner.Err(); err != nil {
		return Secrets{}, err
	}
	return Secrets{
		HFToken:          values["HF_TOKEN"],
		HFBillTo:         values["HF_BILL_TO"],
		OllamaAPIKey:     values["OLLAMA_API_KEY"],
		LiteLLMMasterKey: values["LITELLM_MASTER_KEY"],
	}, nil
}

func (s Secrets) Redacted() string {
	return fmt.Sprintf("HF_TOKEN=%s\nHF_BILL_TO=%s\nOLLAMA_API_KEY=%s\nLITELLM_MASTER_KEY=%s",
		presence(s.HFToken), visibleOrEmpty(s.HFBillTo), presence(s.OllamaAPIKey), presence(s.LiteLLMMasterKey))
}

func writeEnvLine(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "%s=%q\n", key, value)
}

func presence(value string) string {
	if strings.TrimSpace(value) == "" {
		return "empty"
	}
	return "set"
}

func visibleOrEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "empty"
	}
	return value
}
