package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Runtime struct {
	Path    string
	Port    int
	Profile string
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type DoctorReport struct {
	RuntimePath string  `json:"runtime_path"`
	Checks      []Check `json:"checks"`
}

func NewRuntime(path string) Runtime {
	return Runtime{Path: path, Port: 4400, Profile: "default"}
}

func DefaultRuntimePath(env map[string]string, profile string) (string, error) {
	paths, err := ResolvePaths(env, profile)
	if err != nil {
		return "", err
	}
	return paths.ProfileDir, nil
}

func Doctor(runtime Runtime) DoctorReport {
	checks := []Check{
		fileCheck(filepath.Join(runtime.Path, "config.yaml")),
		fileCheck(filepath.Join(runtime.Path, ".env")),
		fileCheck(filepath.Join(runtime.Path, "run-proxy.sh")),
		dirCheck(filepath.Join(runtime.Path, "logs")),
	}
	if secrets, err := ReadSecrets(filepath.Join(runtime.Path, ".env")); err == nil {
		checks = append(checks,
			Check{Name: "OLLAMA_API_KEY", OK: strings.TrimSpace(secrets.OllamaAPIKey) != "", Detail: presence(secrets.OllamaAPIKey)},
			Check{Name: "LITELLM_MASTER_KEY", OK: strings.TrimSpace(secrets.LiteLLMMasterKey) != "", Detail: presence(secrets.LiteLLMMasterKey)},
			Check{Name: "HF_TOKEN", OK: strings.TrimSpace(secrets.HFToken) != "", Detail: presence(secrets.HFToken)},
		)
	}
	checks = append(checks,
		httpCheck("LiteLLM liveliness", fmt.Sprintf("http://127.0.0.1:%d/health/liveliness", runtime.Port), ""),
		httpCheck("ngrok inspector", "http://127.0.0.1:4040/api/tunnels", ""),
	)
	if domain, err := DetectNgrokDomain(runtime.Path); err == nil {
		checks = append(checks, httpCheck("tunnel liveliness", "https://"+domain+"/health/liveliness", "domain: "+domain))
	}
	return DoctorReport{RuntimePath: runtime.Path, Checks: checks}
}

func (r DoctorReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func CursorSettings(runtime Runtime) (string, error) {
	domain, err := DetectNgrokDomain(runtime.Path)
	if err != nil {
		return "", err
	}
	secrets, err := ReadSecrets(filepath.Join(runtime.Path, ".env"))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Base URL: https://%s/v1\nAPI key: %s\nModels: litellm-auto, litellm-simple, litellm-medium, litellm-complex, litellm-reasoning\n", domain, presence(secrets.LiteLLMMasterKey)), nil
}

func DetectNgrokDomain(runtimePath string) (string, error) {
	candidates := []string{
		filepath.Join(runtimePath, "metadata.json"),
		filepath.Join(runtimePath, "logs", "ngrok.out.log"),
		filepath.Join(runtimePath, "logs", "ngrok.err.log"),
	}
	for _, candidate := range candidates {
		if domain, err := domainFromFile(candidate); err == nil {
			return domain, nil
		}
	}
	return "", errors.New("ngrok domain not found")
}

func domainFromFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`https://([a-z0-9][a-z0-9-]*\.ngrok-free\.(dev|app))`)
	for scanner.Scan() {
		if match := re.FindStringSubmatch(scanner.Text()); len(match) > 1 {
			return NormalizeNgrokDomain(match[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("ngrok domain not found")
}

func fileCheck(path string) Check {
	_, err := os.Stat(path)
	return Check{Name: filepath.Base(path), OK: err == nil, Detail: redactPathErr(path, err)}
}

func dirCheck(path string) Check {
	info, err := os.Stat(path)
	return Check{Name: filepath.Base(path), OK: err == nil && info.IsDir(), Detail: redactPathErr(path, err)}
}

func httpCheck(name, target, detail string) Check {
	client := http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(target)
	if err != nil {
		if detail == "" {
			detail = err.Error()
		}
		return Check{Name: name, OK: false, Detail: detail}
	}
	defer resp.Body.Close()
	if detail == "" {
		detail = resp.Status
	}
	return Check{Name: name, OK: resp.StatusCode >= 200 && resp.StatusCode < 300, Detail: detail}
}

func redactPathErr(path string, err error) string {
	if err == nil {
		return path
	}
	return err.Error()
}
