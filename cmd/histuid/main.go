// Package main is the entry point for the histuid notification daemon.
package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	godbus "github.com/godbus/dbus/v5"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/jmylchreest/histui/internal/audio"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/daemon"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/display"
	"github.com/jmylchreest/histui/internal/icon"
	"github.com/jmylchreest/histui/internal/model"
	"github.com/jmylchreest/histui/internal/store"
	"github.com/jmylchreest/histui/internal/theme"
)

// dismissedIDsCache tracks which histui IDs we know are dismissed
// to avoid re-processing on every store reload.
var dismissedIDsCache = make(map[string]bool)

const (
	appID   = "io.github.jmylchreest.histuid"
	appName = "histuid"
)

var (
	// Build-time variables
	version = "dev"
)

func main() {
	// Suppress verbose graphics debug output from Mesa/Vulkan
	// These write directly to stderr and aren't controlled by our logging
	suppressGraphicsDebug()

	// Define command-line flags using pflag
	monitorMode := pflag.Bool("monitor", false, "Run in monitor mode (passive, no popups/sounds, works alongside another notification daemon)")
	showVersion := pflag.Bool("version", false, "Show version and exit")
	logLevel := pflag.String("log-level", "", "Log level: debug, info, warn, error (default: info)")

	// Config override flags (for testing different display positions)
	// These are bound to Viper config keys
	pflag.String("position", "", "Override display position (top-right, top-left, top-center, bottom-right, bottom-left, bottom-center)")
	pflag.Int("offset-x", 0, "Override horizontal offset from screen edge")
	pflag.Int("offset-y", 0, "Override vertical offset from screen edge")
	pflag.Int("max-visible", 0, "Override maximum visible notifications")
	pflag.Int("display-monitor", 0, "Override monitor number (0=all, 1+=specific)")
	pflag.Bool("new-on-top", false, "Override new notifications appearing at top of stack")
	pflag.String("theme", "", "Override theme name")
	pflag.String("font", "", "Override font family (e.g., 'Sans', 'Monospace', 'Ubuntu')")
	pflag.Int("font-size", 0, "Override base font size in pixels (e.g., 14, 16, 18)")
	pflag.Bool("no-audio", false, "Disable notification sounds (overrides theme and config)")
	pflag.Float64("volume", -1, "Set global audio volume (0.0-1.0, -1 = use config)")

	pflag.Parse()

	if *showVersion {
		println("histuid version", version)
		os.Exit(0)
	}

	// Determine log level: flag > config > default (info)
	level := slog.LevelInfo
	if *logLevel != "" {
		level = config.ParseLogLevel(*logLevel)
	} else {
		// Try to read log level from config file
		v := viper.New()
		v.SetConfigName("histuid")
		v.SetConfigType("toml")
		if configDir, err := os.UserConfigDir(); err == nil {
			v.AddConfigPath(filepath.Join(configDir, "histui"))
		}
		if err := v.ReadInConfig(); err == nil {
			if cfgLevel := v.GetString("log_level"); cfgLevel != "" {
				level = config.ParseLogLevel(cfgLevel)
			}
		}
	}

	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	if *monitorMode {
		runMonitorMode(logger)
		return
	}

	runDaemonMode(logger)
}

// runMonitorMode runs histuid in passive monitor mode.
// It observes D-Bus notification traffic without claiming the notification service name.
// No popups are displayed and no sounds are played.
func runMonitorMode(logger *slog.Logger) {
	logger.Info("starting histuid in monitor mode", "version", version)

	// Clean up any leftover temp files from previous runs
	daemon.CleanupTemp(logger)

	// Initialize history store with persistence
	historyPath, err := store.HistoryPath()
	if err != nil {
		logger.Error("failed to get history path", "error", err)
		os.Exit(1)
	}

	persistence, err := store.NewJSONLPersistence(historyPath)
	if err != nil {
		logger.Error("failed to create persistence", "error", err)
		os.Exit(1)
	}

	historyStore := store.NewStore(persistence)
	if err := historyStore.Hydrate(); err != nil {
		logger.Warn("failed to hydrate store", "error", err)
	}
	logger.Info("history store initialized", "path", historyPath, "count", historyStore.Count())

	// Create and configure the monitor
	monitor := dbus.NewMonitor(logger)
	monitor.SetNotifyHandler(func(notification *dbus.DBusNotification, id uint32) {
		// Create a model.Notification for persistence
		n, err := model.NewNotification("histuid-monitor")
		if err != nil {
			logger.Error("failed to create notification model", "error", err)
			return
		}

		// Populate from D-Bus notification
		n.ID = int(id)
		n.AppName = notification.AppName
		n.Summary = notification.Summary
		n.Body = notification.Body
		n.Timestamp = time.Now().Unix()
		n.ExpireTimeout = int(notification.ExpireTimeout)
		n.IconPath = notification.AppIcon
		n.SetUrgency(notification.Urgency())
		n.Category = notification.Category()

		// Store D-Bus specific extensions
		n.Extensions = &model.Extensions{
			Actions:      convertActions(notification.ParsedActions()),
			SoundFile:    notification.SoundFile(),
			SoundName:    notification.SoundName(),
			DesktopEntry: notification.DesktopEntry(),
			Resident:     notification.Resident(),
			Transient:    notification.Transient(),
		}

		// Don't persist transient notifications
		if !notification.Transient() {
			if err := historyStore.Add(*n); err != nil {
				logger.Error("failed to persist notification", "id", id, "error", err)
			} else {
				logger.Debug("persisted notification", n.LogAttrs()...)
			}
		} else {
			logger.Debug("skipped transient notification", n.LogAttrs()...)
		}
	})

	// Start the monitor
	if err := monitor.Start(); err != nil {
		logger.Error("failed to start D-Bus monitor", "error", err)
		os.Exit(1)
	}

	logger.Info("histuid monitor ready - passively capturing notifications")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig)

	// Clean up
	if err := monitor.Stop(); err != nil {
		logger.Warn("error stopping monitor", "error", err)
	}
	if err := historyStore.Close(); err != nil {
		logger.Warn("error closing store", "error", err)
	}
	daemon.CleanupTemp(logger)

	logger.Info("histuid monitor stopped")
}

// runDaemonMode runs histuid as the primary notification daemon with full functionality.
func runDaemonMode(logger *slog.Logger) {
	logger.Info("starting histuid", "version", version)

	// Clean up any leftover temp files from previous runs (in case of unclean shutdown)
	daemon.CleanupTemp(logger)

	// Initialize embedded fonts (Nerd Font symbols, etc.) for icon fallback
	// Must be done before GTK initialization so fontconfig picks it up
	if fontDir, err := icon.InitFonts(); err != nil {
		logger.Warn("failed to initialize embedded fonts", "error", err)
	} else {
		logger.Debug("initialized embedded fonts", "dir", fontDir, "fonts", icon.ListEmbeddedFonts())
	}

	// Initialize Viper for configuration
	v, err := config.NewViper()
	if err != nil {
		logger.Error("failed to initialize config", "error", err)
		os.Exit(1)
	}

	// Bind pflags to Viper (only flags that were explicitly set will override)
	if err := config.BindPFlags(v, pflag.CommandLine); err != nil {
		logger.Error("failed to bind flags", "error", err)
		os.Exit(1)
	}

	// Load configuration (precedence: flags > env > config file > defaults)
	cfg, err := config.LoadDaemonConfigWithViper(v)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Apply audio flags (overrides config)
	if pflag.CommandLine.Changed("no-audio") {
		noAudio, _ := pflag.CommandLine.GetBool("no-audio")
		if noAudio {
			cfg.Audio.Enabled = false
			logger.Info("audio disabled via --no-audio flag")
		}
	}
	if pflag.CommandLine.Changed("volume") {
		audioVolume, _ := pflag.CommandLine.GetFloat64("volume")
		if audioVolume >= 0 && audioVolume <= 1.0 {
			cfg.Audio.Volume = int(audioVolume * 100)
			logger.Info("audio volume set via --volume flag", "volume", audioVolume)
		}
	}

	// Log effective configuration (helpful for debugging overrides)
	if v.IsSet("display.position") && v.GetString("display.position") != "" {
		logger.Info("using display position", "position", cfg.Display.Position, "source", getConfigSource(v, "display.position"))
	}

	// Create the libadwaita application
	app := adw.NewApplication(appID, 0)

	// Set AdwStyleManager color scheme early (in startup phase) to avoid GtkSettings warning
	app.ConnectStartup(func() {
		styleManager := adw.StyleManagerGetDefault()
		switch config.ColorScheme(cfg.Theme.ColorScheme) {
		case config.ColorSchemeLight:
			styleManager.SetColorScheme(adw.ColorSchemeForceLight)
		case config.ColorSchemeDark:
			styleManager.SetColorScheme(adw.ColorSchemeForceDark)
		default:
			styleManager.SetColorScheme(adw.ColorSchemeDefault)
		}
	})

	// Shared state between GTK main loop and signal handlers
	var (
		dbusServer       *dbus.NotificationServer
		displayManager   *display.Manager
		themeLoader      *theme.Loader
		audioManager     *audio.Manager
		historyStore     *store.Store
		imageStore       *store.ImageStore
		retentionMgr     *store.RetentionManager
		displayState     *daemon.DisplayStateManager
		storeWatcher     *daemon.StoreWatcher
		stateWatcher     *daemon.StateWatcher
		configWatcher    *daemon.ConfigWatcher
		internalNotifier *daemon.InternalNotifier
		sharedState      *store.SharedState
		running          atomic.Bool
	)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()

		// Stop components in GTK main loop context
		glib.IdleAdd(func() {
			if running.Load() {
				if audioManager != nil {
					audioManager.Stop()
				}
				if themeLoader != nil {
					themeLoader.StopHotReload()
				}
				if configWatcher != nil {
					configWatcher.Stop()
				}
				if stateWatcher != nil {
					stateWatcher.Stop()
				}
				if storeWatcher != nil {
					storeWatcher.Stop()
				}
				if displayManager != nil {
					displayManager.Stop()
				}
				if dbusServer != nil {
					_ = dbusServer.Stop()
				}
				if retentionMgr != nil {
					_ = retentionMgr.Close()
				}
				if historyStore != nil {
					_ = historyStore.Close()
				}
				if imageStore != nil {
					_ = imageStore.Close()
				}
				app.Quit()
			}
		})
	}()

	// Handle application activation
	app.ConnectActivate(func() {
		if running.Load() {
			logger.Warn("application already running")
			return
		}
		running.Store(true)

		// Check D-Bus availability early before initializing other components
		if err := dbus.CheckBusNameAvailable(); err != nil {
			logger.Error("cannot start notification daemon", "error", err)
			app.Quit()
			return
		}

		// Initialize history store with persistence
		historyPath, err := store.HistoryPath()
		if err != nil {
			logger.Error("failed to get history path", "error", err)
			app.Quit()
			return
		}

		persistence, err := store.NewJSONLPersistence(historyPath)
		if err != nil {
			logger.Error("failed to create persistence", "error", err)
			app.Quit()
			return
		}

		historyStore = store.NewStore(persistence)
		if err := historyStore.Hydrate(); err != nil {
			logger.Warn("failed to hydrate store", "error", err)
		}
		logger.Info("history store initialized", "path", historyPath, "count", historyStore.Count())

		// Initialize image store if enabled
		if cfg.History.StoreImages {
			imagePath, err := store.DefaultImageStorePath()
			if err != nil {
				logger.Warn("failed to get image store path", "error", err)
			} else {
				imageStore, err = store.NewImageStore(imagePath)
				if err != nil {
					logger.Warn("failed to create image store", "error", err)
				} else {
					logger.Debug("image store initialized", "path", imagePath)
				}
			}
		}

		// Initialize retention manager for auto-pruning
		if cfg.History.MaxNotifications > 0 {
			retentionMgr = store.NewRetentionManager(persistence, imageStore, store.RetentionConfig{
				MaxNotifications: cfg.History.MaxNotifications,
			}, logger)
			// Run initial prune on startup
			if err := retentionMgr.PruneOnStartup(); err != nil {
				logger.Warn("failed to prune on startup", "error", err)
			}
		}

		// Load shared state (DnD, etc.)
		sharedState, err = store.LoadSharedState()
		if err != nil {
			logger.Warn("failed to load shared state", "error", err)
			sharedState = store.DefaultSharedState()
		}
		logger.Info("shared state loaded", "dnd_enabled", sharedState.DnDEnabled)

		// Initialize display state manager (maps D-Bus IDs to histui IDs)
		displayState = daemon.NewDisplayStateManager()

		// Initialize theme loader
		themeLoader = theme.NewLoader(logger)
		if err := themeLoader.LoadTheme(cfg.Theme.Name); err != nil {
			logger.Warn("failed to load theme, using default", "error", err)
		}
		themeLoader.Apply(nil)
		themeLoader.ApplyFontOverrides(cfg.Theme.FontFamily, cfg.Theme.FontSize)
		themeLoader.StartHotReload(ctx)

		// Initialize audio manager
		audioManager = audio.NewManager(cfg, logger)
		if err := audioManager.Start(ctx); err != nil {
			logger.Warn("failed to start audio manager", "error", err)
		}

		// Load theme sounds into audio manager (theme manifest takes priority over daemon config)
		loadThemeSounds(themeLoader, audioManager, logger)

		// Initialize display manager
		displayManager = display.NewManager(&app.Application, cfg, logger)
		if err := displayManager.Start(ctx); err != nil {
			logger.Error("failed to start display manager", "error", err)
			app.Quit()
			return
		}

		// Initialize D-Bus server
		dbusServer = dbus.NewNotificationServer(logger)
		dbusServer.SetServerInfo(dbus.ServerInfo{
			Name:        appName,
			Vendor:      "histui",
			Version:     version,
			SpecVersion: "1.2",
		})

		// Connect D-Bus notifications to display manager AND store
		dbusServer.SetNotifyHandler(func(notification *dbus.DBusNotification, id uint32) {
			// Check if this is a replayed notification from histui
			if dbus.IsReplayHint(notification.Hints) {
				originalID := dbus.GetOriginalID(notification.Hints)
				if originalID != "" {
					// Update existing notification's replayed status
					existing := historyStore.GetByID(originalID)
					if existing != nil {
						existing.MarkReplayed()
						if err := historyStore.Update(*existing); err != nil {
							logger.Warn("failed to update replayed notification", "id", originalID, "error", err)
						} else {
							logger.Debug("marked notification as replayed", "id", originalID)
						}
					}
					// Continue to display the notification (don't return)
					// but use the existing histui ID for tracking
				}
			}

			// Create a model.Notification for persistence
			n, err := model.NewNotification("histuid")
			if err != nil {
				logger.Error("failed to create notification model", "error", err)
				return
			}

			// Populate from D-Bus notification
			n.ID = int(id)
			n.AppName = notification.AppName
			n.Summary = notification.Summary
			n.Body = notification.Body
			n.Timestamp = time.Now().Unix()
			n.ExpireTimeout = int(notification.ExpireTimeout)
			n.IconPath = notification.AppIcon
			n.SetUrgency(notification.Urgency())
			n.Category = notification.Category()

			// Store D-Bus specific extensions
			n.Extensions = &model.Extensions{
				Actions:      convertActions(notification.ParsedActions()),
				SoundFile:    notification.SoundFile(),
				SoundName:    notification.SoundName(),
				DesktopEntry: notification.DesktopEntry(),
				Resident:     notification.Resident(),
				Transient:    notification.Transient(),
			}

			// Store original hints for faithful replay
			n.OriginalHints = convertHintsToJSON(notification.Hints)

			// Capture and store image data if image store is enabled
			if imageStore != nil && !notification.Transient() {
				// Check for image-data hint
				if imgData := notification.ImageData(); imgData != nil {
					// Convert to PNG and store
					pngData := convertImageDataToPNG(imgData)
					if len(pngData) > 0 {
						ref, err := imageStore.Save(n.HistuiID, store.ImageRefImage, pngData)
						if err != nil {
							logger.Warn("failed to save notification image", "error", err)
						} else if ref != "" {
							n.AddImageRef(ref)
						}
					}
				}
			}

			// Don't persist transient notifications or replays
			isReplay := dbus.IsReplayHint(notification.Hints)
			if !notification.Transient() && !isReplay {
				if err := historyStore.Add(*n); err != nil {
					logger.Error("failed to persist notification", "id", id, "error", err)
				}

				// Trigger retention manager (debounced)
				if retentionMgr != nil {
					retentionMgr.TriggerPrune()
				}
			}

			// Track the mapping between D-Bus ID and histui ID
			timeout := cfg.GetTimeoutForUrgency(notification.Urgency(), notification.ExpireTimeout)
			var expiresAt time.Time
			if timeout > 0 {
				expiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
			}
			displayState.Register(n.HistuiID, id, expiresAt)

			// Check if DnD is enabled (suppress popups and sounds)
			urgency := notification.Urgency()
			isDnDEnabled := sharedState != nil && sharedState.DnDEnabled
			isCriticalBypass := cfg.DnD.CriticalBypass && urgency == model.UrgencyCritical

			// Suppress popup and sound if DnD is enabled (unless critical bypass)
			if isDnDEnabled && !isCriticalBypass {
				logger.Debug("notification suppressed by DnD", "id", id, "urgency", urgency)
				// Note: Notification is still persisted to store (done above)
				return
			}

			// Play notification sound based on urgency
			// Use sound-file hint if provided, otherwise use per-urgency configured sound
			go func() {
				soundFile := notification.SoundFile()
				if soundFile != "" {
					if err := audioManager.PlayFile(soundFile); err != nil {
						logger.Debug("failed to play notification sound file", "file", soundFile, "error", err)
					}
				} else {
					if err := audioManager.PlayForUrgency(urgency); err != nil {
						logger.Debug("failed to play urgency sound", "urgency", urgency, "error", err)
					}
				}
			}()

			// Schedule display on GTK main loop
			glib.IdleAdd(func() {
				if err := displayManager.Show(notification, id, n.HistuiID); err != nil {
					logger.Error("failed to show notification", "id", id, "error", err)
				}
			})
		})

		dbusServer.SetCloseHandler(func(id uint32) {
			glib.IdleAdd(func() {
				displayManager.Close(id, dbus.CloseReasonClosed)
			})
		})

		// Connect display manager callbacks to D-Bus and store
		displayManager.SetCloseCallback(func(dbusID uint32, reason dbus.CloseReason) {
			// Emit D-Bus signal
			if err := dbusServer.CloseWithReason(dbusID, reason); err != nil {
				logger.Warn("failed to emit close signal", "id", dbusID, "error", err)
			}

			// Update store if user dismissed (not expired)
			if reason == dbus.CloseReasonDismissed {
				histuiID := displayState.GetHistuiIDByDBusID(dbusID)
				if histuiID != "" {
					if err := historyStore.Dismiss(histuiID); err != nil {
						logger.Warn("failed to mark notification as dismissed", "histui_id", histuiID, "error", err)
					}
				}
			}

			// Update display state
			displayState.RemoveByDBusID(dbusID)
		})

		displayManager.SetActionCallback(func(dbusID uint32, actionKey string) {
			if err := dbusServer.EmitActionInvoked(dbusID, actionKey); err != nil {
				logger.Warn("failed to emit action signal", "id", dbusID, "error", err)
			}
		})

		// Start D-Bus server
		if err := dbusServer.Start(); err != nil {
			logger.Error("failed to start D-Bus server", "error", err)
			displayManager.Stop()
			app.Quit()
			return
		}

		// Initialize store watcher for external changes (e.g., histui CLI prune/dismiss)
		storeWatcher = daemon.NewStoreWatcher(historyPath, logger)
		storeWatcher.SetChangeCallback(func() {
			// Store file changed - check if we need to reload (external prune/delete)
			glib.IdleAdd(func() {
				// Only reload if disk has fewer entries than memory (external prune)
				reloaded, err := historyStore.ReloadIfNeeded()
				if err != nil {
					logger.Warn("failed to check/reload store", "error", err)
				}
				if reloaded {
					logger.Debug("store reloaded due to external changes")
				}
				checkForExternalDismissals(historyStore, displayManager, displayState, logger)
			})
		})
		if err := storeWatcher.Start(ctx); err != nil {
			logger.Warn("failed to start store watcher", "error", err)
		}

		// Initialize state watcher for DnD changes (e.g., histui dnd toggle)
		statePath, err := store.StateFilePath()
		if err != nil {
			logger.Warn("failed to get state file path", "error", err)
		} else {
			stateWatcher = daemon.NewStateWatcher(statePath, logger)
			stateWatcher.SetChangeCallback(func() {
				// State file changed - reload shared state
				newState, err := store.LoadSharedState()
				if err != nil {
					logger.Warn("failed to reload shared state", "error", err)
					return
				}
				if newState.DnDEnabled != sharedState.DnDEnabled {
					logger.Info("DnD state changed", "enabled", newState.DnDEnabled)
				}
				// Check for audio stop request
				if newState.StopAudioRequestedAt != sharedState.StopAudioRequestedAt &&
					newState.StopAudioRequestedAt > 0 {
					logger.Debug("audio stop requested via state file")
					if audioManager != nil {
						audioManager.StopPlayback()
					}
				}
				sharedState = newState
			})
			if err := stateWatcher.Start(ctx); err != nil {
				logger.Warn("failed to start state watcher", "error", err)
			}
		}

		// Initialize internal notifier for self-notifications
		internalNotifier = daemon.NewInternalNotifier(logger)
		internalNotifier.SetNotifyHandler(func(notification *dbus.DBusNotification) uint32 {
			// Use the D-Bus server to create the notification internally
			return dbusServer.NotifyInternal(notification)
		})

		// Initialize config watcher for hot-reload
		configWatcher, err = daemon.NewConfigWatcher(logger)
		if err != nil {
			logger.Warn("failed to create config watcher", "error", err)
		} else {
			configWatcher.SetReloadCallback(func(newConfig *config.DaemonConfig) {
				// Update components with new config
				glib.IdleAdd(func() {
					// Update display manager config
					displayManager.UpdateConfig(newConfig)

					// Update audio manager config
					audioManager.UpdateConfig(newConfig)

					// Reload theme if changed
					if newConfig.Theme.Name != cfg.Theme.Name {
						if err := themeLoader.LoadTheme(newConfig.Theme.Name); err != nil {
							logger.Warn("failed to load new theme", "theme", newConfig.Theme.Name, "error", err)
							internalNotifier.NotifyThemeError(err)
						} else {
							themeLoader.Apply(nil)
							// Reload sounds from new theme
							loadThemeSounds(themeLoader, audioManager, logger)
							internalNotifier.NotifyThemeReloaded(newConfig.Theme.Name)
						}
					}

					// Update font overrides if changed
					if newConfig.Theme.FontFamily != cfg.Theme.FontFamily ||
						newConfig.Theme.FontSize != cfg.Theme.FontSize {
						themeLoader.ApplyFontOverrides(newConfig.Theme.FontFamily, newConfig.Theme.FontSize)
					}

					// Update color scheme if changed
					if newConfig.Theme.ColorScheme != cfg.Theme.ColorScheme {
						display.ApplyColorScheme(newConfig.Theme.ColorScheme, logger)
					}

					// Update the config reference
					cfg = newConfig

					// Notify user
					internalNotifier.NotifyConfigReloaded()
				})
			})
			configWatcher.SetErrorCallback(func(err error) {
				// Config validation failed - notify user
				internalNotifier.NotifyConfigError(err)
			})
			if err := configWatcher.Start(ctx, cfg); err != nil {
				logger.Warn("failed to start config watcher", "error", err)
			}
		}

		logger.Info("histuid ready", "dbus_interface", dbus.DBusInterface)

		// Create a hidden window to keep the application running
		// (GTK apps quit when all windows are closed)
		keepAliveWindow := gtk.NewWindow()
		keepAliveWindow.SetApplication(&app.Application)
		keepAliveWindow.SetDefaultSize(1, 1)
		keepAliveWindow.SetDecorated(false)
		keepAliveWindow.SetVisible(false)
	})

	// Handle shutdown
	app.ConnectShutdown(func() {
		logger.Info("application shutting down")
		if audioManager != nil {
			audioManager.Stop()
		}
		if themeLoader != nil {
			themeLoader.StopHotReload()
		}
		if configWatcher != nil {
			configWatcher.Stop()
		}
		if stateWatcher != nil {
			stateWatcher.Stop()
		}
		if storeWatcher != nil {
			storeWatcher.Stop()
		}
		if displayManager != nil {
			displayManager.Stop()
		}
		if dbusServer != nil {
			_ = dbusServer.Stop()
		}
		// Run any pending prune before closing
		if retentionMgr != nil {
			_ = retentionMgr.Close()
		}
		if historyStore != nil {
			_ = historyStore.Close()
		}
		if imageStore != nil {
			_ = imageStore.Close()
		}
		// Clean up temporary files
		daemon.CleanupTemp(logger)
		running.Store(false)
	})

	// Run the application
	// Pass only program name to GTK - we've already parsed our flags with pflag
	status := app.Run([]string{os.Args[0]})

	// Ensure context is cancelled
	cancel()
	_ = ctx

	if status != 0 {
		logger.Error("application exited with error", "status", status)
		os.Exit(status)
	}

	logger.Info("histuid stopped")
}

// getConfigSource returns a string describing where a config value came from.
func getConfigSource(v *viper.Viper, key string) string {
	// Check if it was set via flag (pflag)
	if pflag.CommandLine.Changed(keyToFlag(key)) {
		return "flag"
	}
	// Check if it was set via environment variable
	envKey := "HISTUID_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	if os.Getenv(envKey) != "" {
		return "env"
	}
	// Check if config file was used
	if v.ConfigFileUsed() != "" {
		return "config"
	}
	return "default"
}

// keyToFlag converts a config key to its corresponding flag name.
func keyToFlag(key string) string {
	switch key {
	case "display.position":
		return "position"
	case "display.offset_x":
		return "offset-x"
	case "display.offset_y":
		return "offset-y"
	case "display.max_visible":
		return "max-visible"
	case "display.monitor":
		return "monitor"
	case "display.new_on_top":
		return "new-on-top"
	case "theme.name":
		return "theme"
	default:
		return ""
	}
}

// checkForExternalDismissals checks if any active popups were dismissed externally.
// This is called when the store file changes (e.g., histui CLI dismissed a notification).
// It reads the current state directly from the persistence file.
func checkForExternalDismissals(
	historyStore *store.Store,
	displayManager *display.Manager,
	displayState *daemon.DisplayStateManager,
	logger *slog.Logger,
) {
	if historyStore == nil || displayManager == nil || displayState == nil {
		return
	}

	// Get all active histui IDs from the display manager
	activeIDs := displayManager.GetActiveHistuiIDs()
	if len(activeIDs) == 0 {
		return
	}

	// Build a set of active IDs for quick lookup
	activeIDSet := make(map[string]bool)
	for _, id := range activeIDs {
		activeIDSet[id] = true
	}

	// Re-read the store from disk to get the latest state
	// This creates a temporary persistence to read the file
	historyPath, err := store.HistoryPath()
	if err != nil {
		logger.Warn("failed to get history path for external check", "error", err)
		return
	}

	persistence, err := store.NewJSONLPersistence(historyPath)
	if err != nil {
		logger.Warn("failed to open persistence for external check", "error", err)
		return
	}
	defer func() { _ = persistence.Close() }()

	notifications, err := persistence.Load()
	if err != nil {
		logger.Warn("failed to load notifications for external check", "error", err)
		return
	}

	// Build index of current notifications by histui ID
	currentState := make(map[string]*model.Notification)
	for i := range notifications {
		currentState[notifications[i].HistuiID] = &notifications[i]
	}

	// Check each active notification against the current file state
	for _, histuiID := range activeIDs {
		// Skip if we've already processed this dismissal
		if dismissedIDsCache[histuiID] {
			continue
		}

		n, exists := currentState[histuiID]
		if !exists {
			// Notification was deleted from store - close the popup
			logger.Debug("notification deleted externally, closing popup", "histui_id", histuiID)
			dismissedIDsCache[histuiID] = true
			displayManager.CloseByHistuiID(histuiID, dbus.CloseReasonDismissed)
			continue
		}

		// Check if it was dismissed
		if n.IsDismissed() {
			logger.Debug("notification dismissed externally, closing popup", "histui_id", histuiID)
			dismissedIDsCache[histuiID] = true
			displayManager.CloseByHistuiID(histuiID, dbus.CloseReasonDismissed)
		}
	}
}

// convertActions converts D-Bus actions to model.Action slice.
func convertActions(dbusActions []dbus.Action) []model.Action {
	actions := make([]model.Action, len(dbusActions))
	for i, a := range dbusActions {
		actions[i] = model.Action{
			Key:   a.Key,
			Label: a.Label,
		}
	}
	return actions
}

// convertHintsToJSON converts D-Bus hints to a JSON-serializable map.
// This preserves the original hints exactly as received for faithful replay.
// Some hint values (like image-data) are skipped as they're stored separately.
func convertHintsToJSON(hints map[string]godbus.Variant) map[string]any {
	if len(hints) == 0 {
		return nil
	}

	result := make(map[string]any, len(hints))
	for key, variant := range hints {
		// Skip large binary data hints - these are stored separately
		switch key {
		case "image-data", "image_data", "icon_data":
			continue
		case "x-histui-image-png": // Our custom hint
			continue
		}

		// Convert variant value to JSON-serializable type
		value := variant.Value()

		// Handle byte slices (common in D-Bus) by skipping if too large
		if b, ok := value.([]byte); ok {
			if len(b) > 1024 { // Skip binary data > 1KB
				continue
			}
		}

		result[key] = value
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// loadThemeSounds loads sounds from the theme manifest into the audio manager.
// Theme sounds are the only source of notification sounds.
func loadThemeSounds(themeLoader *theme.Loader, audioManager *audio.Manager, logger *slog.Logger) {
	t := themeLoader.GetTheme()
	if t == nil {
		return
	}

	// Clear existing sounds first (in case switching to a theme without sounds)
	audioManager.ClearSounds()

	// Load sounds for each urgency level
	// Paths are already resolved to absolute paths by theme.resolveManifestPaths()
	for urgency := 0; urgency <= 2; urgency++ {
		soundCfg := t.GetSoundConfig(urgency)
		if soundCfg == nil || soundCfg.Path == "" {
			continue
		}

		audioManager.SetSoundForUrgency(urgency, soundCfg.Path, soundCfg.Volume)
		logger.Debug("loaded theme sound", "urgency", urgency, "path", soundCfg.Path, "volume", soundCfg.Volume)
	}
}

// suppressGraphicsDebug sets environment variables to suppress verbose
// debug output from Mesa/Vulkan/libGL that would otherwise pollute stderr.
// These graphics libraries write directly to stderr without using Go's logging.
func suppressGraphicsDebug() {
	// Only set if not already set (allow user override)
	envVars := map[string]string{
		// Suppress Mesa debug output
		"MESA_DEBUG": "silent",
		// Suppress libGL debug output
		"LIBGL_DEBUG": "quiet",
		// Suppress Vulkan loader debug output (all levels)
		"VK_LOADER_DEBUG": "none",
		// Suppress GLib log messages from Vulkan loader (goes through GTK logging)
		"G_MESSAGES_DEBUG": "",
	}

	for key, value := range envVars {
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

// convertImageDataToPNG converts D-Bus image-data format to PNG bytes.
// The D-Bus image-data format is: (width, height, rowstride, has_alpha, bits_per_sample, channels, data)
// This represents raw RGBA or RGB pixel data.
func convertImageDataToPNG(imgData *dbus.ImageDataStruct) []byte {
	if imgData == nil || len(imgData.Data) == 0 {
		return nil
	}

	width := int(imgData.Width)
	height := int(imgData.Height)
	rowstride := int(imgData.Rowstride)
	hasAlpha := imgData.HasAlpha
	channels := int(imgData.Channels)

	// Validate dimensions
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil
	}

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Copy pixel data
	for y := 0; y < height; y++ {
		rowStart := y * rowstride
		for x := 0; x < width; x++ {
			pixelStart := rowStart + x*channels
			if pixelStart+channels > len(imgData.Data) {
				break
			}

			var r, g, b, a uint8
			if channels >= 3 {
				r = imgData.Data[pixelStart]
				g = imgData.Data[pixelStart+1]
				b = imgData.Data[pixelStart+2]
			}
			if hasAlpha && channels >= 4 {
				a = imgData.Data[pixelStart+3]
			} else {
				a = 255
			}

			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}

	return buf.Bytes()
}
