package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	ServiceStateLoaded         = "loaded"
	ServiceStateRunning        = "running"
	ServiceStateStopped        = "stopped"
	ServiceStateMissing        = "missing"
	ServiceStateNotConfigured  = "not configured"
	ServiceStateUnknown        = "unknown"
	ServiceStateAlreadyRunning = "already running"
)

type ServiceRunner interface {
	RunLaunchctl(args ...string) (string, error)
}

type LaunchctlRunner struct{}

func (LaunchctlRunner) RunLaunchctl(args ...string) (string, error) {
	cmd := exec.Command("launchctl", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type ServiceManager struct {
	Paths  ProfilePaths
	Runner ServiceRunner
	UID    int
}

type ServiceActionResult struct {
	Service string
	Action  string
	State   string
	Detail  string
}

func (m ServiceManager) Start() ([]ServiceActionResult, error) {
	return m.run("start")
}

func (m ServiceManager) Stop() ([]ServiceActionResult, error) {
	return m.run("stop")
}

func (m ServiceManager) StopServices(services []string) ([]ServiceActionResult, error) {
	return m.runServices("stop", services, false)
}

func (m ServiceManager) Restart() ([]ServiceActionResult, error) {
	return m.run("restart")
}

func (m ServiceManager) Status() ([]ServiceActionResult, error) {
	return m.run("status")
}

func (m ServiceManager) run(action string) ([]ServiceActionResult, error) {
	return m.runServices(action, []string{"litellm", "ngrok"}, true)
}

func (m ServiceManager) runServices(action string, services []string, allowMissingOptionalNgrok bool) ([]ServiceActionResult, error) {
	runner := m.Runner
	if runner == nil {
		runner = LaunchctlRunner{}
	}
	uid := m.UID
	if uid == 0 {
		uid = os.Getuid()
	}
	results := []ServiceActionResult{}
	for _, service := range services {
		plist := m.Paths.LaunchAgentPath(service)
		if _, err := os.Stat(plist); err != nil {
			state := ServiceStateMissing
			detail := "missing LaunchAgent: " + plist
			if allowMissingOptionalNgrok && service == "ngrok" && os.IsNotExist(err) {
				state = ServiceStateNotConfigured
				detail = "ngrok LaunchAgent is not configured for this profile"
			}
			result := ServiceActionResult{Service: service, Action: action, State: state, Detail: detail}
			results = append(results, result)
			if !allowMissingOptionalNgrok || service == "litellm" || !os.IsNotExist(err) {
				return results, fmt.Errorf("%s %s: %s", action, service, detail)
			}
			continue
		}
		// Starting a service that launchd has already bootstrapped is not a
		// failure: report it as already running and continue so one loaded
		// service does not abort the rest (e.g. leaving ngrok unstarted).
		if action == "start" && m.isBootstrapped(runner, service, uid) {
			results = append(results, ServiceActionResult{
				Service: service,
				Action:  action,
				State:   ServiceStateAlreadyRunning,
				Detail:  "already bootstrapped; leaving running",
			})
			continue
		}
		args := serviceCommandArgs(action, m.Paths, service, uid)
		out, err := runner.RunLaunchctl(args...)
		if err != nil {
			return results, fmt.Errorf("%s %s: %w", action, service, err)
		}
		state := ServiceStateLoaded
		if action == "status" {
			state = parseLaunchctlState(out)
		}
		results = append(results, ServiceActionResult{
			Service: service,
			Action:  action,
			State:   state,
			Detail:  strings.TrimSpace(out),
		})
	}
	return results, nil
}

// isBootstrapped reports whether launchd already has the service loaded in this
// domain. A successful `launchctl print` (regardless of run state) means the
// service is loaded and a re-bootstrap would fail with EIO (exit 5). A
// loaded-but-stopped job (waiting/exited/not running) is still bootstrapped, so
// we must not gate on the parsed run state here.
func (m ServiceManager) isBootstrapped(runner ServiceRunner, service string, uid int) bool {
	_, err := runner.RunLaunchctl(serviceCommandArgs("status", m.Paths, service, uid)...)
	return err == nil
}

func serviceCommandArgs(action string, paths ProfilePaths, service string, uid int) []string {
	gui := fmt.Sprintf("gui/%d", uid)
	switch action {
	case "start":
		return []string{"bootstrap", gui, paths.LaunchAgentPath(service)}
	case "stop":
		return []string{"bootout", gui, paths.LaunchAgentPath(service)}
	case "restart":
		return []string{"kickstart", "-k", gui + "/" + paths.LaunchAgentLabel(service)}
	case "status":
		return []string{"print", gui + "/" + paths.LaunchAgentLabel(service)}
	default:
		return []string{action}
	}
}

func parseLaunchctlState(out string) string {
	lower := strings.ToLower(out)
	switch {
	case strings.Contains(lower, "state = running"):
		return ServiceStateRunning
	case strings.Contains(lower, "state = waiting"), strings.Contains(lower, "state = exited"), strings.Contains(lower, "state = not running"):
		return ServiceStateStopped
	case strings.TrimSpace(out) != "":
		return ServiceStateLoaded
	default:
		return ServiceStateUnknown
	}
}
