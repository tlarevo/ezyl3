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
	items, err := readEnvValues(path)
	if err != nil {
		return Secrets{}, err
	}
	values := map[string]string{}
	for _, item := range items {
		values[item.key] = item.value
	}
	return Secrets{
		HFToken:          values["HF_TOKEN"],
		HFBillTo:         values["HF_BILL_TO"],
		OllamaAPIKey:     values["OLLAMA_API_KEY"],
		LiteLLMMasterKey: values["LITELLM_MASTER_KEY"],
	}, nil
}

func ReadEnvFile(path string) ([]string, error) {
	items, err := readEnvValues(path)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.key+"="+item.value)
	}
	return values, nil
}

type envValue struct {
	key   string
	value string
}

func readEnvValues(path string) ([]envValue, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := []envValue{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		values = append(values, envValue{key: parts[0], value: strings.Trim(parts[1], `"`)})
	}
	return values, scanner.Err()
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
