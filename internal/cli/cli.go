package cli

import (
	"fmt"

	"github.com/lancher-dev/lancher/internal/cli/commands"
	"github.com/lancher-dev/lancher/internal/cli/shared"
	"github.com/lancher-dev/lancher/internal/cli/template"
	"github.com/lancher-dev/lancher/internal/version"
)

// Run executes the CLI command based on arguments
func Run(args []string) error {
	if len(args) == 0 {
		return runHelp()
	}

	command := args[0]
	commandArgs := args[1:]

	switch command {
	case "create":
		// Check for help flag
		if len(commandArgs) > 0 && (commandArgs[0] == "help" || commandArgs[0] == "-h" || commandArgs[0] == "--help") {
			return commands.RunCreateHelp()
		}
		return commands.Run(commandArgs)
	case "template":
		if len(commandArgs) == 0 {
			return template.RunHelp()
		}
		subcommand := commandArgs[0]
		subArgs := commandArgs[1:]
		switch subcommand {
		case "add":
			// Check for help flag
			if len(subArgs) > 0 && (subArgs[0] == "help" || subArgs[0] == "-h" || subArgs[0] == "--help") {
				return template.RunAddHelp()
			}
			return template.RunAdd(subArgs)
		case "list", "ls":
			// Check for help flag
			if len(subArgs) > 0 && (subArgs[0] == "help" || subArgs[0] == "-h" || subArgs[0] == "--help") {
				return template.RunListHelp()
			}
			return template.RunList(subArgs)
		case "update":
			// Check for help flag
			if len(subArgs) > 0 && (subArgs[0] == "help" || subArgs[0] == "-h" || subArgs[0] == "--help") {
				return template.RunUpdateHelp()
			}
			return template.RunUpdate(subArgs)
		case "remove", "rm":
			// Check for help flag
			if len(subArgs) > 0 && (subArgs[0] == "help" || subArgs[0] == "-h" || subArgs[0] == "--help") {
				return template.RunRemoveHelp()
			}
			return template.RunRemove(subArgs)
		case "help", "-h", "--help":
			return template.RunHelp()
		default:
			usage := "USAGE:\n    lancher template <SUBCOMMAND> [ARGS...] [OPTIONS]"
			return shared.FormatUnknownSubcommandError(subcommand, "lancher template", usage)
		}
	case "templates":
		// Alias for template ls
		// Check for help flag
		if len(commandArgs) > 0 && (commandArgs[0] == "help" || commandArgs[0] == "-h" || commandArgs[0] == "--help") {
			return template.RunListHelp()
		}
		return template.RunList(commandArgs)
	case "-v", "--version":
		fmt.Printf("lancher %s\n", version.Get())
		return nil
	case "help", "-h", "--help":
		return runHelp()
	default:
		usage := "USAGE:\n    lancher <COMMAND> [ARGS...] [OPTIONS]"
		return shared.FormatUnknownCommandError(command, usage, "lancher ")
	}
}

// runHelp displays usage information
func runHelp() error {
	fmt.Printf("%slancher%s %s%s%s\n", shared.ColorGreen+shared.ColorBold, shared.ColorReset, shared.ColorBold, version.Get(), shared.ColorReset)
	fmt.Printf("A minimal local project template manager\n\n")

	fmt.Printf("%sUSAGE:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    lancher <command> [args...] [options]\n\n")

	fmt.Printf("%sCOMMANDS:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    %s%-20s%s %s\n", shared.ColorGreen, "create", shared.ColorReset, "Create a new project from template")
	fmt.Printf("    %s%-20s%s %s\n", shared.ColorGreen, "template", shared.ColorReset, "Manage templates (add, list, update, remove)")
	fmt.Printf("    %s%-20s%s %s\n", shared.ColorGreen, "templates", shared.ColorReset, "List all available templates")
	fmt.Printf("    %shelp%s, %s-h%s             %s\n\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, "Print this help message")

	fmt.Printf("%sOPTIONS:%s\n", shared.ColorCyan+shared.ColorBold, shared.ColorReset)
	fmt.Printf("    %s-v%s, %s--version%s        %s\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, "Print version information")
	fmt.Printf("    %s-h%s, %s--help%s           %s\n\n", shared.ColorGreen, shared.ColorReset, shared.ColorGreen, shared.ColorReset, "Show help for any command")
	return nil
}
