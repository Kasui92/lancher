package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kasui92/lancher/internal/config"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantName    string
		wantDesc    string
		wantAuthor  string
		wantVersion string
		wantHooks   int
		wantIgnore  int
		wantErr     bool
	}{
		{
			name: "valid config with all fields",
			yamlContent: `name: Test Template
description: A test template
author: Test Author
version: 1.0.0
hooks:
  - npm install
  - git init
ignore:
  - node_modules
  - "*.log"`,
			wantName:    "Test Template",
			wantDesc:    "A test template",
			wantAuthor:  "Test Author",
			wantVersion: "1.0.0",
			wantHooks:   2,
			wantIgnore:  2,
			wantErr:     false,
		},
		{
			name: "minimal config",
			yamlContent: `name: Minimal
description: Minimal template`,
			wantName:   "Minimal",
			wantDesc:   "Minimal template",
			wantHooks:  0,
			wantIgnore: 0,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, config.ConfigFileNames[0])

			// Write config file
			if err := os.WriteFile(configPath, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			// Load config
			cfg, err := config.LoadConfig(tmpDir)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg == nil {
				t.Fatal("expected config but got nil")
			}

			if cfg.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cfg.Name, tt.wantName)
			}
			if cfg.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", cfg.Description, tt.wantDesc)
			}
			if cfg.Author != tt.wantAuthor {
				t.Errorf("Author = %q, want %q", cfg.Author, tt.wantAuthor)
			}
			if cfg.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", cfg.Version, tt.wantVersion)
			}
			if len(cfg.Hooks) != tt.wantHooks {
				t.Errorf("Hooks count = %d, want %d", len(cfg.Hooks), tt.wantHooks)
			}
			if len(cfg.Ignore) != tt.wantIgnore {
				t.Errorf("Ignore count = %d, want %d", len(cfg.Ignore), tt.wantIgnore)
			}
		})
	}
}

func TestLoadConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, err := config.LoadConfig(tmpDir)

	if err != nil {
		t.Errorf("expected no error when config not found, got: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config when file not found, got: %+v", cfg)
	}
}

func TestShouldIgnore(t *testing.T) {
	cfg := &config.Config{
		Ignore: []string{
			"node_modules",
			"*.log",
			".git",
			"*.tmp",
		},
	}

	tests := []struct {
		name     string
		path     string
		wantSkip bool
	}{
		{"node_modules directory", "node_modules", true},
		{"file in node_modules", "node_modules/package.json", false}, // glob only matches basename
		{"log file", "app.log", true},
		{"nested log file", "logs/app.log", true},
		{"tmp file", "test.tmp", true},
		{"git directory", ".git", true},
		{"normal file", "README.md", false},
		{"normal directory", "src", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ShouldIgnore(tt.path)
			if got != tt.wantSkip {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.wantSkip)
			}
		})
	}
}

func TestShouldIgnoreNilConfig(t *testing.T) {
	var cfg *config.Config
	if cfg.ShouldIgnore("anything") {
		t.Error("nil config should not ignore any files")
	}
}

func TestHasHooks(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			name: "config with hooks",
			cfg: &config.Config{
				Hooks: []string{"npm install", "git init"},
			},
			want: true,
		},
		{
			name: "config without hooks",
			cfg: &config.Config{
				Hooks: []string{},
			},
			want: false,
		},
		{
			name: "nil hooks",
			cfg:  &config.Config{},
			want: false,
		},
		{
			name: "nil config",
			cfg:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.HasHooks()
			if got != tt.want {
				t.Errorf("HasHooks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasIgnore(t *testing.T) {
	cfg := &config.Config{Ignore: []string{"node_modules"}}
	if !cfg.HasIgnore() {
		t.Error("HasIgnore() should return true when ignore patterns exist")
	}

	var nilCfg *config.Config
	if nilCfg.HasIgnore() {
		t.Error("HasIgnore() should return false for nil config")
	}
}

func TestGetMetadata(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantText []string
	}{
		{
			name: "full metadata",
			cfg: &config.Config{
				Name:        "My Template",
				Description: "A great template",
				Author:      "John Doe",
				Version:     "1.0.0",
			},
			wantText: []string{"My Template", "A great template", "John Doe", "1.0.0"},
		},
		{
			name: "partial metadata",
			cfg: &config.Config{
				Name:    "Minimal",
				Version: "0.1.0",
			},
			wantText: []string{"Minimal", "0.1.0"},
		},
		{
			name:     "empty config",
			cfg:      &config.Config{},
			wantText: []string{},
		},
		{
			name:     "nil config",
			cfg:      nil,
			wantText: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetMetadata()

			if len(tt.wantText) == 0 {
				if got != "" {
					t.Errorf("expected empty metadata, got: %q", got)
				}
				return
			}

			for _, want := range tt.wantText {
				if len(want) > 0 && !contains(got, want) {
					t.Errorf("GetMetadata() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
