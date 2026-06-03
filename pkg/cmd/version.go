package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tucuota/debi-cli/pkg/version"
)

func (a *App) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the debi CLI version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), version.Template, version.Version)
			return nil
		},
	}
}
