package core

import (
	"os"
	"strings"
)

// UninstallPlan describes what removing a profile would do. It is computed by
// PlanUninstall without performing any deletion or service call, so the decision
// logic can be inspected and tested independently of the filesystem and launchctl.
type UninstallPlan struct {
	// Profile is the profile name the plan targets.
	Profile string
	// Mode is the profile mode, "managed" or "external".
	Mode string
	// Services lists the services whose LaunchAgents exist and should be stopped
	// before removal, in stop order.
	Services []string
	// RemovePaths lists the filesystem paths the plan would remove. For external
	// profiles this never includes the imported runtime directory.
	RemovePaths []string
	// ExternalRuntimeDir records the imported runtime path for an external profile
	// so it can be shown as preserved. It is empty for managed profiles and is
	// never part of RemovePaths.
	ExternalRuntimeDir string
}

// PlanUninstall computes the removal plan for a profile. Managed profiles plan
// removal of their profile directory, logs directory, and any LaunchAgents ezyl3
// wrote. External (imported) profiles plan removal only of ezyl3's own metadata
// directory and LaunchAgents; the imported runtime files are never removed. The
// shared cache directory is intentionally left untouched because it may be used by
// other profiles.
func PlanUninstall(paths ProfilePaths, profile Profile) UninstallPlan {
	mode := strings.TrimSpace(profile.Mode)
	if mode == "" {
		mode = ProfileModeManaged
	}

	plan := UninstallPlan{Profile: paths.Profile, Mode: mode}

	seen := map[string]bool{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		plan.RemovePaths = append(plan.RemovePaths, path)
	}

	add(paths.ProfileDir)
	if mode == ProfileModeExternal {
		plan.ExternalRuntimeDir = strings.TrimSpace(profile.RuntimeDir)
	} else {
		add(paths.LogsDir)
	}
	for _, service := range []string{"litellm", "ngrok"} {
		launchAgentPath := paths.LaunchAgentPath(service)
		if _, err := os.Stat(launchAgentPath); err == nil {
			plan.Services = append(plan.Services, service)
			add(launchAgentPath)
		}
	}

	return plan
}
