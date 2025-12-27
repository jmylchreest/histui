// Package main is the entry point for the histuid notification daemon.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/jmylchreest/histui/internal/audio"
	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/daemon"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/display"
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
	// Parse command line flags
	monitorMode := flag.Bool("monitor", false, "Run in monitor mode (passive, no popups/sounds, works alongside another notification daemon)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		println("histuid version", version)
		os.Exit(0)
	}

	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
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
				logger.Info("captured notification", "id", id, "app", n.AppName, "summary", n.Summary)
			}
		} else {
			logger.Debug("skipped transient notification", "id", id, "app", n.AppName)
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

	logger.Info("histuid monitor stopped")
}

// runDaemonMode runs histuid as the primary notification daemon with full functionality.
func runDaemonMode(logger *slog.Logger) {
	logger.Info("starting histuid", "version", version)

	// Load configuration
	cfg, err := config.LoadDaemonConfig()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Create the libadwaita application
	app := adw.NewApplication(appID, 0)

	// Shared state between GTK main loop and signal handlers
	var (
		dbusServer       *dbus.NotificationServer
		displayManager   *display.Manager
		themeLoader      *theme.Loader
		audioManager     *audio.Manager
		historyStore     *store.Store
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
				if historyStore != nil {
					_ = historyStore.Close()
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
		themeLoader.StartHotReload(ctx)

		// Initialize audio manager
		audioManager = audio.NewManager(cfg, logger)
		if err := audioManager.Start(ctx); err != nil {
			logger.Warn("failed to start audio manager", "error", err)
		}

		// Initialize display manager
		displayManager = display.NewManager(&app.Application, cfg, logger)
		if err := displayManager.Start(); err != nil {
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

			// Don't persist transient notifications
			if !notification.Transient() {
				if err := historyStore.Add(*n); err != nil {
					logger.Error("failed to persist notification", "id", id, "error", err)
				}
			}

			// Track the mapping between D-Bus ID and histui ID
			timeout := cfg.GetTimeoutForUrgency(notification.Urgency())
			var expiresAt time.Time
			if timeout > 0 {
				expiresAt = time.Now().Add(time.Duration(timeout) * time.Millisecond)
			}
			displayState.Register(n.HistuiID, id, expiresAt)

			// Check if DnD is enabled (suppress popups and sounds)
			urgency := notification.Urgency()
			isDnDEnabled := sharedState != nil && sharedState.DnDEnabled
			isCriticalBypass := cfg.DnD.CriticalBypass && urgency == 2 // Critical urgency

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

		// Initialize store watcher for external changes (e.g., histui CLI dismiss)
		storeWatcher = daemon.NewStoreWatcher(historyPath, logger)
		storeWatcher.SetChangeCallback(func() {
			// Store file changed - check for dismissed notifications
			glib.IdleAdd(func() {
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
							internalNotifier.NotifyThemeReloaded(newConfig.Theme.Name)
						}
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
		if historyStore != nil {
			_ = historyStore.Close()
		}
		running.Store(false)
	})

	// Run the application
	status := app.Run(os.Args)

	// Ensure context is cancelled
	cancel()
	_ = ctx

	if status != 0 {
		logger.Error("application exited with error", "status", status)
		os.Exit(status)
	}

	logger.Info("histuid stopped")
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
