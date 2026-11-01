package main

import (
	"fmt"
	"os"

	"github.com/its-the-vibe/TinyUpdater/cmd/tinyupdater/commands"
)

func main() {
	if err := commands.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
