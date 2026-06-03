package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/debipro/cli/pkg/keyring"
)

func (a *App) logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored API key for the active profile",
		Long:  "Deletes the secret API key from the OS keychain for the active profile. Profile settings in config.toml are kept; use `debi config unset` to remove the profile entirely.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			if err := keyring.Delete(cfg.Profile); err != nil {
				return fmt.Errorf("removing key for profile %q: %w", cfg.Profile, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed stored API key for profile %q.\n", cfg.Profile)
			return nil
		},
	}
}
