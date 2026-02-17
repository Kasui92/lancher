package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kasui92/lancher/internal/fileutil"
)

// TestIgnoreStylePatterns verifies that ignore patterns with trailing slashes and wildcards work correctly
func TestIgnoreStylePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories and files
	os.MkdirAll(filepath.Join(tmpDir, "node_modules", "pkg"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "node_modules", "package.json"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, ".next", "cache"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".next", "build.js"), []byte("build"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.log"), []byte("log"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "src", "vendor"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "vendor", "lib.go"), []byte("lib"), 0644)

	// Create .lancherignore with trailing slashes
	ignoreContent := `# Test patterns
node_modules/
.next/
*.log
**/vendor/
`
	os.WriteFile(filepath.Join(tmpDir, ".lancherignore"), []byte(ignoreContent), 0644)

	// Load patterns
	patterns := fileutil.LoadIgnoreFile(tmpDir)
	if len(patterns) != 4 {
		t.Fatalf("Expected 4 patterns, got %d: %v", len(patterns), patterns)
	}

	// Test ShouldIgnorePath
	tests := []struct {
		path     string
		expected bool
		reason   string
	}{
		{"node_modules", true, "node_modules/ pattern should match"},
		{"node_modules/pkg", true, "nested path in node_modules should match"},
		{".next", true, ".next/ pattern should match"},
		{".next/cache", true, "nested path in .next should match"},
		{"test.log", true, "*.log pattern should match"},
		{"logs/app.log", true, "*.log should match nested .log files"},
		{"src/vendor", true, "**/vendor pattern should match at any depth"},
		{"vendor", true, "**/vendor pattern should match at root too"},
		{"main.go", false, "main.go should not be ignored"},
		{"src/handler.go", false, "regular files should not be ignored"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := fileutil.ShouldIgnorePath(tt.path, patterns)
			if result != tt.expected {
				t.Errorf("ShouldIgnorePath(%q) = %v, want %v - %s",
					tt.path, result, tt.expected, tt.reason)
			}
		})
	}
}

// TestRootRelativePatterns verifies that patterns starting with / work correctly
func TestRootRelativePatterns(t *testing.T) {
	patterns := []string{"/config", "*.tmp"}

	tests := []struct {
		path     string
		expected bool
		reason   string
	}{
		{"config", true, "/config should match at root"},
		{"config/app.yml", true, "/config should match nested paths"},
		{"src/config", false, "/config should NOT match in subdirectory"},
		{"test.tmp", true, "*.tmp should match anywhere"},
		{"src/test.tmp", true, "*.tmp should match in subdirectory"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := fileutil.ShouldIgnorePath(tt.path, patterns)
			if result != tt.expected {
				t.Errorf("ShouldIgnorePath(%q) = %v, want %v - %s",
					tt.path, result, tt.expected, tt.reason)
			}
		})
	}
}

// TestRecursivePatterns verifies that ** patterns work correctly
func TestRecursivePatterns(t *testing.T) {
	patterns := []string{"**/node_modules", "dist/**", "**/*.pyc"}

	tests := []struct {
		path     string
		expected bool
		reason   string
	}{
		{"node_modules", true, "**/node_modules should match at root"},
		{"src/node_modules", true, "**/node_modules should match nested"},
		{"deep/nested/node_modules", true, "**/node_modules should match deeply nested"},
		{"dist", true, "dist/** should match dist directory"},
		{"dist/bundle.js", true, "dist/** should match files in dist"},
		{"test.pyc", true, "**/*.pyc should match .pyc files"},
		{"src/__pycache__/test.pyc", true, "**/*.pyc should match nested .pyc files"},
		{"main.go", false, "main.go should not match"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := fileutil.ShouldIgnorePath(tt.path, patterns)
			if result != tt.expected {
				t.Errorf("ShouldIgnorePath(%q) = %v, want %v - %s",
					tt.path, result, tt.expected, tt.reason)
			}
		})
	}
}

// TestLoadIgnoreFile verifies that .lancherignore files are parsed correctly
func TestLoadIgnoreFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "basic patterns",
			content: `node_modules
*.log
dist/`,
			expected: []string{"node_modules", "*.log", "dist/"},
		},
		{
			name: "with comments and blank lines",
			content: `# Dependencies
node_modules/

# Build output
dist/
*.tmp

# Empty lines should be ignored
`,
			expected: []string{"node_modules/", "dist/", "*.tmp"},
		},
		{
			name: "recursive and root patterns",
			content: `**/vendor
/config
**/*.pyc`,
			expected: []string{"**/vendor", "/config", "**/*.pyc"},
		},
		{
			name: "empty file",
			content: `
# Only comments

`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create .lancherignore with test content
			ignoreFile := filepath.Join(tmpDir, ".lancherignore")
			if err := os.WriteFile(ignoreFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create .lancherignore: %v", err)
			}

			patterns := fileutil.LoadIgnoreFile(tmpDir)

			if len(patterns) != len(tt.expected) {
				t.Errorf("Expected %d patterns, got %d: %v", len(tt.expected), len(patterns), patterns)
				return
			}

			for i, expected := range tt.expected {
				if patterns[i] != expected {
					t.Errorf("Pattern[%d]: expected %q, got %q", i, expected, patterns[i])
				}
			}
		})
	}
}

// TestEdgeCasePatterns tests edge cases and special scenarios
func TestEdgeCasePatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		path     string
		expected bool
	}{
		{"empty pattern", []string{""}, "any/path", false},
		{"whitespace pattern", []string{"  "}, "any/path", false},
		{"trailing whitespace", []string{"node_modules  "}, "node_modules", true},
		{"leading whitespace", []string{"  *.log"}, "test.log", true},
		{"multiple slashes", []string{"node_modules/"}, "node_modules", true},
		{"dot files", []string{".*"}, ".env", true},
		{"dot files in path", []string{".*"}, "src/.cache", true},
		{"no patterns", []string{}, "any/path", false},
		{"nested directory pattern", []string{"**/test"}, "src/test/file.js", true},
		{"path with double extension", []string{"*.tar.gz"}, "archive.tar.gz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileutil.ShouldIgnorePath(tt.path, tt.patterns)
			if result != tt.expected {
				t.Errorf("ShouldIgnorePath(%q, %v) = %v, want %v",
					tt.path, tt.patterns, result, tt.expected)
			}
		})
	}
}
