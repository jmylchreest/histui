// Package main is the entry point for the histuid notification daemon.
package main

import (
	"context"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
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
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/display"
	"github.com/jmylchreest/histui/internal/icon"
	"github.com/jmylchreest/histui/internal/model"
	"github.com/jmylchreest/histui/internal/theme"
)

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
	pprofAddr := pflag.String("pprof", "", "Enable pprof profiling server on address (e.g., 'localhost:6060')")

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

	// Start pprof server if requested (for memory/CPU profiling)
	if *pprofAddr != "" {
		go func() {
			// pprof handlers are automatically registered at /debug/pprof/
			// Access heap profile: curl http://localhost:6060/debug/pprof/heap > heap.out
			// Then analyze: go tool pprof heap.out
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				slog.Error("pprof server failed", "error", err)
			}
		}()
		slog.Info("pprof profiling enabled", "addr", *pprofAddr)
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

	// Initialize SQLite database
	database, err := db.Open("")
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	count, _ := database.Count()
	logger.Info("database initialized", "path", database.Path(), "count", count)

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
			if err := database.AddNotification(n); err != nil {
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
	if err := database.Close(); err != nil {
		logger.Warn("error closing database", "error", err)
	}
	daemon.CleanupTemp(logger)

	logger.Info("histuid monitor stopped")
}

// runDaemonMode runs histuid as the primary notification daemon with full functionality.
func runDaemonMode(logger *slog.Logger) {
	logger.Info("starting histuid", "version", version)

	// Clean up any leftover temp files from previous runs (in case of unclean shutdown)
	daemon.CleanupTemp(logger)

	// Check D-Bus name availability BEFORE initializing GTK
	// This must happen early because GTK application uniqueness check
	// would silently exit if another instance is running, without calling our activate callback
	if err := dbus.CheckBusNameAvailable(); err != nil {
		logger.Error("cannot start notification daemon", "error", err)
		os.Exit(1)
	}

	// Initialize embedded fonts (Nerd Font symbols, etc.) for icon fallback
	// Must be done before GTK initialization so fontconfig picks it up
	if fontDir, err := icon.InitFonts(); err != nil {
		logger.Warn("failed to initialize embedded fonts", "error", err)
	} else {
		logger.Debug("initialized embedded fonts", "dir", fontDir, "fonts", icon.ListEmbeddedFonts())
	}

	// Log embedded aliases metadata and statistics
	if stats, err := icon.GetEmbeddedAliasesStats(); err != nil {
		logger.Debug("failed to get embedded aliases stats", "error", err)
	} else if stats.Meta.GeneratedAt != "" {
		logger.Info("loaded embedded icon aliases",
			"version", stats.Meta.Version,
			"generated", stats.Meta.GeneratedAt,
			"aliases", stats.Aliases,
			"apps", stats.Apps,
			"categories", stats.Categories)
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
		daemonServer     *dbus.DaemonServer
		displayManager   *display.Manager
		themeLoader      *theme.Loader
		audioManager     *audio.Manager
		database         *db.DB
		dndManager       *db.DnDManager
		displayState     *daemon.DisplayStateManager
		configWatcher    *daemon.ConfigWatcher
		internalNotifier *daemon.InternalNotifier
		updateNotifier   *daemon.Debouncer
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
				if daemonServer != nil {
					_ = daemonServer.Stop()
				}
				if audioManager != nil {
					audioManager.Stop()
				}
				if themeLoader != nil {
					themeLoader.StopHotReload()
				}
				if configWatcher != nil {
					configWatcher.Stop()
				}
				if displayManager != nil {
					displayManager.Stop()
				}
				if dbusServer != nil {
					_ = dbusServer.Stop()
				}
				if database != nil {
					_ = database.Close()
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

		// Note: D-Bus name availability is checked at the start of runDaemonMode(),
		// before GTK initialization, to ensure proper error reporting.

		// Initialize SQLite database
		database, err = db.Open("")
		if err != nil {
			logger.Error("failed to open database", "error", err)
			app.Quit()
			return
		}

		// Get initial count
		count, _ := database.Count()
		logger.Info("database initialized", "path", database.Path(), "count", count)

		// Create update notifier - debounced file touch to signal histui
		updateNotifier = daemon.NewDebouncer(500*time.Millisecond, func() {
			if err := db.TouchLastUpdated(); err != nil {
				logger.Debug("failed to touch last updated file", "error", err)
			}
		})

		// Run initial prune on startup if configured
		if cfg.History.MaxNotifications > 0 {
			pruned, err := database.Prune(cfg.History.MaxNotifications)
			if err != nil {
				logger.Warn("failed to prune on startup", "error", err)
			} else if pruned > 0 {
				logger.Info("pruned old notifications on startup", "count", pruned)
			}
		}

		// Initialize DnD manager (in-memory, controlled via IPC)
		dndManager = db.NewDnDManager()

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

		// Apply theme icon settings to display manager
		if loadedTheme := themeLoader.GetTheme(); loadedTheme != nil {
			displayManager.UpdateTheme(loadedTheme.Aliases, loadedTheme.Symbols, loadedTheme.GtkIcons, loadedTheme.IconsDir)
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
			// If so, we'll use the original histui ID for tracking popups
			var trackingID string
			isReplay := dbus.IsReplayHint(notification.Hints)
			if isReplay {
				originalID := dbus.GetOriginalID(notification.Hints)
				if originalID != "" {
					trackingID = originalID
					// Update existing notification's replayed status
					if err := database.MarkReplayed(originalID); err != nil {
						logger.Warn("failed to update replayed notification", "id", originalID, "error", err)
					} else {
						logger.Debug("marked notification as replayed", "id", originalID)
					}
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
				StackTag:     notification.StackTag(),
				Progress:     notification.Progress(),
			}

			// Store original hints for faithful replay
			n.OriginalHints = convertHintsToJSON(notification.Hints)

			// Use trackingID for replays, otherwise use the new notification's ID
			if trackingID == "" {
				trackingID = n.HistuiID
			}

			// Don't persist transient notifications or replays
			if !notification.Transient() && !isReplay {
				if err := database.AddNotification(n); err != nil {
					logger.Error("failed to persist notification", "id", id, "error", err)
				} else {
					// Signal histui that notifications changed
					updateNotifier.Trigger()
				}

				// Capture and store image data if enabled. PNG encoding allocates a
				// full RGBA buffer (up to 4096x4096) and runs deflate, so do it off
				// the D-Bus dispatch goroutine to avoid delaying the Notify reply to
				// the sending application.
				if cfg.History.StoreImages {
					if imgData := notification.ImageData(); imgData != nil {
						histuiID := n.HistuiID
						go func() {
							pngData := imgData.ToPNG()
							if len(pngData) == 0 {
								return
							}
							if err := database.SaveImage(histuiID, db.ImageRefImage, pngData); err != nil {
								logger.Warn("failed to save notification image", "error", err)
							}
						}()
					}
				}
			}

			// Track the mapping between D-Bus ID and histui ID (use trackingID for replays)
			timeout := cfg.GetTimeoutForUrgency(notification.Urgency(), notification.ExpireTimeout)
			var expiresAt time.Time
			if timeout > 0 {
				expiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
			}
			displayState.Register(trackingID, id, expiresAt)

			// Check if DnD is enabled (suppress popups and sounds)
			urgency := notification.Urgency()
			isDnDEnabled := dndManager != nil && dndManager.Enabled()
			// By default (suppress_critical=false), critical notifications bypass DnD
			allowCriticalBypass := !cfg.DnD.SuppressCritical && urgency == model.UrgencyCritical

			// Suppress popup and sound if DnD is enabled (unless critical bypass is allowed)
			if isDnDEnabled && !allowCriticalBypass {
				logger.Debug("notification suppressed by DnD", "id", id, "urgency", urgency)
				// Note: Notification is still persisted to database (done above)
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

			// Schedule display on GTK main loop (use trackingID for signal emission)
			histuiIDForDisplay := trackingID
			glib.IdleAdd(func() {
				if err := displayManager.Show(notification, id, histuiIDForDisplay); err != nil {
					logger.Error("failed to show notification", "id", id, "error", err)
				}
			})
		})

		dbusServer.SetCloseHandler(func(id uint32) {
			glib.IdleAdd(func() {
				displayManager.Close(id, dbus.CloseReasonClosed)
			})
		})

		// Connect display manager callbacks to D-Bus and database
		// histuiIDs includes all stacked notification IDs that should be dismissed together
		displayManager.SetCloseCallback(func(dbusID uint32, histuiIDs []string, reason dbus.CloseReason) {
			// Emit standard D-Bus NotificationClosed signal
			if err := dbusServer.CloseWithReason(dbusID, reason); err != nil {
				logger.Warn("failed to emit close signal", "id", dbusID, "error", err)
			}

			// Emit daemon NotificationDismissed signal for histui tracking
			// Emit for all histuiIDs (primary + stacked)
			if daemonServer != nil && len(histuiIDs) > 0 {
				var reasonStr string
				switch reason {
				case dbus.CloseReasonDismissed:
					reasonStr = "dismissed"
				case dbus.CloseReasonClosed:
					reasonStr = "closed"
				default:
					reasonStr = "expired"
				}
				for _, histuiID := range histuiIDs {
					if err := daemonServer.EmitNotificationDismissed(histuiID, reasonStr); err != nil {
						logger.Warn("failed to emit NotificationDismissed signal", "histui_id", histuiID, "error", err)
					}
				}
			}

			// Update database if user dismissed (not expired)
			// Dismiss all notifications in the stack
			if reason == dbus.CloseReasonDismissed && len(histuiIDs) > 0 {
				if err := database.DismissBatch(histuiIDs); err != nil {
					logger.Warn("failed to mark notifications as dismissed", "histui_ids", histuiIDs, "error", err)
				} else if len(histuiIDs) > 0 {
					// Signal histui that notifications changed
					updateNotifier.Trigger()
				}
			}

			// Update display state for all histuiIDs
			for _, histuiID := range histuiIDs {
				displayState.RemoveByHistuiID(histuiID)
			}
			displayState.RemoveByDBusID(dbusID)
		})

		displayManager.SetActionCallback(func(dbusID uint32, actionKey string) {
			if err := dbusServer.EmitActionInvoked(dbusID, actionKey); err != nil {
				logger.Warn("failed to emit action signal", "id", dbusID, "error", err)
			}
		})

		// Set display callback for D-Bus signal emission
		// This will be connected to daemonServer after it's created
		displayManager.SetDisplayCallback(func(histuiID string) {
			if daemonServer != nil {
				if err := daemonServer.EmitNotificationDisplayed(histuiID); err != nil {
					logger.Warn("failed to emit NotificationDisplayed signal", "histui_id", histuiID, "error", err)
				}
			}
		})

		// Start D-Bus notification server
		if err := dbusServer.Start(); err != nil {
			logger.Error("failed to start D-Bus server", "error", err)
			displayManager.Stop()
			app.Quit()
			return
		}

		// Start D-Bus daemon server for histui CLI communication
		daemonHandler := daemon.NewDaemonHandler(dndManager, audioManager, displayManager)
		daemonServer = dbus.NewDaemonServer(dbusServer.Connection(), daemonHandler, version, logger)
		if err := daemonServer.Start(); err != nil {
			logger.Warn("failed to start daemon D-Bus server", "error", err)
		}

		// Wire up DnD change callback to emit D-Bus signal
		dndManager.SetChangeCallback(func(enabled bool) {
			if daemonServer != nil {
				if err := daemonServer.EmitDnDChanged(enabled); err != nil {
					logger.Warn("failed to emit DnDChanged signal", "error", err)
				}
			}
		})

		// Initialize internal notifier for self-notifications
		internalNotifier = daemon.NewInternalNotifier(logger)
		internalNotifier.SetNotifyHandler(func(notification *dbus.DBusNotification) uint32 {
			// Use the D-Bus server to create the notification internally
			return dbusServer.NotifyInternal(notification)
		})

		// Set up theme hot-reload notification callback
		themeLoader.SetHotReloadCallback(func(themeName string, changedFile string) {
			internalNotifier.NotifyThemeHotReload(themeName, changedFile)
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
							// Update watcher to watch new theme's imported files
							themeLoader.RefreshWatcher()
							// Reload sounds from new theme
							loadThemeSounds(themeLoader, audioManager, logger)
							// Update display manager with theme icon settings
							if loadedTheme := themeLoader.GetTheme(); loadedTheme != nil {
								displayManager.UpdateTheme(loadedTheme.Aliases, loadedTheme.Symbols, loadedTheme.GtkIcons, loadedTheme.IconsDir)
							}
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

					// Emit D-Bus signal for histui TUI to refresh
					if daemonServer != nil {
						if err := daemonServer.EmitConfigChanged(); err != nil {
							logger.Warn("failed to emit ConfigChanged signal", "error", err)
						}
					}
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
		if daemonServer != nil {
			_ = daemonServer.Stop()
		}
		if audioManager != nil {
			audioManager.Stop()
		}
		if themeLoader != nil {
			themeLoader.StopHotReload()
		}
		if configWatcher != nil {
			configWatcher.Stop()
		}
		if displayManager != nil {
			displayManager.Stop()
		}
		if dbusServer != nil {
			_ = dbusServer.Stop()
		}
		if updateNotifier != nil {
			updateNotifier.Stop()
		}
		if database != nil {
			_ = database.Close()
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
// debug output from GTK/Vulkan that would otherwise pollute stderr.
func suppressGraphicsDebug() {
	// Only set if not already set (allow user override)
	envVars := map[string]string{
		// Suppress GLib/GTK debug messages (includes Gdk domain)
		"G_MESSAGES_DEBUG": "",
		// Only show errors/warnings from Vulkan loader, suppress info/debug
		"VK_LOADER_DEBUG": "error,warn",
	}

	for key, value := range envVars {
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}
