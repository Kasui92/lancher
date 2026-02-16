package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kasui92/lancher/internal/config"
	"github.com/Kasui92/lancher/internal/fileutil"
)

// copyTemplate replicates the logic from create.go for testing
func copyTemplate(srcPath, dstPath string, cfg *config.Config) error {
	// Load .lancherignore patterns (takes priority over config ignore)
	ignorePatterns := fileutil.LoadIgnoreFile(srcPath)
	useLancherIgnore := len(ignorePatterns) > 0

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		// Skip .lancher.yaml config file
		fileName := filepath.Base(relPath)
		for _, configFile := range config.ConfigFileNames {
			if fileName == configFile {
				return nil
			}
		}

		// Skip .lancherignore file (never copy to destination)
		if fileName == fileutil.IgnoreFileName {
			return nil
		}

		// Skip .git directory (always excluded from templates)
		if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check ignore patterns: .lancherignore takes priority, fallback to config ignore
		if useLancherIgnore {
			if fileutil.ShouldIgnorePath(relPath, ignorePatterns) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		} else if cfg != nil && cfg.ShouldIgnore(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Construct destination path
		targetPath := filepath.Join(dstPath, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return fileutil.CopyFile(path, targetPath)
	})
}

// TestCreateCommandExcludesGit verifies that .git directory is never copied from templates
func TestCreateCommandExcludesGit(t *testing.T) {
	// Create temp directories
	srcDir, err := os.MkdirTemp("", "template-src-*")
	if err != nil {
		t.Fatalf("Failed to create temp src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "template-dst-*")
	if err != nil {
		t.Fatalf("Failed to create temp dst dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	// Create source structure with .git directory
	gitDir := filepath.Join(srcDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Add a file in .git
	gitFile := filepath.Join(gitDir, "config")
	if err := os.WriteFile(gitFile, []byte("git config"), 0644); err != nil {
		t.Fatalf("Failed to create git config file: %v", err)
	}

	// Add a normal file
	normalFile := filepath.Join(srcDir, "README.md")
	if err := os.WriteFile(normalFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create README: %v", err)
	}

	// Copy template
	err = copyTemplate(srcDir, dstDir, nil)
	if err != nil {
		t.Fatalf("copyTemplate failed: %v", err)
	}

	// Verify .git was not copied
	dstGitDir := filepath.Join(dstDir, ".git")
	if _, err := os.Stat(dstGitDir); !os.IsNotExist(err) {
		t.Errorf(".git directory should not be copied, but it exists")
	}

	// Verify normal file was copied
	dstNormalFile := filepath.Join(dstDir, "README.md")
	if _, err := os.Stat(dstNormalFile); os.IsNotExist(err) {
		t.Errorf("README.md should be copied, but it doesn't exist")
	}
}

// TestCreateCommandExcludesConfig verifies that .lancher.yaml is never copied to destination
func TestCreateCommandExcludesConfig(t *testing.T) {
	// Create temp directories
	srcDir, err := os.MkdirTemp("", "template-src-*")
	if err != nil {
		t.Fatalf("Failed to create temp src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "template-dst-*")
	if err != nil {
		t.Fatalf("Failed to create temp dst dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	// Create .lancher.yaml
	lancherYaml := filepath.Join(srcDir, config.ConfigFileNames[0])
	if err := os.WriteFile(lancherYaml, []byte("name: test\nversion: 1.0.0"), 0644); err != nil {
		t.Fatalf("Failed to create .lancher.yaml: %v", err)
	}

	// Add a normal file
	normalFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(normalFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test.txt: %v", err)
	}

	// Copy template
	err = copyTemplate(srcDir, dstDir, nil)
	if err != nil {
		t.Fatalf("copyTemplate failed: %v", err)
	}

	// Verify .lancher.yaml was not copied
	dstLancherYaml := filepath.Join(dstDir, config.ConfigFileNames[0])
	if _, err := os.Stat(dstLancherYaml); !os.IsNotExist(err) {
		t.Errorf(".lancher.yaml should not be copied, but it exists")
	}

	// Verify normal file was copied
	dstNormalFile := filepath.Join(dstDir, "test.txt")
	if _, err := os.Stat(dstNormalFile); os.IsNotExist(err) {
		t.Errorf("test.txt should be copied, but it doesn't exist")
	}
}

// TestCreateCommandLancherIgnorePriority verifies .lancherignore takes priority over config ignore
func TestCreateCommandLancherIgnorePriority(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Config ignores "dist" but NOT "vendor"
	cfg := &config.Config{
		Ignore: []string{"dist"},
	}

	// .lancherignore ignores "vendor" but NOT "dist"
	ignoreContent := "vendor\n"
	os.WriteFile(filepath.Join(srcDir, ".lancherignore"), []byte(ignoreContent), 0644)

	// Create both directories
	os.MkdirAll(filepath.Join(srcDir, "dist"), 0755)
	os.WriteFile(filepath.Join(srcDir, "dist", "bundle.js"), []byte("bundle"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "vendor"), 0755)
	os.WriteFile(filepath.Join(srcDir, "vendor", "lib.go"), []byte("lib"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)

	err := copyTemplate(srcDir, dstDir, cfg)
	if err != nil {
		t.Fatalf("copyTemplate failed: %v", err)
	}

	// .lancherignore takes priority: "vendor" should be excluded
	if _, err := os.Stat(filepath.Join(dstDir, "vendor")); !os.IsNotExist(err) {
		t.Error("vendor should be excluded by .lancherignore (has priority)")
	}

	// "dist" should NOT be excluded (config ignore is overridden by .lancherignore)
	if _, err := os.Stat(filepath.Join(dstDir, "dist")); os.IsNotExist(err) {
		t.Error("dist should be copied because .lancherignore takes priority and doesn't list it")
	}

	// Normal files should be copied
	if _, err := os.Stat(filepath.Join(dstDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should be copied")
	}
}

// TestCreateCommandFallbackToConfigIgnore verifies config ignore is used when .lancherignore is absent
func TestCreateCommandFallbackToConfigIgnore(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// No .lancherignore — config ignore should be used as fallback
	cfg := &config.Config{
		Ignore: []string{"dist", "*.tmp"},
	}

	os.MkdirAll(filepath.Join(srcDir, "dist"), 0755)
	os.WriteFile(filepath.Join(srcDir, "dist", "bundle.js"), []byte("bundle"), 0644)
	os.WriteFile(filepath.Join(srcDir, "cache.tmp"), []byte("temp"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)

	err := copyTemplate(srcDir, dstDir, cfg)
	if err != nil {
		t.Fatalf("copyTemplate failed: %v", err)
	}

	// dist should be excluded by config fallback
	if _, err := os.Stat(filepath.Join(dstDir, "dist")); !os.IsNotExist(err) {
		t.Error("dist should be excluded by config ignore (fallback)")
	}

	// *.tmp should be excluded by config fallback
	if _, err := os.Stat(filepath.Join(dstDir, "cache.tmp")); !os.IsNotExist(err) {
		t.Error("cache.tmp should be excluded by config ignore (fallback)")
	}

	// main.go should be copied
	if _, err := os.Stat(filepath.Join(dstDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should be copied")
	}
}

// TestCreateCommandExcludesLancherIgnoreFile verifies .lancherignore is not copied to destination
func TestCreateCommandExcludesLancherIgnoreFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create .lancherignore and a normal file
	os.WriteFile(filepath.Join(srcDir, ".lancherignore"), []byte("*.log\n"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)

	err := copyTemplate(srcDir, dstDir, nil)
	if err != nil {
		t.Fatalf("copyTemplate failed: %v", err)
	}

	// .lancherignore should NOT be copied
	if _, err := os.Stat(filepath.Join(dstDir, ".lancherignore")); !os.IsNotExist(err) {
		t.Error(".lancherignore should not be copied to destination")
	}

	// main.go should be copied
	if _, err := os.Stat(filepath.Join(dstDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should be copied")
	}
}
