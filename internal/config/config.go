package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFileNames lists all supported configuration file names in order of priority
var ConfigFileNames = []string{
	".lancher.yaml",
	".lancher.yml",
	"lancher.yaml",
	"lancher.yml",
}

// Config represents the lancher configuration file
type Config struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	Version     string   `yaml:"version"`
	Hooks       []string `yaml:"hooks"`
	Ignore      []string `yaml:"ignore"`
}

// LoadResult contains the loaded config and metadata about the loading process
type LoadResult struct {
	Config      *Config
	FoundFiles  []string // List of all config files found
	UsedFile    string   // The config file that was actually used
}

// LoadConfig loads configuration from the template directory
// Searches for config files in order of priority and returns the first one found
func LoadConfig(templatePath string) (*Config, error) {
	result := LoadConfigWithDetails(templatePath)
	return result.Config, nil
}

// LoadConfigWithDetails loads configuration and returns detailed information
func LoadConfigWithDetails(templatePath string) *LoadResult {
	result := &LoadResult{
		FoundFiles: []string{},
	}

	// Check all possible config file names
	for _, fileName := range ConfigFileNames {
		configPath := filepath.Join(templatePath, fileName)

		// Check if config file exists
		if _, err := os.Stat(configPath); err == nil {
			result.FoundFiles = append(result.FoundFiles, fileName)

			// Load the first config found (highest priority)
			if result.Config == nil {
				data, err := os.ReadFile(configPath)
				if err != nil {
					// Could return error here, but for now just skip this file
					continue
				}

				var cfg Config
				if err := yaml.Unmarshal(data, &cfg); err != nil {
					// Could return error here, but for now just skip this file
					continue
				}

				result.Config = &cfg
				result.UsedFile = fileName
			}
		}
	}

	return result
}

// ShouldIgnore checks if a file should be ignored during template creation
func (c *Config) ShouldIgnore(relativePath string) bool {
	if c == nil {
		return false
	}

	for _, pattern := range c.Ignore {
		if matched, _ := filepath.Match(pattern, relativePath); matched {
			return true
		}
		// Also check if pattern matches any part of the path
		if matched, _ := filepath.Match(pattern, filepath.Base(relativePath)); matched {
			return true
		}
	}
	return false
}

// HasHooks returns true if config has hooks defined
func (c *Config) HasHooks() bool {
	return c != nil && len(c.Hooks) > 0
}

// HasIgnore returns true if config has deprecated ignore patterns defined in YAML
func (c *Config) HasIgnore() bool {
	return c != nil && len(c.Ignore) > 0
}

// GetMetadata returns formatted metadata string for display
func (c *Config) GetMetadata() string {
	if c == nil {
		return ""
	}

	var metadata string
	if c.Name != "" {
		metadata += fmt.Sprintf("Name: %s\n", c.Name)
	}
	if c.Description != "" {
		metadata += fmt.Sprintf("Description: %s\n", c.Description)
	}
	if c.Author != "" {
		metadata += fmt.Sprintf("Author: %s\n", c.Author)
	}
	if c.Version != "" {
		metadata += fmt.Sprintf("Version: %s\n", c.Version)
	}
	return metadata
}
