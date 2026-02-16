package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Kasui92/lancher/internal/cli/shared"
	"github.com/Kasui92/lancher/internal/fileutil"
	"github.com/Kasui92/lancher/internal/storage"
)

// isGitURL checks if source is a git URL
func isGitURL(source string) bool {
	return strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasSuffix(source, ".git")
}

// isGitHubAlias checks if source uses gh: alias
func isGitHubAlias(source string) bool {
	return strings.HasPrefix(source, "gh:")
}

// isGitLabAlias checks if source uses gl: alias
func isGitLabAlias(source string) bool {
	return strings.HasPrefix(source, "gl:")
}

// ensureGitSuffix adds .git suffix if not present
func ensureGitSuffix(url string) string {
	if !strings.HasSuffix(url, ".git") {
		return url + ".git"
	}
	return url
}

// isZipFile checks if source is a ZIP file
func isZipFile(source string) bool {
	return strings.HasSuffix(strings.ToLower(source), ".zip")
}

// RunAddHelp displays help for template add command
func RunAddHelp() error {
	fmt.Printf("%slancher template add%s\n", shared.ColorGreen+shared.ColorBold, shared.ColorReset)
	fmt.Printf("Add a new template\n\n")

	fmt.Printf("%sUSAGE:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    lancher template add [name] [source] [options]\n\n")

	fmt.Printf("%sARGS:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    %s%-15s%s %s\n", shared.ColorGreen, "name", shared.ColorReset, "Template name (interactive if omitted)")
	fmt.Printf("    %s%-15s%s %s\n\n", shared.ColorGreen, "source", shared.ColorReset, "Local path, ZIP file, git URL, or alias (gh:, gl:)")

	fmt.Printf("%sALIASES:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    %sgh:%s<repo>     %sGitHub repository (uses GitHub CLI if available)%s\n", shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %sgl:%s<repo>     %sGitLab repository (uses GitLab CLI if available)%s\n\n", shared.ColorGreen, shared.ColorReset, "", "")

	fmt.Printf("%sOPTIONS:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    %s-p%s, %s--print%s  %sShow detailed output (no spinner)%s\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s-h%s, %s--help%s   %sShow this help message%s\n\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, "", "")

	return nil
}

// runAdd adds a new template from path or git repository
func RunAdd(args []string) error {
	var name, source string
	var verbose bool

	// Parse flags first
	for i := 0; i < len(args); i++ {
		if args[i] == "-p" || args[i] == "--print" {
			verbose = true
			// Remove flag from args
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	// Interactive mode if no arguments provided
	if len(args) == 0 {
		nameInput, err := shared.PromptStringWithDefault("Enter template name:", "my-template")
		if err != nil {
			if strings.Contains(err.Error(), "cancelled") {
				fmt.Printf("%sCancelled.%s\n", shared.ColorYellow, shared.ColorReset)
				return nil
			}
			return shared.FormatError("failed to read input")
		}
		name = nameInput
		fmt.Printf("%s✓ Template name:%s %s\n", shared.ColorGreen, shared.ColorReset, name)

		if name == "" {
			return shared.FormatError("Template name cannot be empty")
		}

		sourceInput, err := shared.PromptStringWithDefault("Enter source (local path, git URL, or ZIP file):", ".")
		if err != nil {
			if strings.Contains(err.Error(), "cancelled") {
				fmt.Printf("%sCancelled.%s\n", shared.ColorYellow, shared.ColorReset)
				return nil
			}
			return shared.FormatError("failed to read input")
		}
		source = sourceInput

		if source == "" {
			return shared.FormatError("Source cannot be empty")
		}

		// Validate source before printing confirmation
		if !isGitURL(source) && !isGitHubAlias(source) && !isGitLabAlias(source) {
			// For local paths and ZIP files, verify they exist
			sourceAbs, err := filepath.Abs(source)
			if err != nil {
				return shared.FormatError(fmt.Sprintf("Invalid source path '%s'", source))
			}
			if _, err := os.Stat(sourceAbs); os.IsNotExist(err) {
				if isZipFile(source) {
					return shared.FormatError(fmt.Sprintf("ZIP file not found: '%s'", source))
				}
				return shared.FormatError(fmt.Sprintf("Directory not found: '%s'", source))
			}
		}

		fmt.Printf("%s✓ Source:%s %s\n", shared.ColorGreen, shared.ColorReset, source)
	} else {
		// Command-line arguments mode
		if len(args) < 2 {
			var missing []string
			if len(args) == 0 {
				missing = []string{"name", "source"}
			} else {
				missing = []string{"source"}
			}
			usage := "USAGE:\n    lancher template add <name> <source>"
			return shared.FormatMissingArgsError(missing, usage)
		}
		name = args[0]
		source = args[1]
	}

	// Validate template name
	if err := shared.SanitizeTemplateName(name); err != nil {
		return shared.FormatError(err.Error())
	}

	// Check if template already exists
	exists, err := storage.TemplateExists(name)
	if err != nil {
		return shared.FormatError(fmt.Sprintf("Failed to check template: %v", err))
	}
	if exists {
		return shared.FormatError(fmt.Sprintf("Template '%s' already exists", name))
	}

	// Get destination path
	destPath, err := storage.GetTemplatePath(name)
	if err != nil {
		return shared.FormatError(fmt.Sprintf("Failed to get template path: %v", err))
	}

	// Handle GitHub alias (gh:)
	if isGitHubAlias(source) {
		repoPath := strings.TrimPrefix(source, "gh:")
		return cloneWithAlias(name, repoPath, destPath, "gh", "https://github.com/", verbose)
	}

	// Handle GitLab alias (gl:)
	if isGitLabAlias(source) {
		repoPath := strings.TrimPrefix(source, "gl:")
		return cloneWithAlias(name, repoPath, destPath, "glab", "https://gitlab.com/", verbose)
	}

	// Handle git URL, ZIP file, or local path
	if isGitURL(source) {
		var spinner *shared.Spinner
		writer := shared.NewSpinnerWriter(verbose)

		if !verbose {
			spinner = shared.NewSpinner("Cloning repository...")
			spinner.Start()
			defer spinner.Stop()
		} else {
			fmt.Printf("%sCloning repository...%s\n", shared.ColorYellow, shared.ColorReset)
		}

		cmd := exec.Command("git", "clone", "--depth", "1", source, destPath)
		cmd.Stdout = writer.MultiWriter()
		cmd.Stderr = writer.MultiWriter()
		if err := cmd.Run(); err != nil {
			if spinner != nil {
				spinner.Fail(fmt.Sprintf("Failed to clone repository: %v", err))
			}
			return shared.FormatError(fmt.Sprintf("Failed to clone repository: %v", err))
		}

		// Apply .lancherignore patterns after cloning
		if err := fileutil.ApplyIgnorePatterns(destPath); err != nil {
			if spinner != nil {
				spinner.Fail(fmt.Sprintf("Failed to apply ignore patterns: %v", err))
			}
			return shared.FormatError(fmt.Sprintf("Failed to apply ignore patterns: %v", err))
		}

		if spinner != nil {
			spinner.Success(fmt.Sprintf("Template '%s' added from git repository", name))
		} else {
			fmt.Printf("%s✓ Template '%s' added from git repository%s\n", shared.ColorGreen, name, shared.ColorReset)
		}
		fmt.Printf("  %sSource:%s %s\n", shared.ColorYellow, shared.ColorReset, source)
	} else if isZipFile(source) {
		// ZIP file
		sourceAbs, err := filepath.Abs(source)
		if err != nil {
			return shared.FormatError(fmt.Sprintf("Invalid source path: %v", err))
		}

		if _, err := os.Stat(sourceAbs); os.IsNotExist(err) {
			return shared.FormatError(fmt.Sprintf("ZIP file not found: '%s'", sourceAbs))
		}

		var spinner *shared.Spinner
		if !verbose {
			spinner = shared.NewSpinner("Extracting ZIP file...")
			spinner.Start()
			defer spinner.Stop()
		} else {
			fmt.Printf("%sExtracting ZIP file...%s\n", shared.ColorYellow, shared.ColorReset)
		}

		if err := fileutil.UnzipToDir(sourceAbs, destPath); err != nil {
			if spinner != nil {
				spinner.Fail(fmt.Sprintf("Failed to extract ZIP: %v", err))
			}
			return shared.FormatError(fmt.Sprintf("Failed to extract ZIP: %v", err))
		}

		// Apply .lancherignore patterns after extraction
		if err := fileutil.ApplyIgnorePatterns(destPath); err != nil {
			if spinner != nil {
				spinner.Fail(fmt.Sprintf("Failed to apply ignore patterns: %v", err))
			}
			return shared.FormatError(fmt.Sprintf("Failed to apply ignore patterns: %v", err))
		}

		if spinner != nil {
			spinner.Success(fmt.Sprintf("Template '%s' added from ZIP file", name))
		} else {
			fmt.Printf("%s✓ Template '%s' added from ZIP file%s\n", shared.ColorGreen, name, shared.ColorReset)
		}
		fmt.Printf("  %sSource:%s %s\n", shared.ColorYellow, shared.ColorReset, sourceAbs)
	} else {
		// Local path
		sourceAbs, err := filepath.Abs(source)
		if err != nil {
			return shared.FormatError(fmt.Sprintf("Invalid source path: %v", err))
		}

		if _, err := os.Stat(sourceAbs); os.IsNotExist(err) {
			return shared.FormatError(fmt.Sprintf("Directory not found: '%s'", sourceAbs))
		}

		// Copy directory
		if err := fileutil.CopyDir(sourceAbs, destPath); err != nil {
			return shared.FormatError(fmt.Sprintf("Failed to copy template: %v", err))
		}

		fmt.Printf("%s✓ Template '%s' added successfully%s\n", shared.ColorGreen, name, shared.ColorReset)
		fmt.Printf("  %sSource:%s %s\n", shared.ColorYellow, shared.ColorReset, sourceAbs)
	}

	fmt.Printf("  %sStored:%s %s\n", shared.ColorYellow, shared.ColorReset, destPath)

	return nil
}

// cloneWithAlias handles cloning with gh: or gl: alias
// cliCmd is "gh" for GitHub or "glab" for GitLab
// baseURL is the base HTTPS URL for the platform
func cloneWithAlias(name, repoPath, destPath, cliCmd, baseURL string, verbose bool) error {
	var spinner *shared.Spinner
	writer := shared.NewSpinnerWriter(verbose)

	var cmd *exec.Cmd
	var sourceDisplay string

	if shared.CommandExists(cliCmd) {
		// Use CLI tool (gh repo clone or glab repo clone)
		if !verbose {
			spinner = shared.NewSpinner(fmt.Sprintf("Cloning with CLI..."))
			spinner.Start()
			defer spinner.Stop()
		} else {
			fmt.Printf("%sCloning with CLI...%s\n", shared.ColorYellow, shared.ColorReset)
		}

		cmd = exec.Command(cliCmd, "repo", "clone", repoPath, destPath, "--", "--depth", "1")
		sourceDisplay = fmt.Sprintf("%s:%s", cliCmd, repoPath)
	} else {
		// Fallback to git clone with HTTPS URL
		gitURL := baseURL + repoPath
		gitURL = ensureGitSuffix(gitURL)

		if !verbose {
			spinner = shared.NewSpinner("Cloning repository...")
			spinner.Start()
			defer spinner.Stop()
		} else {
			fmt.Printf("%sCloning repository...%s\n", shared.ColorYellow, shared.ColorReset)
		}

		cmd = exec.Command("git", "clone", "--depth", "1", gitURL, destPath)
		sourceDisplay = gitURL
	}

	cmd.Stdout = writer.MultiWriter()
	cmd.Stderr = writer.MultiWriter()

	if err := cmd.Run(); err != nil {
		if spinner != nil {
			spinner.Fail(fmt.Sprintf("Failed to clone repository: %v", err))
		}
		return shared.FormatError(fmt.Sprintf("Failed to clone repository: %v", err))
	}

	// Apply .lancherignore patterns after cloning
	if err := fileutil.ApplyIgnorePatterns(destPath); err != nil {
		if spinner != nil {
			spinner.Fail(fmt.Sprintf("Failed to apply ignore patterns: %v", err))
		}
		return shared.FormatError(fmt.Sprintf("Failed to apply ignore patterns: %v", err))
	}

	if spinner != nil {
		spinner.Success(fmt.Sprintf("Template '%s' added from repository", name))
	} else {
		fmt.Printf("%s✓ Template '%s' added from repository%s\n", shared.ColorGreen, name, shared.ColorReset)
	}
	fmt.Printf("  %sSource:%s %s\n", shared.ColorYellow, shared.ColorReset, sourceDisplay)
	fmt.Printf("  %sStored:%s %s\n", shared.ColorYellow, shared.ColorReset, destPath)

	return nil
}
