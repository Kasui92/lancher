package tests

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kasui92/lancher/internal/fileutil"
)

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	content := []byte("test content")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "dest.txt")
	if err := fileutil.CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile() failed: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", dstContent, content)
	}
}

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dest")
	if err := fileutil.CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir() failed: %v", err)
	}

	// Verify structure
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt was not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("subdir/file2.txt was not copied")
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
	if string(content) != "content2" {
		t.Errorf("Content mismatch in subdirectory file")
	}
}

func TestRemoveDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory to remove
	dirToRemove := filepath.Join(tmpDir, "to-remove")
	os.MkdirAll(dirToRemove, 0755)
	os.WriteFile(filepath.Join(dirToRemove, "file.txt"), []byte("content"), 0644)

	// Remove directory
	if err := fileutil.RemoveDir(dirToRemove); err != nil {
		t.Fatalf("RemoveDir() failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(dirToRemove); !os.IsNotExist(err) {
		t.Error("Directory was not removed")
	}
}

func TestUnzipToDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ZIP file
	zipPath := filepath.Join(tmpDir, "test.zip")
	if err := createTestZip(zipPath); err != nil {
		t.Fatalf("Failed to create test ZIP: %v", err)
	}

	// Extract ZIP
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := fileutil.UnzipToDir(zipPath, extractDir); err != nil {
		t.Fatalf("UnzipToDir() failed: %v", err)
	}

	// Verify extracted structure
	if _, err := os.Stat(filepath.Join(extractDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt was not extracted")
	}
	if _, err := os.Stat(filepath.Join(extractDir, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("subdir/file2.txt was not extracted")
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(extractDir, "file1.txt"))
	if string(content) != "content1" {
		t.Errorf("Content mismatch: got %q, want %q", content, "content1")
	}

	content2, _ := os.ReadFile(filepath.Join(extractDir, "subdir", "file2.txt"))
	if string(content2) != "content2" {
		t.Errorf("Content mismatch in subdirectory: got %q, want %q", content2, "content2")
	}
}

// createTestZip creates a test ZIP file with some sample content
func createTestZip(zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	// Add file1.txt
	f1, err := w.Create("file1.txt")
	if err != nil {
		return err
	}
	if _, err := f1.Write([]byte("content1")); err != nil {
		return err
	}

	// Add subdir/file2.txt
	f2, err := w.Create("subdir/file2.txt")
	if err != nil {
		return err
	}
	if _, err := f2.Write([]byte("content2")); err != nil {
		return err
	}

	return nil
}

func TestApplyIgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .lancherignore
	ignoreContent := "node_modules\n*.log\n"
	os.WriteFile(filepath.Join(tmpDir, ".lancherignore"), []byte(ignoreContent), 0644)

	// Create files and directories
	os.MkdirAll(filepath.Join(tmpDir, "node_modules", "pkg"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "node_modules", "pkg", "index.js"), []byte("module"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "app.log"), []byte("log data"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "server.log"), []byte("nested log"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "handler.go"), []byte("handler"), 0644)

	err := fileutil.ApplyIgnorePatterns(tmpDir)
	if err != nil {
		t.Fatalf("ApplyIgnorePatterns failed: %v", err)
	}

	// node_modules should be removed
	if _, err := os.Stat(filepath.Join(tmpDir, "node_modules")); !os.IsNotExist(err) {
		t.Error("node_modules should be removed")
	}

	// app.log should be removed
	if _, err := os.Stat(filepath.Join(tmpDir, "app.log")); !os.IsNotExist(err) {
		t.Error("app.log should be removed")
	}

	// src/server.log should be removed (recursive)
	if _, err := os.Stat(filepath.Join(tmpDir, "src", "server.log")); !os.IsNotExist(err) {
		t.Error("src/server.log should be removed")
	}

	// main.go should remain
	if _, err := os.Stat(filepath.Join(tmpDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should remain")
	}

	// src/handler.go should remain
	if _, err := os.Stat(filepath.Join(tmpDir, "src", "handler.go")); os.IsNotExist(err) {
		t.Error("src/handler.go should remain")
	}
}

// TestCopyDirRespectsLancherIgnore verifies that CopyDir integrates with .lancherignore
// This is a basic integration test to ensure the ignore mechanism works end-to-end.
func TestCopyDirRespectsLancherIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory with various files
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(filepath.Join(srcDir, "node_modules", "pkg"), 0755)
	os.WriteFile(filepath.Join(srcDir, "node_modules", "pkg", "index.js"), []byte("module"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(srcDir, "test.log"), []byte("log"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "src"), 0755)
	os.WriteFile(filepath.Join(srcDir, "src", "handler.go"), []byte("handler"), 0644)

	// Create .lancherignore with basic patterns
	ignoreContent := `node_modules
*.log
`
	os.WriteFile(filepath.Join(srcDir, ".lancherignore"), []byte(ignoreContent), 0644)

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dest")
	if err := fileutil.CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir() failed: %v", err)
	}

	// Verify ignored items are not copied
	if _, err := os.Stat(filepath.Join(dstDir, "node_modules")); !os.IsNotExist(err) {
		t.Error("node_modules should not be copied (ignored by .lancherignore)")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "test.log")); !os.IsNotExist(err) {
		t.Error("test.log should not be copied (*.log pattern)")
	}

	// Verify non-ignored items are copied
	if _, err := os.Stat(filepath.Join(dstDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should be copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "src", "handler.go")); os.IsNotExist(err) {
		t.Error("src/handler.go should be copied")
	}
}
