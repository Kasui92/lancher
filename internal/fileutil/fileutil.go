package fileutil

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IgnoreFileName is the name of the .lancherignore file
const IgnoreFileName = ".lancherignore"

// ignorePattern represents a normalized ignore pattern with pre-computed properties
type ignorePattern struct {
	raw        string // Original pattern
	normalized string // Normalized pattern (no leading/trailing slashes)
	isRoot     bool   // Pattern starts with /
	isRecursive bool  // Pattern contains **
	hasWildcard bool  // Pattern contains * or ?
}

// parsePattern parses and normalizes a pattern string
func parsePattern(raw string) ignorePattern {
	pattern := strings.TrimSpace(raw)
	isRoot := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")

	return ignorePattern{
		raw:         raw,
		normalized:  pattern,
		isRoot:      isRoot,
		isRecursive: strings.Contains(pattern, "**"),
		hasWildcard: strings.ContainsAny(pattern, "*?"),
	}
}

// shouldIgnore checks if a file or directory name should be ignored based on patterns.
// Used internally by CopyDir where matching is done on entry names only.
func shouldIgnore(name string, patterns []string) bool {
	for _, raw := range patterns {
		if raw == "" {
			continue
		}

		p := parsePattern(raw)
		pattern := p.normalized

		if pattern == "" {
			continue
		}

		// Handle recursive patterns - simplified for name matching
		if p.isRecursive {
			pattern = strings.ReplaceAll(pattern, "**", "*")
		}

		// Exact match
		if name == pattern {
			return true
		}

		// Wildcard pattern (glob)
		if strings.ContainsAny(pattern, "*?") {
			if matched, _ := filepath.Match(pattern, name); matched {
				return true
			}
		}
	}
	return false
}

// ShouldIgnorePath checks if a relative path should be ignored based on patterns.
// It matches against both the full relative path and the basename, supporting
// exact match and glob patterns. Used during project creation (copyTemplate).
// Supports patterns with trailing slashes, ** for recursive matching, and / prefix for root-only matches.
func ShouldIgnorePath(relativePath string, patterns []string) bool {
	// Pre-compute path components once
	var pathParts []string
	var baseName string
	computedParts := false

	for _, raw := range patterns {
		if raw == "" {
			continue
		}

		p := parsePattern(raw)
		pattern := p.normalized

		if pattern == "" {
			continue
		}

		// Lazy compute path parts only when needed
		if !computedParts {
			pathParts = strings.Split(relativePath, string(filepath.Separator))
			baseName = filepath.Base(relativePath)
			computedParts = true
		}

		// Handle recursive patterns (**)
		if p.isRecursive {
			if matched := matchRecursivePattern(pattern, relativePath, baseName, pathParts); matched {
				return true
			}
			continue
		}

		// Root-relative pattern (starts with /)
		if p.isRoot {
			if relativePath == pattern || strings.HasPrefix(relativePath, pattern+string(filepath.Separator)) {
				return true
			}
			if p.hasWildcard {
				if matched, _ := filepath.Match(pattern, relativePath); matched {
					return true
				}
			}
			continue
		}

		// Exact matches
		if relativePath == pattern || baseName == pattern {
			return true
		}

		// Match if pattern appears anywhere in the path (for directories)
		for _, part := range pathParts {
			if part == pattern {
				return true
			}
		}

		// Wildcard/glob matching
		if p.hasWildcard {
			// Match against full path
			if matched, _ := filepath.Match(pattern, relativePath); matched {
				return true
			}
			// Match against basename
			if matched, _ := filepath.Match(pattern, baseName); matched {
				return true
			}
			// Match against any path component
			for _, part := range pathParts {
				if matched, _ := filepath.Match(pattern, part); matched {
					return true
				}
			}
		}
	}
	return false
}

// matchRecursivePattern handles matching of patterns containing ** (recursive wildcards)
func matchRecursivePattern(pattern, relativePath, baseName string, pathParts []string) bool {
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false
	}

	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	switch {
	case prefix == "" && suffix != "":
		// **/suffix pattern (e.g., **/node_modules or **/*.pyc)
		if strings.ContainsAny(suffix, "*?") {
			// Wildcard suffix - check against basename and all path components
			if matched, _ := filepath.Match(suffix, baseName); matched {
				return true
			}
			if matched, _ := filepath.Match(suffix, relativePath); matched {
				return true
			}
			for _, part := range pathParts {
				if matched, _ := filepath.Match(suffix, part); matched {
					return true
				}
			}
		} else {
			// Exact suffix - check if path ends with it or basename matches
			if strings.HasSuffix(relativePath, suffix) || baseName == suffix {
				return true
			}
			// Also check each path component
			for _, part := range pathParts {
				if part == suffix {
					return true
				}
			}
		}

	case prefix != "" && suffix == "":
		// prefix/** pattern - matches everything under prefix
		return strings.HasPrefix(relativePath, prefix) || strings.Contains(relativePath, prefix+string(filepath.Separator))

	case prefix != "" && suffix != "":
		// prefix/**/suffix pattern - check both parts exist in path
		return strings.Contains(relativePath, prefix) && strings.Contains(relativePath, suffix)
	}

	return false
}

// LoadIgnoreFile loads ignore patterns from a .lancherignore file if it exists in the given directory.
// Returns an empty slice if the file does not exist or cannot be read.
// Supported pattern syntax:
// - Lines starting with # are comments
// - Trailing slashes indicate directories (e.g., node_modules/)
// - Leading slashes match from root (e.g., /config)
// - ** matches across directories (e.g., **/node_modules)
// - * and ? wildcards supported
// - ! to negate patterns (not yet implemented)
func LoadIgnoreFile(srcDir string) []string {
	ignoreFile := filepath.Join(srcDir, IgnoreFileName)
	file, err := os.Open(ignoreFile)
	if err != nil {
		// File doesn't exist or can't be read, return empty slice
		return []string{}
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Note: We keep the pattern as-is, including:
		// - trailing slashes (will be handled in matching functions)
		// - leading slashes (for root-relative patterns)
		// - ** wildcards (for recursive matching)
		patterns = append(patterns, line)
	}

	return patterns
}

// ApplyIgnorePatterns reads the .lancherignore file from dir and removes matching files/directories.
// Used after git clone or ZIP extraction to apply ignore rules retroactively.
func ApplyIgnorePatterns(dir string) error {
	patterns := LoadIgnoreFile(dir)
	if len(patterns) == 0 {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if shouldIgnore(name, patterns) {
			path := filepath.Join(dir, name)
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove ignored path %s: %w", path, err)
			}
		}
	}

	// Recurse into remaining subdirectories
	entries, err = os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to re-read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			if err := applyIgnorePatternsRecursive(subDir, patterns); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyIgnorePatternsRecursive removes entries matching patterns within subdirectories
func applyIgnorePatternsRecursive(dir string, patterns []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if shouldIgnore(name, patterns) {
			path := filepath.Join(dir, name)
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove ignored path %s: %w", path, err)
			}
		}
	}

	// Re-read and recurse into remaining subdirectories
	entries, err = os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to re-read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			if err := applyIgnorePatternsRecursive(subDir, patterns); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyDir recursively copies a directory from src to dst
func CopyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	// Load ignore patterns from .lancherignore file if it exists
	ignorePatterns := LoadIgnoreFile(src)

	return copyDirWithIgnore(src, dst, src, ignorePatterns)
}

// copyDirWithIgnore is the internal function that handles recursive copying with ignore patterns
func copyDirWithIgnore(src, dst, rootSrc string, ignorePatterns []string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		name := entry.Name()

		// Skip if should be ignored
		if shouldIgnore(name, ignorePatterns) {
			continue
		}

		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDirWithIgnore(srcPath, dstPath, rootSrc, ignorePatterns); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyFile copies a single file from src to dst
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// RemoveDir removes a directory and all its contents
func RemoveDir(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}
	return nil
}

// UnzipToDir extracts a ZIP file to the specified directory
func UnzipToDir(zipPath, destDir string) error {
	// Open ZIP file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract each file
	for _, file := range reader.File {
		// Construct destination path
		destPath := filepath.Join(destDir, file.Name)

		// Security check: prevent zip slip vulnerability
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// Create directory
			if err := os.MkdirAll(destPath, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Extract file
			if err := extractZipFile(file, destPath); err != nil {
				return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
			}
		}
	}

	return nil
}

// extractZipFile extracts a single file from ZIP
func extractZipFile(file *zip.File, destPath string) error {
	// Open file in ZIP
	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}
