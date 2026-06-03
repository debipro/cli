package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/debipro/cli/pkg/config"
	"github.com/debipro/cli/pkg/debi"
	"github.com/debipro/cli/pkg/keyring"
)

func (a *App) loginCmd() *cobra.Command {
	var skipValidation bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store a Debi secret API key in your OS keychain",
		Long: "Stores a Debi secret API key (sk_live_... or sk_test_...) securely in your\n" +
			"operating system keychain for the active profile. The key is validated\n" +
			"against the API unless --skip-validation is given.\n\n" +
			"Provide the key with --api-key, pipe it via stdin, or enter it at the prompt.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.Config()
			if err != nil {
				return err
			}

			key, err := a.readAPIKey(cmd)
			if err != nil {
				return err
			}
			key = strings.TrimSpace(key)
			if key == "" {
				return fmt.Errorf("no API key provided")
			}

			mode := debi.ModeForKey(key)
			if mode == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Warning: key does not look like sk_live_/sk_test_; assuming test environment.")
				mode = config.ModeTest
			}

			if !skipValidation {
				if err := validateKey(withContext(), key, mode, a.apiVersionFlag); err != nil {
					return fmt.Errorf("API key validation failed: %w", err)
				}
			}

			if err := keyring.Set(cfg.Profile, key); err != nil {
				return fmt.Errorf("storing key in keychain: %w", err)
			}
			if err := cfg.Set("mode", mode); err != nil {
				return fmt.Errorf("saving profile mode: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"Done! Stored %s key for profile %q (%s environment).\n",
				maskKey(key), cfg.Profile, mode)
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipValidation, "skip-validation", false, "do not verify the key against the API before storing it")
	return cmd
}

// readAPIKey resolves the key from --api-key, piped stdin, or an interactive
// hidden prompt.
func (a *App) readAPIKey(cmd *cobra.Command) (string, error) {
	if a.apiKeyFlag != "" {
		return a.apiKeyFlag, nil
	}

	stdin := int(os.Stdin.Fd())
	if !term.IsTerminal(stdin) {
		// Non-interactive: read a single line from stdin.
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text(), nil
		}
		return "", fmt.Errorf("no API key on stdin")
	}

	fmt.Fprint(cmd.OutOrStdout(), "Enter your Debi secret API key: ")
	raw, err := term.ReadPassword(stdin)
	fmt.Fprintln(cmd.OutOrStdout())
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// validateKey performs a cheap authenticated request to confirm the key works.
func validateKey(ctx context.Context, key, mode, apiVersion string) error {
	client := debi.NewClient(key, debi.BaseURLForMode(mode), apiVersion)
	_, err := client.Do(ctx, debi.Request{
		Method: "GET",
		Path:   "/v1/customers",
		Query:  map[string][]string{"limit": {"1"}},
	})
	return err
}

// maskKey returns a redacted form of a secret key for display.
func maskKey(key string) string {
	if len(key) <= 12 {
		return "****"
	}
	return key[:9] + "..." + key[len(key)-4:]
}
