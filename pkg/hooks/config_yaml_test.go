package hooks

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestHookUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlData    string
		wantTimeout time.Duration
		wantErr     bool
	}{
		{
			name: "valid duration string",
			yamlData: `
name: test
command: echo
timeout: 10s
`,
			wantTimeout: 10 * time.Second,
		},
		{
			name: "valid duration string minutes",
			yamlData: `
name: test
command: echo
timeout: 1m30s
`,
			wantTimeout: 90 * time.Second,
		},
		{
			name: "numeric timeout",
			yamlData: `
name: test
command: echo
timeout: 30
`,
			wantTimeout: 30 * time.Second,
		},
		{
			name: "invalid duration",
			yamlData: `
name: test
command: echo
timeout: invalid
`,
			wantErr: true,
		},
		{
			name: "empty timeout",
			yamlData: `
name: test
command: echo
`,
			wantTimeout: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h Hook
			err := yaml.Unmarshal([]byte(tt.yamlData), &h)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && h.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %v, want %v", h.Timeout, tt.wantTimeout)
			}
		})
	}
}
