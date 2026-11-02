package commands

import (
	"fmt"

	"github.com/its-the-vibe/TinyUpdater/internal/config"
	"github.com/its-the-vibe/TinyUpdater/internal/server"
	"github.com/spf13/cobra"
)

func newServerCommand() *cobra.Command {
	var configPath string

	command := &cobra.Command{
		Use:   "server",
		Short: "Run the TinyUpdater HTTP service",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}

			httpServer := server.NewHTTPServer(cfg)
			fmt.Fprintf(cmd.OutOrStdout(), "TinyUpdater server listening on %s\n", httpServer.Addr)
			return httpServer.ListenAndServe()
		},
	}
	command.Flags().StringVar(&configPath, "config", "config.yaml", "path to the YAML configuration file")
	return command
}
