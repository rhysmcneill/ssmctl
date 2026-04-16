package main

import (
	"os"

	"github.com/rhysmcneill/ssmctl/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		os.Exit(1)
	}
}
