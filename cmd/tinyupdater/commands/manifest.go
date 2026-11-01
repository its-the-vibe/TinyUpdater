package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newManifestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "manifest",
		Short: "Create or inspect an update manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "hello world")
			return err
		},
	}
}
