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
		litellmBinaryCheck(runtime.Path),
		dirCheck("logs", RuntimeLogsDir(runtime)),
	}
	if secrets, err := ReadSecrets(filepath.Join(runtime.Path, ".env")); err == nil {
		checks = append(checks,
			Check{Name: "OLLAMA_API_KEY", OK: strings.TrimSpace(secrets.OllamaAPIKey) != "", Detail: presence(secrets.OllamaAPIKey)},
			Check{Name: "LITELLM_MASTER_KEY", OK: strings.TrimSpace(secrets.LiteLLMMasterKey) != "", Detail: presence(secrets.LiteLLMMasterKey)},
			Check{Name: "HF_TOKEN", OK: strings.TrimSpace(secrets.HFToken) != "", Detail: presence(secrets.HFToken)},
		)
	}
	checks = append(checks,
		httpCheck("LiteLLM liveliness", fmt.Sprintf("http://127.0.0.1:%d/health/liveliness", runtime.Port), "LiteLLM is not reachable; run ezyl3 service start or inspect ezyl3 logs litellm"),
	)

	// Exposure-specific reachability. Cursor needs a public HTTPS base URL — via
	// an ngrok tunnel (--domain) or a public HTTPS endpoint reaching the proxy
	// (--public-url). Branch on the profile's exposure mode.
	profile, _ := LoadProfileFromRuntime(runtime.Path)
	switch profile.Exposure() {
	case ExposureDirect:
		checks = append(checks, httpCheck("public endpoint",
			strings.TrimRight(profile.PublicURL, "/")+"/health/liveliness",
			"public: "+profile.PublicURL))
	case ExposureTunnel:
		checks = append(checks,
			httpCheck("ngrok inspector", "http://127.0.0.1:4040/api/tunnels", "ngrok is not reachable; check ezyl3 logs ngrok"),
		)
		ngrokReady := Check{Name: "ngrok ready", OK: true, Detail: "installed and configured"}
		if readyErr := (DefaultNgrokChecker{}).CheckNgrokReady(); readyErr != nil {
			ngrokReady.OK = false
			ngrokReady.Detail = firstLine(readyErr.Error())
		}
		checks = append(checks, ngrokReady)
		if domain, err := DetectNgrokDomain(runtime.Path); err == nil {
			checks = append(checks, httpCheck("tunnel liveliness", "https://"+domain+"/health/liveliness", "domain: "+domain))
		}
	default:
		// local-only: nothing public to probe. Cursor cannot use this profile.
		checks = append(checks, Check{Name: "exposure", OK: false,
			Detail: "local-only: Cursor needs public HTTPS — set up a tunnel (--domain) or a public endpoint (--public-url)"})
	}
	return DoctorReport{RuntimePath: runtime.Path, Checks: checks}
}

// firstLine returns the first line of s, keeping multi-line guidance out of the
// single-line doctor table; full instructions are shown by setup itself.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func RuntimeLogsDir(runtime Runtime) string {
	if profile, err := LoadProfileFromRuntime(runtime.Path); err == nil && strings.TrimSpace(profile.LogsDir) != "" {
		return profile.LogsDir
	}
	return filepath.Join(runtime.Path, "logs")
}

func (r DoctorReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// CursorModelNames are the model names a user selects in Cursor, in display
// order. litellm-auto is the complexity router; the rest pin a tier.
var CursorModelNames = []string{
	"litellm-auto", "litellm-simple", "litellm-medium", "litellm-complex", "litellm-reasoning",
}

// CursorInfo is the structured source of truth for what a user needs to connect
// Cursor. MasterKey is the clean, unquoted key; callers decide whether to redact.
type CursorInfo struct {
	BaseURL   string
	MasterKey string
	LocalOnly bool
	Models    []string
}

// CursorSettingsInfo resolves the Cursor connection details for a runtime. It is
// the single source consumed by both the CLI (CursorSettings) and the TUI.
func CursorSettingsInfo(runtime Runtime) (CursorInfo, error) {
	// Base URL precedence: a recorded public URL (direct exposure) wins over an
	// ngrok tunnel, which wins over local-only.
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", runtime.Port)
	localOnly := true
	if profile, err := LoadProfileFromRuntime(runtime.Path); err == nil && strings.TrimSpace(profile.PublicURL) != "" {
		baseURL = strings.TrimRight(profile.PublicURL, "/") + "/v1"
		localOnly = false
	} else if domain, err := DetectNgrokDomain(runtime.Path); err == nil {
		baseURL = "https://" + domain + "/v1"
		localOnly = false
	}
	secrets, err := ReadSecrets(filepath.Join(runtime.Path, ".env"))
	if err != nil {
		return CursorInfo{}, err
	}
	return CursorInfo{
		BaseURL:   baseURL,
		MasterKey: strings.TrimSpace(secrets.LiteLLMMasterKey),
		LocalOnly: localOnly,
		Models:    CursorModelNames,
	}, nil
}

func CursorSettings(runtime Runtime, reveal bool) (string, error) {
	info, err := CursorSettingsInfo(runtime)
	if err != nil {
		return "", err
	}
	apiKey := presence(info.MasterKey)
	if reveal {
		apiKey = info.MasterKey
	}
	out := fmt.Sprintf("Base URL: %s\nAPI key: %s\nModels: %s\n", info.BaseURL, apiKey, strings.Join(info.Models, ", "))
	if info.LocalOnly {
		out += "\nWARNING: This is a local-only base URL. Cursor cannot use it: Cursor's\n" +
			"backend rejects localhost/private addresses (it requires a public HTTPS\n" +
			"target). Configure an ngrok tunnel with:\n" +
			"  ezyl3 setup --force --domain <name>.ngrok-free.dev\n"
	}
	return out, nil
}

func DetectNgrokDomain(runtimePath string) (string, error) {
	if profile, err := LoadProfileFromRuntime(runtimePath); err == nil && strings.TrimSpace(profile.Domain) != "" {
		return NormalizeNgrokDomain(profile.Domain)
	}
	candidates := []string{
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
	return Check{Name: filepath.Base(path), OK: err == nil, Detail: setupFileDetail(filepath.Base(path), err)}
}

func litellmBinaryCheck(runtimePath string) Check {
	path := filepath.Join(runtimePath, ".venv", "bin", "litellm")
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		return Check{Name: "litellm binary", OK: true, Detail: "found"}
	}
	if err == nil {
		return Check{Name: "litellm binary", OK: false, Detail: ".venv/bin/litellm is a directory; rerun ezyl3 setup --force"}
	}
	if os.IsNotExist(err) {
		return Check{Name: "litellm binary", OK: false, Detail: "missing .venv/bin/litellm; rerun ezyl3 setup without --skip-python-deps to recreate the managed environment"}
	}
	return Check{Name: "litellm binary", OK: false, Detail: err.Error()}
}

func dirCheck(name, path string) Check {
	info, err := os.Stat(path)
	ok := err == nil && info.IsDir()
	if ok {
		return Check{Name: name, OK: true, Detail: "found"}
	}
	if err == nil {
		return Check{Name: name, OK: false, Detail: "not a directory; run ezyl3 setup --force to recreate the managed profile"}
	}
	return Check{Name: name, OK: false, Detail: setupFileDetail(name, err)}
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

func setupFileDetail(name string, err error) string {
	if err == nil {
		return "found"
	}
	if os.IsNotExist(err) {
		return fmt.Sprintf("missing %s; run ezyl3 setup to create a managed profile or ezyl3 import <runtime-path>", name)
	}
	return err.Error()
}
