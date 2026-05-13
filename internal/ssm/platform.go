package ssm

import (
	"os"
	"path/filepath"
)

// awsConfigPath returns the platform-appropriate path to the AWS shared
// config file. It uses os.UserHomeDir so the result is correct on both
// Unix (~/.aws/config) and Windows (%USERPROFILE%\.aws\config).
func awsConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.aws/config"
	}
	return filepath.Join(home, ".aws", "config")
}
