// Package main provides the CLI entrypoint for histui.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/store"
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
		verbose     bool
		historyFile string
		configPath  string
	}
	logger *slog.Logger

	// historyStore is the global store instance
	historyStore  *store.Store
	tombstoneFile *store.TombstoneFile
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

		// Use custom history file path if specified, otherwise use default
		historyPath := globalOpts.historyFile
		if historyPath == "" {
			historyPath = config.HistoryPath()
		}

		persistence, err := store.NewJSONLPersistence(historyPath)
		if err != nil {
			return fmt.Errorf("failed to initialize persistence: %w", err)
		}

		historyStore = store.NewStore(persistence)

		// Load tombstones
		tombstoneFile = store.NewTombstoneFile(config.TombstonePath())
		tombstones, err := tombstoneFile.Load()
		if err != nil {
			logger.Warn("failed to load tombstones", "error", err)
		} else if len(tombstones) > 0 {
			historyStore.LoadTombstones(tombstones)
		}

		if err := historyStore.Hydrate(); err != nil {
			logger.Warn("failed to hydrate store from disk", "error", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Save tombstones
		if tombstoneFile != nil && historyStore != nil {
			tombstones := historyStore.GetTombstones()
			if len(tombstones) > 0 {
				if err := tombstoneFile.Save(tombstones); err != nil {
					logger.Warn("failed to save tombstones", "error", err)
				}
			}
		}

		// Cleanup store
		if historyStore != nil {
			return historyStore.Close()
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
	rootCmd.PersistentFlags().StringVar(&globalOpts.historyFile, "history-file", "",
		"Path to history file (default: ~/.local/share/histui/history.jsonl)")
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

// getStore returns the global store instance.
func getStore() *store.Store {
	return historyStore
}

// getConfig returns the global config instance.
func getConfig() *config.Config {
	return cfg
}
