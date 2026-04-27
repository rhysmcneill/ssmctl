// Package config defines the configuration structure for ssmctl and validation
// logic. Configuration values can be provided via command-line flags, environment
// variables, or AWS configuration files.
package config

import (
	"fmt"
	"time"
)

// Config holds all configuration options for ssmctl including AWS credentials,
// output format, and command execution settings.
type Config struct {
	Profile string
	Region  string
	Output  string
	Debug   bool
	Timeout time.Duration
}

// Validate checks that the Config values are valid. It verifies the output
// format is supported and the timeout is positive.
func (c *Config) Validate() error {
	switch c.Output {
	case "json", "text":
	default:
		return fmt.Errorf("invalid output format: %s", c.Output)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}
