package config

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid",
			cfg:     Config{Profile: "test", Region: "us-east-1", Output: "text", Debug: false, Timeout: 60 * time.Second},
			wantErr: false,
		},
		{
			name:    "invalid output format",
			cfg:     Config{Profile: "test", Region: "us-east-1", Output: "invalid", Debug: false, Timeout: 60 * time.Second},
			wantErr: true,
		},
		{
			name:    "invalid timeout",
			cfg:     Config{Profile: "test", Region: "us-east-1", Output: "text", Debug: false, Timeout: 0},
			wantErr: true,
		},
		{
			name:    "zero timeout",
			cfg:     Config{Profile: "test", Region: "us-east-1", Output: "text", Debug: false, Timeout: 0},
			wantErr: true,
		},
		{
			name:    "negative timeout",
			cfg:     Config{Profile: "test", Region: "us-east-1", Output: "text", Debug: false, Timeout: -1},
			wantErr: true,
		},
		{
			name:    "valid timeout",
			cfg:     Config{Profile: "test", Region: "us-east-1", Output: "text", Debug: false, Timeout: 60 * time.Second},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
