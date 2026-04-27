// Package main is the entry point for the ssmctl CLI application.
package main

import (
	"os"

	"github.com/rhysmcneill/ssmctl/internal/cmd"
)

func main() {
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
