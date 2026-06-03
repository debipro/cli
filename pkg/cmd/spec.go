package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/debipro/cli/pkg/spec"
)

func (a *App) specCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Inspect or refresh the embedded Debi OpenAPI specification",
		Long: "Resource commands are generated from Debi's OpenAPI specification. A canonical\n" +
			"copy is embedded in the binary; `debi spec update` refreshes a local cached\n" +
			"copy from " + spec.SourceURL + ".",
	}
	cmd.AddCommand(a.specInfoCmd(), a.specUpdateCmd())
	return cmd
}

func (a *App) specInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show the active specification source and version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.loadSpec()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Title:   %s\n", s.Info.Title)
			fmt.Fprintf(out, "Version: %s\n", s.Info.Version)
			fmt.Fprintf(out, "Source:  %s\n", spec.Source())
			fmt.Fprintf(out, "Origin:  %s\n", spec.SourceURL)
			fmt.Fprintf(out, "Paths:   %d\n", len(s.Paths))
			return nil
		},
	}
}

func (a *App) specUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Download the latest specification and cache it locally",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := spec.Update(withContext())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated spec cached at %s\n", path)
			return nil
		},
	}
}
