// Command debi is a command-line interface for the Debi API.
package main

import (
	"fmt"
	"os"

	"github.com/tucuota/debi-cli/pkg/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
