package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ezyl3/internal/core"
	"ezyl3/internal/setupwizard"
	"ezyl3/internal/tui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type options struct {
	profile string
	path    string
	json    bool
}

var proxyRunner = core.RunProxy

func NewRootCommand() *cobra.Command {
	opts := &options{profile: "default"}
	cmd := &cobra.Command{
		Use:   "ezyl3",
		Short: "Manage a LiteLLM Cursor bridge",
	}
	cmd.PersistentFlags().StringVar(&opts.profile, "profile", "default", "profile name")
	cmd.PersistentFlags().StringVar(&opts.path, "path", "", "existing runtime path")

	cmd.AddCommand(doctorCommand(opts))
	cmd.AddCommand(cursorCommand(opts))
	cmd.AddCommand(modelsCommand(opts))
	cmd.AddCommand(importCommand(opts))
	cmd.AddCommand(setupCommand(opts))
	cmd.AddCommand(serviceCommand(opts))
	cmd.AddCommand(logsCommand(opts))
	cmd.AddCommand(proxyCommand(opts))
	cmd.AddCommand(tuiCommand(opts))
	return cmd
}

func doctorCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check runtime health",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := runtimeFromOptions(opts)
			if err != nil {
				return err
			}
			report := core.Doctor(runtime)
			if opts.json {
				data, err := report.JSON()
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			for _, check := range report.Checks {
				mark := "fail"
				if check.OK {
					mark = "ok"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-22s %s %s\n", check.Name, mark, check.Detail)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.json, "json", false, "print JSON")
	return cmd
}

func cursorCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "cursor", Short: "Print Cursor settings"}
	cmd.AddCommand(&cobra.Command{
		Use:   "settings",
		Short: "Print Cursor model settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := runtimeFromOptions(opts)
			if err != nil {
				return err
			}
			settings, err := core.CursorSettings(runtime)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), settings)
			return nil
		},
	})
	return cmd
}

func modelsCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "models", Short: "Manage LiteLLM model tiers"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured model tiers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}
			for _, model := range cfg.Models() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%v\n", model.ModelName, model.LiteLLMParams["model"])
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate model tiers and fallbacks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "config valid")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set <tier> <provider-model>",
		Short: "Set one LiteLLM tier model",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := runtimeFromOptions(opts)
			if err != nil {
				return err
			}
			path := filepath.Join(runtime.Path, "config.yaml")
			cfg, err := core.LoadLiteLLMConfig(path)
			if err != nil {
				return err
			}
			if err := cfg.SetModel(args[0], args[1]); err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := cfg.Save(path); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", args[0])
			return nil
		},
	})
	return cmd
}

func importCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "import <runtime-path>",
		Short: "Import an existing runtime as an external profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm := core.NewProfileManager(envMap())
			profile, err := pm.ImportExternal(opts.profile, args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "imported %s as external profile %s\n", profile.RuntimeDir, profile.Name)
			return nil
		},
	}
}

func setupCommand(opts *options) *cobra.Command {
	var domain string
	var ollamaKey string
	var hfToken string
	var hfBillTo string
	var masterKey string
	var skipPythonDeps bool
	var force bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Create a managed ezyl3 runtime profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := core.ResolvePaths(envMap(), opts.profile)
			if err != nil {
				return err
			}
			executablePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve ezyl3 executable path: %w", err)
			}
			if isInteractive(cmd) {
				_, err := setupwizard.Run(setupwizard.Options{
					Paths:          paths,
					Domain:         domain,
					ExecutablePath: executablePath,
					Secrets:        core.Secrets{HFToken: hfToken, HFBillTo: hfBillTo, OllamaAPIKey: ollamaKey, LiteLLMMasterKey: masterKey},
					Force:          force,
					SkipPythonDeps: skipPythonDeps,
				})
				return err
			}
			secrets := core.Secrets{HFToken: hfToken, HFBillTo: hfBillTo, OllamaAPIKey: ollamaKey, LiteLLMMasterKey: masterKey}
			result, err := core.RunSetup(core.SetupOptions{
				Paths:          paths,
				Domain:         domain,
				ExecutablePath: executablePath,
				Secrets:        secrets,
				Force:          force,
				SkipPythonDeps: skipPythonDeps,
			}, core.SetupDependencies{})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), result.Summary())
			return nil
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "ngrok domain, for example name.ngrok-free.dev")
	cmd.Flags().StringVar(&ollamaKey, "ollama-api-key", "", "Ollama API key")
	cmd.Flags().StringVar(&hfToken, "hf-token", "", "Hugging Face token")
	cmd.Flags().StringVar(&hfBillTo, "hf-bill-to", "", "Hugging Face org billing slug")
	cmd.Flags().StringVar(&masterKey, "master-key", "", "LiteLLM master key; generated if omitted")
	cmd.Flags().BoolVar(&skipPythonDeps, "skip-python-deps", false, "skip venv creation and LiteLLM pip install")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing managed profile")
	return cmd
}

func isInteractive(cmd *cobra.Command) bool {
	file, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func serviceCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "service", Short: "Manage LaunchAgent services"}
	for _, action := range []string{"start", "stop", "restart", "status"} {
		action := action
		cmd.AddCommand(&cobra.Command{
			Use:   action,
			Short: action + " services",
			RunE: func(cmd *cobra.Command, args []string) error {
				paths, err := core.ResolvePaths(envMap(), opts.profile)
				if err != nil {
					return err
				}
				return runServiceAction(cmd, action, paths)
			},
		})
	}
	return cmd
}

func logsCommand(opts *options) *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs litellm|ngrok",
		Short: "Show service logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := runtimeFromOptions(opts)
			if err != nil {
				return err
			}
			reader := core.FileLogReader{Runtime: runtime}
			if follow {
				return reader.Follow(cmd.Context(), args[0], cmd.OutOrStdout())
			}
			data, err := reader.Read(args[0])
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().BoolVar(&follow, "follow", false, "follow log output")
	return cmd
}

func proxyCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Run the LiteLLM proxy",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run LiteLLM for the selected runtime",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := runtimeFromOptions(opts)
			if err != nil {
				return err
			}
			return proxyRunner(runtime, core.ProxyStdio{
				Stdin:  cmd.InOrStdin(),
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		},
	})
	return cmd
}

func tuiCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the terminal dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := runtimeFromOptions(opts)
			if err != nil {
				return err
			}
			return tui.Run(runtime)
		},
	}
}

func runtimeFromOptions(opts *options) (core.Runtime, error) {
	path := opts.path
	var err error
	if strings.TrimSpace(path) == "" {
		path, err = core.DefaultRuntimePath(envMap(), opts.profile)
		if err != nil {
			return core.Runtime{}, err
		}
	}
	return core.NewRuntime(path), nil
}

func loadConfig(opts *options) (*core.LiteLLMConfig, error) {
	runtime, err := runtimeFromOptions(opts)
	if err != nil {
		return nil, err
	}
	return core.LoadLiteLLMConfig(filepath.Join(runtime.Path, "config.yaml"))
}

func envMap() map[string]string {
	env := map[string]string{}
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

func runServiceAction(cmd *cobra.Command, action string, paths core.ProfilePaths) error {
	manager := core.ServiceManager{Paths: paths}
	var results []core.ServiceActionResult
	var err error
	switch action {
	case "start":
		results, err = manager.Start()
	case "stop":
		results, err = manager.Stop()
	case "restart":
		results, err = manager.Restart()
	case "status":
		results, err = manager.Status()
	}
	for _, result := range results {
		detail := result.Detail
		if detail != "" {
			detail = " " + detail
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-14s %s%s\n", result.Service, result.State, result.Action, detail)
	}
	return err
}

func PrintJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
