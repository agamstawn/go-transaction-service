package main

import "testing"

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{
			name: "valid configuration",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"API_KEY":      "local-test-secret",
			},
		},
		{
			name: "missing database URL",
			env: map[string]string{
				"API_KEY": "local-test-secret",
			},
			wantErr: true,
		},
		{
			name: "missing API key",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
			},
			wantErr: true,
		},
		{
			name:    "missing all configuration",
			env:     map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string {
				return tt.env[key]
			}

			config, err := loadConfig(getenv)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected configuration error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if config.DatabaseURL != tt.env["DATABASE_URL"] {
				t.Error("unexpected database URL")
			}

			if config.APIKey != tt.env["API_KEY"] {
				t.Error("unexpected API key")
			}

			if config.ListenAddress != "127.0.0.1:8080" {
				t.Errorf(
					"listen address = %s, want 127.0.0.1:8080",
					config.ListenAddress,
				)
			}
		})
	}
}
