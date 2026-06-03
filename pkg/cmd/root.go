// Package cmd wires together the debi CLI command tree.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/debipro/cli/pkg/config"
	"github.com/debipro/cli/pkg/debi"
	"github.com/debipro/cli/pkg/keyring"
	"github.com/debipro/cli/pkg/output"
	"github.com/debipro/cli/pkg/spec"
)

// App holds global flags and lazily-loaded configuration shared across all
// commands.
type App struct {
	cfg *config.Config

	profileFlag    string
	configFile     string
	apiKeyFlag     string
	apiVersionFlag string
	liveFlag       bool
	testFlag       bool
	jsonFlag       bool
	noColor        bool
	verboseFlag    bool
}

// Execute builds the command tree and runs it.
func Execute() error {
	app := &App{}
	root, err := app.rootCmd()
	if err != nil {
		return err
	}
	return root.Execute()
}

func (a *App) rootCmd() (*cobra.Command, error) {
	root := &cobra.Command{
		Use:           "debi",
		Short:         "debi is a command-line interface for the Debi API",
		Long:          "debi helps you build, test, and manage your Debi integration right from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if a.noColor {
				color.NoColor = true
			}
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&a.profileFlag, "profile", "", "config profile to use")
	pf.StringVar(&a.configFile, "config", "", "config file (default: $XDG_CONFIG_HOME/debi/config.toml)")
	pf.StringVar(&a.apiKeyFlag, "api-key", "", "Debi secret API key to use for this command")
	pf.StringVar(&a.apiVersionFlag, "api-version", "", "value of the Api-Version header")
	pf.BoolVar(&a.liveFlag, "live", false, "use the live environment (api.debi.pro)")
	pf.BoolVar(&a.testFlag, "test", false, "use the sandbox environment (api.debi-test.pro)")
	pf.BoolVar(&a.jsonFlag, "json", false, "print raw JSON (no indentation or color)")
	pf.BoolVar(&a.noColor, "no-color", false, "disable colorized output")
	pf.BoolVar(&a.verboseFlag, "verbose", false, "log API requests to stderr (also enabled by DEBI_DEBUG)")

	root.AddCommand(
		a.versionCmd(),
		a.loginCmd(),
		a.logoutCmd(),
		a.configCmd(),
		a.specCmd(),
		a.eventsCmd(),
		a.listenCmd(),
		a.completionCmd(),
	)
	a.addGenericCommands(root)
	if err := a.addResourceCommands(root); err != nil {
		return nil, err
	}

	return root, nil
}

// Config returns the loaded configuration, loading it on first use.
func (a *App) Config() (*config.Config, error) {
	if a.cfg != nil {
		return a.cfg, nil
	}
	cfg, err := config.New(a.configFile, a.profileFlag)
	if err != nil {
		return nil, err
	}
	a.cfg = cfg
	return cfg, nil
}

// ResolveAPIKey returns the secret key to use, in priority order:
// --api-key flag, DEBI_API_KEY env, then the OS keychain for the active
// profile.
func (a *App) ResolveAPIKey() (string, error) {
	if a.apiKeyFlag != "" {
		return a.apiKeyFlag, nil
	}
	if env := os.Getenv(config.EnvAPIKey); env != "" {
		return env, nil
	}
	cfg, err := a.Config()
	if err != nil {
		return "", err
	}
	key, err := keyring.Get(cfg.Profile)
	if err != nil {
		return "", fmt.Errorf("no API key for profile %q: run `debi login` or set %s (%w)", cfg.Profile, config.EnvAPIKey, err)
	}
	return key, nil
}

// Mode determines the API environment: explicit flag, then key prefix, then
// the active profile's configured mode.
func (a *App) Mode(apiKey string) string {
	switch {
	case a.liveFlag:
		return config.ModeLive
	case a.testFlag:
		return config.ModeTest
	}
	if m := debi.ModeForKey(apiKey); m != "" {
		return m
	}
	if cfg, err := a.Config(); err == nil {
		return cfg.CurrentProfile().Mode
	}
	return config.ModeTest
}

// Client builds an API client from the resolved key, mode and version.
func (a *App) Client() (*debi.Client, error) {
	key, err := a.ResolveAPIKey()
	if err != nil {
		return nil, err
	}
	mode := a.Mode(key)
	apiVersion := a.apiVersionFlag
	deviceName := ""
	if cfg, cerr := a.Config(); cerr == nil {
		if apiVersion == "" {
			apiVersion = cfg.CurrentProfile().APIVersion
		}
		deviceName = cfg.CurrentProfile().DeviceName
	}
	client := debi.NewClient(key, debi.BaseURLForMode(mode), apiVersion)
	client.DeviceName = deviceName
	if a.verbose() {
		client.Logf = func(format string, args ...interface{}) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
	}
	return client, nil
}

func (a *App) verbose() bool {
	return a.verboseFlag || os.Getenv(config.EnvDebug) != ""
}

// ColorEnabled reports whether colorized output should be produced.
func (a *App) ColorEnabled() bool {
	return !a.jsonFlag && !color.NoColor
}

// PrintResponse renders a successful API response to stdout.
func (a *App) PrintResponse(resp *debi.Response) error {
	return output.PrintJSON(os.Stdout, resp.Body, a.jsonFlag, a.ColorEnabled())
}

// printRaw renders an arbitrary JSON body to stdout.
func (a *App) printRaw(body []byte) error {
	return output.PrintJSON(os.Stdout, body, a.jsonFlag, a.ColorEnabled())
}

// loadSpec parses the embedded OpenAPI spec.
func (a *App) loadSpec() (*spec.Spec, error) {
	return spec.Load()
}

// withContext returns a background context for API calls.
func withContext() context.Context {
	return context.Background()
}
