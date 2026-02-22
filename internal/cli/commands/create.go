package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lancher-dev/lancher/internal/cli/shared"
	"github.com/lancher-dev/lancher/internal/config"
	"github.com/lancher-dev/lancher/internal/fileutil"
	"github.com/lancher-dev/lancher/internal/storage"
)

// RunCreateHelp displays help for create command
func RunCreateHelp() error {
	fmt.Printf("%slancher create%s\n", shared.ColorGreen+shared.ColorBold, shared.ColorReset)
	fmt.Printf("Create a new project from template\n\n")

	fmt.Printf("%sUSAGE:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    lancher create [options]\n\n")

	fmt.Printf("%sOPTIONS:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    %s-t%s, %s--template%s %s<name>%s     %sTemplate name to use%s\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, shared.ColorCyan, shared.ColorReset, "", "")
	fmt.Printf("    %s-d%s, %s--destination%s %s<path>%s  %sDestination directory for the project%s\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, shared.ColorCyan, shared.ColorReset, "", "")
	fmt.Printf("    %s    --git%s                 %sInitialize git repository automatically%s\n", shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s    --no-git%s              %sSkip git initialization prompt%s\n", shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s    --hooks%s               %sExecute hooks automatically (skip prompt)%s\n", shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s    --no-hooks%s            %sSkip hooks execution%s\n", shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s    --keep-config%s         %sKeep lancher config and ignore files in project%s\n", shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s    --no-keep-config%s      %sRemove lancher config and ignore files from project%s\n", shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s-p%s, %s--print%s               %sShow detailed output (no spinner)%s\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, "", "")
	fmt.Printf("    %s-h%s, %s--help%s                %sShow this help message%s\n\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, "", "")

	return nil
}

// runCreate creates a new project from a template
func Run(args []string) error {
	var templateName, destination string
	var verbose, gitInit, noGit, executeHooks, noHooks, keepConfig, noKeepConfig bool

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t", "--template":
			if i+1 < len(args) {
				templateName = args[i+1]
				i++
			} else {
				return shared.FormatError("flag -t/--template requires a value")
			}
		case "-d", "--destination":
			if i+1 < len(args) {
				destination = args[i+1]
				i++
			} else {
				return shared.FormatError("flag -d/--destination requires a value")
			}
		case "--git":
			gitInit = true
		case "--no-git":
			noGit = true
		case "--hooks":
			executeHooks = true
		case "--no-hooks":
			noHooks = true
		case "--keep-config":
			keepConfig = true
		case "--no-keep-config":
			noKeepConfig = true
		case "-p", "--print":
			verbose = true
		default:
			if strings.HasPrefix(args[i], "-") {
				usage := "USAGE:\n    lancher create [OPTIONS]"
				return shared.FormatUnknownCommandError(args[i], usage, "lancher create ")
			}
			// Treat as positional argument (ignore for now, could be enhanced later)
		}
	}

	// Validate mutually exclusive flags
	if gitInit && noGit {
		return shared.FormatError("cannot use both --git and --no-git flags")
	}
	if executeHooks && noHooks {
		return shared.FormatError("cannot use both --hooks and --no-hooks flags")
	}
	if keepConfig && noKeepConfig {
		return shared.FormatError("cannot use both --keep-config and --no-keep-config flags")
	}

	// Interactive mode if flags not provided
	if templateName == "" {
		// List available templates
		templates, err := storage.ListTemplates()
		if err != nil {
			return shared.FormatError(fmt.Sprintf("failed to list templates: %v", err))
		}

		if len(templates) == 0 {
			fmt.Printf("%sNo templates found.%s\n", shared.ColorYellow, shared.ColorReset)
			fmt.Printf("Add a template with: %slancher template add <name> <source_dir>%s\n", shared.ColorCyan, shared.ColorReset)
			return nil
		}

		// Use interactive select
		selectedTemplate, err := shared.Select("Choose a template:", templates)
		if err != nil {
			if strings.Contains(err.Error(), "cancelled") {
				fmt.Printf("%sCancelled.%s\n", shared.ColorYellow, shared.ColorReset)
				return nil
			}
			return shared.FormatError(fmt.Sprintf("selection failed: %v", err))
		}
		templateName = selectedTemplate

		// Show selected template
		fmt.Printf("%s✓ Template selected:%s %s\n", shared.ColorGreen, shared.ColorReset, templateName)
	}

	if destination == "" {
		dest, err := shared.PromptStringWithDefault("Enter destination directory:", "my-app")
		if err != nil {
			if strings.Contains(err.Error(), "cancelled") {
				fmt.Printf("%sCancelled.%s\n", shared.ColorYellow, shared.ColorReset)
				return nil
			}
			return shared.FormatError("failed to read input")
		}
		destination = dest
		fmt.Printf("%s✓ Destination set:%s %s\n", shared.ColorGreen, shared.ColorReset, destination)
	}

	// Validate template name
	if err := shared.SanitizeTemplateName(templateName); err != nil {
		return shared.FormatError(err.Error())
	}

	// Check if template exists
	exists, err := storage.TemplateExists(templateName)
	if err != nil {
		return shared.FormatError(fmt.Sprintf("failed to check template: %v", err))
	}
	if !exists {
		return shared.FormatError(fmt.Sprintf("template '%s' not found", templateName))
	}

	// Handle destination path resolution
	var destAbs string

	// Handle "." as current directory
	if destination == "." {
		destAbs, err = os.Getwd()
		if err != nil {
			return shared.FormatError(fmt.Sprintf("failed to get current directory: %v", err))
		}
	} else {
		// Get absolute path
		destAbs, err = filepath.Abs(destination)
		if err != nil {
			return shared.FormatError(fmt.Sprintf("invalid destination path: %v", err))
		}
	}

	// Check if parent directory exists for nested paths
	parentDir := filepath.Dir(destAbs)
	if parentDir != "." && parentDir != "/" {
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return shared.FormatError(fmt.Sprintf("parent directory does not exist: %s", parentDir))
		}
	}

	// Check if destination already exists
	if stat, err := os.Stat(destAbs); err == nil {
		// Destination exists - check if it's a directory
		if !stat.IsDir() {
			return shared.FormatError(fmt.Sprintf("destination exists and is not a directory: %s", destAbs))
		}

		// Check if directory is empty
		entries, err := os.ReadDir(destAbs)
		if err != nil {
			return shared.FormatError(fmt.Sprintf("failed to read destination directory: %v", err))
		}

		if len(entries) > 0 {
			// Directory is not empty - ask for confirmation
			fmt.Printf("%s⚠ Warning:%s Destination directory is not empty (%d items)\n", shared.ColorYellow, shared.ColorReset, len(entries))
			fmt.Printf("  %sLocation:%s %s\n", shared.ColorYellow, shared.ColorReset, destAbs)

			confirmed, err := shared.PromptConfirmWithDefault("Do you want to remove and recreate the directory?", false)
			if err != nil {
				return shared.FormatError("failed to read confirmation")
			}

			if !confirmed {
				fmt.Printf("%sCancelled.%s\n", shared.ColorYellow, shared.ColorReset)
				return nil
			}

			// Remove existing directory
			if err := os.RemoveAll(destAbs); err != nil {
				return shared.FormatError(fmt.Sprintf("failed to remove existing directory: %v", err))
			}
			fmt.Printf("%s✓ Removed existing directory%s\n", shared.ColorGreen, shared.ColorReset)
		}
	}

	// Get template path
	templatePath, err := storage.GetTemplatePath(templateName)
	if err != nil {
		return shared.FormatError(fmt.Sprintf("failed to get template path: %v", err))
	}

	// Load template configuration if exists
	cfg, err := config.LoadConfig(templatePath)
	if err != nil {
		return shared.FormatError(fmt.Sprintf("failed to load template config: %v", err))
	}

	// Warn about deprecated 'ignore' field in config YAML
	if cfg != nil && cfg.HasIgnore() {
		fmt.Printf("%s⚠ Warning:%s The 'ignore' field in lancher.yml is deprecated. Use a .lancherignore file instead.%s\n", shared.ColorYellow, shared.ColorReset, shared.ColorReset)
	}

	// Display template metadata if available
	if cfg != nil {
		metadata := cfg.GetMetadata()
		if metadata != "" {
			fmt.Printf("\n%sTemplate Information:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
			fmt.Print(metadata)
			fmt.Println()
		}
	}

	// Copy template to destination
	var spinner *shared.Spinner
	if !verbose {
		spinner = shared.NewSpinner("Creating project...")
		spinner.Start()
		defer spinner.Stop()
	} else {
		fmt.Printf("%sCreating project...%s\n", shared.ColorYellow, shared.ColorReset)
	}

	if err := copyTemplate(templatePath, destAbs, cfg); err != nil {
		if spinner != nil {
			spinner.Fail(fmt.Sprintf("Failed to create project: %v", err))
		}
		return shared.FormatError(fmt.Sprintf("failed to create project: %v", err))
	}

	if spinner != nil {
		spinner.Success(fmt.Sprintf("Project created successfully from template '%s'", templateName))
	} else {
		fmt.Printf("%s✓ Project created successfully from template '%s'%s\n", shared.ColorGreen, templateName, shared.ColorReset)
	}
	fmt.Printf("  %sLocation:%s %s\n", shared.ColorYellow, shared.ColorReset, destAbs)

	// Execute hooks if defined
	if cfg != nil && cfg.HasHooks() && !noHooks {
		fmt.Printf("\n%sHooks found:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
		for i, hook := range cfg.Hooks {
			fmt.Printf("  %d. %s\n", i+1, hook)
		}
		fmt.Println()

		// Check for lancher commands in hooks (prevent infinite loops)
		if hasLancherCommands(cfg.Hooks) {
			fmt.Printf("%s⚠ Warning: Hooks contain lancher commands - skipping execution to prevent infinite loops%s\n", shared.ColorYellow, shared.ColorReset)
			fmt.Printf("%sPlease remove lancher commands from hooks and run them manually if needed%s\n", shared.ColorYellow, shared.ColorReset)
		} else {
			var confirmed bool
			if executeHooks {
				confirmed = true
			} else {
				var err error
				confirmed, err = shared.PromptConfirmWithDefault("Execute hooks?", true)
				if err != nil {
					return shared.FormatError("failed to read confirmation")
				}
			}

			if confirmed {
				if err := runHooks(cfg.Hooks, destAbs); err != nil {
					fmt.Printf("%s⚠ Some hooks failed: %v%s\n", shared.ColorYellow, err, shared.ColorReset)
				} else {
					fmt.Printf("%s✓ All hooks executed successfully%s\n", shared.ColorGreen, shared.ColorReset)
				}
			} else {
				fmt.Printf("%sSkipped hooks%s\n", shared.ColorYellow, shared.ColorReset)
			}
		}
	}

	// Ask to initialize git repository (if not set via flag)
	fmt.Println()
	if !noGit {
		if !gitInit {
			var err error
			gitInit, err = shared.PromptConfirmWithDefault("Initialize git repository?", false)
			if err != nil {
				return shared.FormatError("failed to read confirmation")
			}
		}

		if gitInit {
			cmd := exec.Command("git", "init")
			cmd.Dir = destAbs
			if output, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("%s⚠ Failed to initialize git: %v%s\n", shared.ColorYellow, err, shared.ColorReset)
				if len(output) > 0 {
					fmt.Printf("%s%s%s\n", shared.ColorGray, string(output), shared.ColorReset)
				}
			} else {
				fmt.Printf("%s✓ Git repository initialized%s\n", shared.ColorGreen, shared.ColorReset)
			}
		} else {
			fmt.Printf("%sSkipped git initialization%s\n", shared.ColorYellow, shared.ColorReset)
		}
	}

	// Handle lancher config and .lancherignore files in destination
	var copiedMetaFiles []string
	for _, cfgFile := range config.ConfigFileNames {
		if _, err := os.Stat(filepath.Join(destAbs, cfgFile)); err == nil {
			copiedMetaFiles = append(copiedMetaFiles, cfgFile)
			break // only one config file can be present
		}
	}
	if _, err := os.Stat(filepath.Join(destAbs, fileutil.IgnoreFileName)); err == nil {
		copiedMetaFiles = append(copiedMetaFiles, fileutil.IgnoreFileName)
	}

	if len(copiedMetaFiles) > 0 {
		fmt.Println()
		if noKeepConfig {
			for _, f := range copiedMetaFiles {
				_ = os.Remove(filepath.Join(destAbs, f))
			}
			fmt.Printf("%s✓ Removed lancher config files from project%s\n", shared.ColorGreen, shared.ColorReset)
		} else if !keepConfig {
			fmt.Printf("%sLancher files found in project:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
			for _, f := range copiedMetaFiles {
				fmt.Printf("  • %s\n", f)
			}
			confirmed, err := shared.PromptConfirmWithDefault("Keep lancher config and ignore files in the project?", false)
			if err != nil {
				return shared.FormatError("failed to read confirmation")
			}
			if !confirmed {
				for _, f := range copiedMetaFiles {
					_ = os.Remove(filepath.Join(destAbs, f))
				}
				fmt.Printf("%s✓ Removed lancher config files from project%s\n", shared.ColorGreen, shared.ColorReset)
			} else {
				fmt.Printf("%s✓ Kept lancher config files in project%s\n", shared.ColorGreen, shared.ColorReset)
			}
		}
	}

	return nil
}

// copyTemplate copies template directory respecting ignore patterns
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

// hasLancherCommands checks if any hook contains lancher commands to prevent infinite loops
func hasLancherCommands(hooks []string) bool {
	for _, hook := range hooks {
		// Trim whitespace and split by common shell operators
		trimmed := strings.TrimSpace(hook)

		// Check if the command starts with "lancher" (direct command)
		if strings.HasPrefix(trimmed, "lancher ") || trimmed == "lancher" {
			return true
		}

		// Check for lancher in command chains (&&, ||, ;, |)
		// Split by these operators and check each part
		for _, part := range strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ';' || r == '|'
		}) {
			// Further split by && and ||
			for _, subpart := range strings.Split(part, "&&") {
				for _, segment := range strings.Split(subpart, "||") {
					// Get the command (first word after trimming)
					segmentTrimmed := strings.TrimSpace(segment)
					fields := strings.Fields(segmentTrimmed)
					if len(fields) > 0 && fields[0] == "lancher" {
						return true
					}
				}
			}
		}
	}
	return false
}

// runHooks runs hooks in the project directory
func runHooks(hooks []string, projectDir string) error {
	for i, hook := range hooks {
		fmt.Printf("\n%sExecuting hook %d/%d:%s %s\n", shared.ColorCyan, i+1, len(hooks), shared.ColorReset, hook)

		// Execute hook via shell to properly handle quotes and redirects
		var cmd *exec.Cmd
		cmd = exec.Command("sh", "-c", hook)

		cmd.Dir = projectDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook '%s' failed: %w", hook, err)
		}
	}
	return nil
}
