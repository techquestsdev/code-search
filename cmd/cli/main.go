package main

import (
	"os"

	"github.com/techquestsdev/code-search/cmd/cli/cmd"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
