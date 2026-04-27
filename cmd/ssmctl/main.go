// Package main is the entry point for the ssmctl CLI application.
package main

import (
	"fmt"
	"os"

	"github.com/rhysmcneill/ssmctl/internal/cmd"
)

func main() {
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if exitErr, ok := err.(*cmd.ExitCodeError); ok {
			os.Exit(exitErr.ExitCode)
		}
		os.Exit(1)
	}
}
