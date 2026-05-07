// Package config defines the configuration structure for ssmctl and validation
// logic. Configuration values can be provided via command-line flags, environment
// variables, or AWS configuration files.
package config

import (
	"fmt"
	"time"
	"os"
	"log"
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

	if c.Debug {
		debugLog := log.New(os.Stderr, "[DEBUG] ", log.LstdFlags)
		debugLog.Printf("Profile: %s\n", c.Profile)
		debugLog.Printf("Region: %s\n", c.Region)
		debugLog.Printf("Output: %s\n", c.Output)
		debugLog.Printf("Timeout: %v\n", c.Timeout)
	}
	return nil
}
