package config

import (
	"fmt"
	"time"
)

type Config struct {
	Profile string
	Region  string
	Output  string
	Debug   bool
	Timeout time.Duration
}

func (c *Config) Validate() error {
	switch c.Output {
	case "json", "text":
		return fmt.Errorf("invalid output format: %s", c.Output)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}
