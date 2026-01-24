/*
Copyright 2025 The Scion Authors.
*/

package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	reaper "github.com/ramr/go-reaper"
	"github.com/spf13/cobra"

	"github.com/ptone/scion-agent/pkg/sciontool/hooks"
	"github.com/ptone/scion-agent/pkg/sciontool/hooks/handlers"
	"github.com/ptone/scion-agent/pkg/sciontool/supervisor"
)

var (
	gracePeriod time.Duration
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [--] <command> [args...]",
	Short: "Run as container init (PID 1) and supervise child processes",
	Long: `The init command runs sciontool as the container's init process (PID 1).

It provides:
  - Zombie process reaping (critical for PID 1)
  - Signal forwarding to child processes
  - Graceful shutdown with configurable grace period
  - Child process exit code propagation

The command after -- is executed as the child process. If no command is
specified, sciontool will exit with an error.

Examples:
  sciontool init -- gemini
  sciontool init -- tmux new-session -A -s main
  sciontool init --grace-period=30s -- claude`,
	DisableFlagParsing: false,
	Run: func(cmd *cobra.Command, args []string) {
		exitCode := runInit(args)
		os.Exit(exitCode)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().DurationVar(&gracePeriod, "grace-period", 10*time.Second,
		"Time to wait after SIGTERM before sending SIGKILL")

	// Override the default SCION_GRACE_PERIOD env var if set
	if envGrace := os.Getenv("SCION_GRACE_PERIOD"); envGrace != "" {
		if d, err := time.ParseDuration(envGrace); err == nil {
			gracePeriod = d
		}
	}
}

func runInit(args []string) int {
	// Start the reaper goroutine for zombie process cleanup.
	// This is critical when running as PID 1 in a container.
	// The reaper configuration uses default settings which are sufficient
	// for our use case: it reaps children and doesn't call os.Exit.
	go reaper.Reap()

	// Extract the child command (everything after --)
	childArgs := extractChildCommand(args)
	if len(childArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no command specified after --")
		fmt.Fprintln(os.Stderr, "Usage: sciontool init [--] <command> [args...]")
		return 1
	}

	// Log startup
	logInfo("sciontool init starting as PID %d", os.Getpid())
	logInfo("Child command: %v", childArgs)
	logInfo("Grace period: %s", gracePeriod)

	// Initialize lifecycle hooks manager
	lifecycleManager := hooks.NewLifecycleManager()

	// Register status and logging handlers for lifecycle events
	// These handlers update agent-info.json and agent.log on container lifecycle events
	statusHandler := handlers.NewStatusHandler()
	loggingHandler := handlers.NewLoggingHandler()

	for _, eventName := range []string{hooks.EventPreStart, hooks.EventPostStart, hooks.EventPreStop, hooks.EventSessionEnd} {
		lifecycleManager.RegisterHandler(eventName, statusHandler.Handle)
		lifecycleManager.RegisterHandler(eventName, loggingHandler.Handle)
	}

	// Run pre-start hooks (after setup, before child process)
	logInfo("Running pre-start hooks...")
	if err := lifecycleManager.RunPreStart(); err != nil {
		logError("Pre-start hooks failed: %v", err)
		// Continue anyway - hooks failing shouldn't prevent startup
	}

	// Create supervisor with configuration
	config := supervisor.Config{
		GracePeriod: gracePeriod,
	}
	sup := supervisor.New(config)

	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling with pre-stop hook for graceful shutdown
	sigHandler := supervisor.NewSignalHandler(sup, cancel).
		WithPreStopHook(func() error {
			logInfo("Running pre-stop hooks...")
			return lifecycleManager.RunPreStop()
		})
	sigHandler.Start()
	defer sigHandler.Stop()

	// Run the child process under supervision
	// We use a goroutine to allow post-start hooks to run after process starts
	exitChan := make(chan struct {
		code int
		err  error
	}, 1)

	go func() {
		code, err := sup.Run(ctx, childArgs)
		exitChan <- struct {
			code int
			err  error
		}{code, err}
	}()

	// Wait a moment for process to start, then run post-start hooks
	// Use a short timeout to detect immediate startup failures
	select {
	case result := <-exitChan:
		// Child exited immediately - likely a startup error
		if result.err != nil {
			logError("Supervisor error: %v", result.err)
			return 1
		}
		logInfo("Child exited with code %d", result.code)
		return result.code
	case <-time.After(100 * time.Millisecond):
		// Process appears to be running, execute post-start hooks
		logInfo("Running post-start hooks...")
		if err := lifecycleManager.RunPostStart(); err != nil {
			logError("Post-start hooks failed: %v", err)
			// Continue anyway
		}
	}

	// Wait for child to exit
	result := <-exitChan

	// Run session-end hooks (graceful shutdown)
	logInfo("Running session-end hooks...")
	if err := lifecycleManager.RunSessionEnd(); err != nil {
		logError("Session-end hooks failed: %v", err)
	}

	if result.err != nil {
		logError("Supervisor error: %v", result.err)
		return 1
	}

	logInfo("Child exited with code %d", result.code)
	return result.code
}

// extractChildCommand extracts the command arguments.
// Cobra handles -- separator, so args contains everything after --.
func extractChildCommand(args []string) []string {
	return args
}

// logInfo logs an informational message to stderr (stdout is reserved for child).
func logInfo(format string, args ...interface{}) {
	level := os.Getenv("SCION_LOG_LEVEL")
	if level == "error" || level == "warn" {
		return
	}
	fmt.Fprintf(os.Stderr, "[sciontool] "+format+"\n", args...)
}

// logError logs an error message to stderr.
func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[sciontool] ERROR: "+format+"\n", args...)
}
