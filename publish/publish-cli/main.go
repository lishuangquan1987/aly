package main

import (
	"os"

	"aly/publish-cli/internal/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
