package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ezyl3/internal/core"
	"ezyl3/internal/tui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type options struct {
	profile string
	path    string
	json    bool
}

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
			if isInteractive(cmd) {
				if !cmd.Flags().Changed("domain") {
					domain, err = readPrompt(cmd, "ngrok domain (optional, leave blank for local-only): ")
					if err != nil {
						return err
					}
				}
				if !cmd.Flags().Changed("hf-token") {
					hfToken, err = readSecretPrompt(cmd, "Hugging Face token (optional): ")
					if err != nil {
						return err
					}
				}
				if !cmd.Flags().Changed("ollama-api-key") {
					ollamaKey, err = readSecretPrompt(cmd, "Ollama API key (optional): ")
					if err != nil {
						return err
					}
				}
				if !cmd.Flags().Changed("hf-bill-to") {
					hfBillTo, err = readPrompt(cmd, "Hugging Face billing org (optional): ")
					if err != nil {
						return err
					}
				}
				if !cmd.Flags().Changed("master-key") {
					masterKey, err = readSecretPrompt(cmd, "LiteLLM master key (optional, generated if blank): ")
					if err != nil {
						return err
					}
				}
			}
			secrets := core.Secrets{HFToken: hfToken, HFBillTo: hfBillTo, OllamaAPIKey: ollamaKey, LiteLLMMasterKey: masterKey}
			result, err := core.RunSetup(core.SetupOptions{
				Paths:          paths,
				Domain:         domain,
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

func readPrompt(cmd *cobra.Command, label string) (string, error) {
	_, _ = fmt.Fprint(cmd.ErrOrStderr(), label)
	value, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func readSecretPrompt(cmd *cobra.Command, label string) (string, error) {
	file, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return readPrompt(cmd, label)
	}
	_, _ = fmt.Fprint(cmd.ErrOrStderr(), label)
	value, err := term.ReadPassword(int(file.Fd()))
	_, _ = fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(value)), nil
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
			if args[0] != "litellm" && args[0] != "ngrok" {
				return fmt.Errorf("unknown log target %q", args[0])
			}
			path := filepath.Join(runtime.Path, "logs", args[0]+".out.log")
			if follow {
				tail := exec.Command("tail", "-f", path)
				tail.Stdout = cmd.OutOrStdout()
				tail.Stderr = cmd.ErrOrStderr()
				return tail.Run()
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(cmd.OutOrStdout(), file)
			return err
		},
	}
	cmd.Flags().BoolVar(&follow, "follow", false, "follow log output")
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
	uid := os.Getuid()
	gui := fmt.Sprintf("gui/%d", uid)
	commands := [][]string{}
	switch action {
	case "start":
		commands = [][]string{{"bootstrap", gui, paths.LaunchAgentPath("litellm")}, {"bootstrap", gui, paths.LaunchAgentPath("ngrok")}}
	case "stop":
		commands = [][]string{{"bootout", gui, paths.LaunchAgentPath("ngrok")}, {"bootout", gui, paths.LaunchAgentPath("litellm")}}
	case "restart":
		commands = [][]string{{"kickstart", "-k", gui + "/" + paths.LaunchAgentLabel("litellm")}, {"kickstart", "-k", gui + "/" + paths.LaunchAgentLabel("ngrok")}}
	case "status":
		commands = [][]string{{"print", gui + "/" + paths.LaunchAgentLabel("litellm")}, {"print", gui + "/" + paths.LaunchAgentLabel("ngrok")}}
	}
	for _, args := range commands {
		if err := runLaunchctl(cmd, args...); err != nil {
			return err
		}
	}
	return nil
}

func runLaunchctl(cmd *cobra.Command, args ...string) error {
	launchctl := exec.Command("launchctl", args...)
	launchctl.Stdout = cmd.OutOrStdout()
	launchctl.Stderr = cmd.ErrOrStderr()
	return launchctl.Run()
}

func PrintJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
