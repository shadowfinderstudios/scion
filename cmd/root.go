/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ptone/scion-agent/pkg/apiclient"
	"github.com/ptone/scion-agent/pkg/config"
	"github.com/ptone/scion-agent/pkg/util"
	"github.com/spf13/cobra"
)

var (
	grovePath    string
	globalMode   bool
	profile      string
	outputFormat string
	hubEndpoint  string // Hub API endpoint override
	noHub        bool   // Disable Hub integration for this invocation
	autoHelp     = true // Default to true, updated in PersistentPreRunE
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "scion",
	Short: "A container-based orchestration tool for managing concurrent LLM agents",
	Long: `Scion is a container-based orchestration tool for managing 
concurrent LLM agents. It enables parallel execution of specialized 
sub-agents with isolated identities, credentials, and workspaces.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if globalMode && grovePath == "" {
			grovePath = "global"
		}

		if util.IsGitRepo() {
			if err := util.CheckGitVersion(); err != nil {
				return fmt.Errorf("git check failed: %w", err)
			}
		}

		// Determine if this command requires explicit grove context
		// Commands that don't require grove context:
		// - help, version, completion (built-in or explicit)
		// - init, grove init (creates grove)
		// - server (runs hub server, doesn't need local grove)
		cmdName := cmd.Name()
		parentName := ""
		if cmd.Parent() != nil {
			parentName = cmd.Parent().Name()
		}

		requiresGrove := true
		switch cmdName {
		case "help", "version", "completion", "server":
			requiresGrove = false
		case "init":
			// Both top-level init and grove init don't require existing grove
			requiresGrove = false
		case "scion":
			// Root command itself doesn't require grove
			requiresGrove = false
		}
		// Also check if parent is grove and command is init
		if parentName == "grove" && cmdName == "init" {
			requiresGrove = false
		}

		// For commands that require grove context, use RequireGrovePath
		// to error if no project found and --global not specified
		if requiresGrove && grovePath == "" {
			if _, _, err := config.RequireGrovePath(grovePath); err != nil {
				return err
			}
		}

		// Load settings to get cli.autohelp
		settings, err := config.LoadSettings(grovePath)
		if err == nil && settings.CLI != nil && settings.CLI.AutoHelp != nil {
			autoHelp = *settings.CLI.AutoHelp
		}

		if outputFormat != "" {
			if outputFormat != "json" && outputFormat != "plain" {
				return fmt.Errorf("invalid format: %s (allowed: json, plain)", outputFormat)
			}
			if cmd != listCmd {
				// TODO: support format for other commands
				return fmt.Errorf("format flag is not yet supported for command %s", cmd.Name())
			}
		}

		// Check for dev auth usage and warn if Hub is enabled
		printDevAuthWarningIfNeeded(grovePath)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Early settings load to determine autoHelp behavior
	// This handles cases where ExecuteC fails during flag parsing or unknown commands
	tempGrovePath := ""
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--grove" || arg == "-g" {
			if i+1 < len(os.Args) {
				tempGrovePath = os.Args[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "--grove=") {
			tempGrovePath = strings.TrimPrefix(arg, "--grove=")
		} else if arg == "--global" {
			tempGrovePath = "global"
		}
	}
	settings, _ := config.LoadSettings(tempGrovePath)
	if settings != nil && settings.CLI != nil && settings.CLI.AutoHelp != nil {
		autoHelp = *settings.CLI.AutoHelp
	}

	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s%sError: %v%s\n\n", util.BgRed, util.Black, err, util.Reset)
		if cmd != nil && autoHelp {
			cmd.Usage()
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.Long = util.GetBanner() + "\n" + rootCmd.Long
	rootCmd.PersistentFlags().StringVarP(&grovePath, "grove", "g", "", "Path to a .scion grove directory")
	rootCmd.PersistentFlags().BoolVar(&globalMode, "global", false, "Use the global grove (equivalent to --grove global)")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "Configuration profile to use")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "", "Output format (e.g., json)")

	// Hub integration flags
	rootCmd.PersistentFlags().StringVar(&hubEndpoint, "hub", "", "Hub API endpoint URL (overrides SCION_HUB_ENDPOINT)")
	rootCmd.PersistentFlags().BoolVar(&noHub, "no-hub", false, "Disable Hub integration for this invocation (local-only mode)")
}

// GetHubEndpoint returns the effective Hub endpoint based on flags and settings.
// Returns empty string if Hub is disabled or not configured.
func GetHubEndpoint(settings interface{ GetHubEndpoint() string }) string {
	if noHub {
		return ""
	}
	if hubEndpoint != "" {
		return hubEndpoint
	}
	if settings != nil {
		return settings.GetHubEndpoint()
	}
	return ""
}

// IsHubEnabled returns true if Hub integration is enabled for this invocation.
func IsHubEnabled() bool {
	return !noHub
}

// printDevAuthWarningIfNeeded checks if dev auth is being used with Hub and prints a warning.
// This function is called on every command invocation via PersistentPreRunE.
func printDevAuthWarningIfNeeded(grovePath string) {
	// Skip if --no-hub flag is set
	if noHub {
		return
	}

	// Try to load settings to check if Hub is enabled
	settings, err := config.LoadSettings(grovePath)
	if err != nil {
		// If we can't load settings, skip the warning
		return
	}

	// Check if Hub is enabled (either via settings or --hub flag override)
	hubEnabled := settings.IsHubEnabled() || hubEndpoint != ""
	if !hubEnabled {
		return
	}

	// Check if explicit auth is configured in settings
	if settings.Hub != nil {
		if settings.Hub.Token != "" || settings.Hub.APIKey != "" || settings.Hub.HostToken != "" {
			// Explicit auth configured, not using dev auth
			return
		}
	}

	// Check if a dev token would be used
	devToken := apiclient.ResolveDevToken()
	if devToken == "" {
		// No dev token available
		return
	}

	// Dev auth is being used with Hub enabled - print warning to stderr
	fmt.Fprintf(os.Stderr, "\n%s%s WARNING: Development authentication enabled - not for production use %s\n\n",
		util.Bold, util.Yellow, util.Reset)
}
