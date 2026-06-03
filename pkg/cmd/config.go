package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tucuota/debi-cli/pkg/keyring"
)

func (a *App) configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage debi CLI configuration profiles",
	}
	cmd.AddCommand(
		a.configListCmd(),
		a.configSetCmd(),
		a.configUseCmd(),
		a.configUnsetCmd(),
	)
	return cmd
}

func (a *App) configListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Config file: %s\n", cfg.File)
			fmt.Fprintf(out, "Active profile: %s\n\n", cfg.Profile)

			profiles := cfg.Profiles()
			if len(profiles) == 0 {
				fmt.Fprintln(out, "No profiles configured yet. Run `debi login` to create one.")
				return nil
			}
			for _, name := range profiles {
				p := cfg.GetProfile(name)
				marker := " "
				if name == cfg.Profile {
					marker = "*"
				}
				hasKey := "no key"
				if _, kerr := keyring.Get(name); kerr == nil {
					hasKey = "key stored"
				}
				fmt.Fprintf(out, "%s %-16s mode=%-4s api_version=%-12s [%s]\n",
					marker, name, p.Mode, dash(p.APIVersion), hasKey)
			}
			return nil
		},
	}
}

func (a *App) configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a value on the active profile (key: mode | api_version | device_name)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			key, value := args[0], args[1]
			switch key {
			case "mode", "api_version", "device_name":
			default:
				return fmt.Errorf("unknown key %q (allowed: mode, api_version, device_name)", key)
			}
			if err := cfg.Set(key, value); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set %s=%s on profile %q.\n", key, value, cfg.Profile)
			return nil
		},
	}
}

func (a *App) configUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Set the default active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			if err := cfg.SetActiveProfile(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active profile is now %q.\n", args[0])
			return nil
		},
	}
}

func (a *App) configUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset [profile]",
		Short: "Remove a profile and its stored key (defaults to the active profile)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			name := cfg.Profile
			if len(args) == 1 {
				name = args[0]
			}
			if err := keyring.Delete(name); err != nil && !errors.Is(err, keyring.ErrNotFound) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not remove key from keychain: %v\n", err)
			}
			if err := cfg.Unset(name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q.\n", name)
			return nil
		},
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
