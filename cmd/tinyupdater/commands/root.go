package commands

import (
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "tinyupdater",
		Short: "TinyUpdater service and tools",
	}
	root.AddCommand(newServerCommand(), newManifestCommand())
	return root
}
