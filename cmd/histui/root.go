// Package main provides the CLI entrypoint for histui.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/db"
)

// Build-time variables (set via ldflags)
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

// Global configuration and state
var (
	cfg        *config.Config
	globalOpts struct {
		verbose    bool
		dbPath     string
		configPath string
	}
	logger *slog.Logger

	// database is the global SQLite database instance
	database *db.DB
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "histui",
	Short: "Notification history browser for Linux desktops",
	Long: `histui is a notification history browser for Linux desktops.

It provides a unified interface for viewing, searching, and acting on
notification history from multiple notification daemons.

Running histui without a subcommand launches the interactive TUI.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildTime),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Setup logging
		setupLogger()

		// `completion` generates a static shell script; it must not touch
		// config or the database so it works in clean build chroots without
		// a writable $HOME.
		if isCompletionCmd(cmd) {
			return nil
		}

		// Load configuration
		var err error
		cfg, err = config.LoadConfig(globalOpts.configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize persistence (always enabled)
		if err := config.EnsureDataDir(); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Open SQLite database (use custom path if specified, otherwise default)
		database, err = db.Open(globalOpts.dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Close database
		if database != nil {
			return database.Close()
		}
		return nil
	},
	// Default to TUI when no subcommand is provided
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.verbose, "verbose", "v", false,
		"Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&globalOpts.dbPath, "db", "",
		"Path to database file (default: ~/.local/share/histui/histui.db)")
	rootCmd.PersistentFlags().StringVar(&globalOpts.configPath, "config", "",
		"Path to config file (default: ~/.config/histui/config.toml)")
}

// setupLogger configures the global slog logger.
func setupLogger() {
	level := slog.LevelWarn
	if globalOpts.verbose {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Log to stderr so stdout is clean for output
	handler := slog.NewTextHandler(os.Stderr, opts)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// getDB returns the global database instance.
func getDB() *db.DB {
	return database
}

// getConfig returns the global config instance.
func getConfig() *config.Config {
	return cfg
}

// isCompletionCmd reports whether cmd (or an ancestor) is the cobra
// `completion` command tree used to generate shell completion scripts.
func isCompletionCmd(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" {
			return true
		}
	}
	return false
}
