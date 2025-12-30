// Package icon provides icon name resolution and fallback handling.
package icon

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/jmylchreest/histui/internal/model"
)

// DefaultFallbackSymbol is the ultimate fallback when no icon can be resolved.
// Uses nf-md-bell-badge (󰵛) - a notification bell with a badge indicator.
const DefaultFallbackSymbol = "󰵛" // U+F0D9B

// DefaultFallbackGTKIcon is the ultimate GTK icon fallback.
const DefaultFallbackGTKIcon = "notification-symbolic"

// Resolver handles icon name resolution with aliases, symbols, and GTK icons.
//
// Priority order for all lookups (user > theme > default):
//   - customAliases/customSymbols/customGtkIcons (user-defined from config file)
//   - themeAliases/themeSymbols/themeGtkIcons (from current theme's aliases.toml)
//   - aliases/symbols/gtkIcons (default/embedded aliases)
//
// Lookup flow for daemon (GTK):
//  1. ResolveApp(appName) → canonical name
//  2. GetGTKIcon(canonical) → GTK icon name for display
//  3. Fallback: GetSymbol(canonical) → symbol font glyph
//
// Lookup flow for TUI:
//  1. ResolveApp(appName) → canonical name
//  2. GetSymbol(canonical) → symbol font glyph
type Resolver struct {
	mu     sync.RWMutex
	logger *slog.Logger

	// Alias maps (app name → canonical icon name)
	aliases       map[string]string // default/embedded
	themeAliases  map[string]string // from theme
	customAliases map[string]string // user-defined (highest priority)

	// Symbol maps (canonical name → symbol font glyph character)
	symbols       map[string]string // default/embedded
	themeSymbols  map[string]string // from theme
	customSymbols map[string]string // user-defined (highest priority)

	// GTK icon maps (canonical name → GTK icon name)
	gtkIcons       map[string]string // default/embedded
	themeGtkIcons  map[string]string // from theme
	customGtkIcons map[string]string // user-defined (highest priority)
}

// NewResolver creates a new icon resolver.
// Default mappings are loaded separately via SetDefaultSymbols/SetDefaultGtkIcons.
func NewResolver() *Resolver {
	return &Resolver{
		logger:         slog.Default(),
		aliases:        make(map[string]string),
		themeAliases:   make(map[string]string),
		customAliases:  make(map[string]string),
		symbols:        make(map[string]string),
		themeSymbols:   make(map[string]string),
		customSymbols:  make(map[string]string),
		gtkIcons:       make(map[string]string),
		themeGtkIcons:  make(map[string]string),
		customGtkIcons: make(map[string]string),
	}
}

// SetLogger sets the logger for debug output.
func (r *Resolver) SetLogger(logger *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if logger != nil {
		r.logger = logger
	}
}

// =============================================================================
// Core Resolution Methods
// =============================================================================

// ResolveApp returns the canonical icon name for the given app name.
// Priority order:
//  1. Custom aliases (user-defined from config file)
//  2. Theme aliases (from current theme's aliases.toml)
//  3. Default aliases (embedded/bundled)
//  4. Original name (if no alias found)
//
// Empty values in config are skipped (allows themes to not override defaults).
func (r *Resolver) ResolveApp(appName string) string {
	if appName == "" {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Normalize app name (lowercase, no leading/trailing spaces)
	normalized := strings.ToLower(strings.TrimSpace(appName))

	// Check custom aliases first (user-defined, highest priority)
	// Skip empty values - they mean "don't override"
	if alias, ok := r.customAliases[normalized]; ok && alias != "" {
		r.logger.Debug("app resolved from user config",
			"input", appName, "normalized", normalized, "resolved", alias, "source", "user")
		return alias
	}

	// Check theme aliases (from current theme, middle priority)
	if alias, ok := r.themeAliases[normalized]; ok && alias != "" {
		r.logger.Debug("app resolved from theme",
			"input", appName, "normalized", normalized, "resolved", alias, "source", "theme")
		return alias
	}

	// Check default aliases (embedded, lowest priority)
	if alias, ok := r.aliases[normalized]; ok && alias != "" {
		r.logger.Debug("app resolved from defaults",
			"input", appName, "normalized", normalized, "resolved", alias, "source", "defaults")
		return alias
	}

	// Return original (as-is, for icon theme lookup)
	r.logger.Debug("app not found in aliases, using original",
		"input", appName, "normalized", normalized,
		"checked_user", len(r.customAliases) > 0,
		"checked_theme", len(r.themeAliases) > 0,
		"checked_defaults", len(r.aliases) > 0)
	return appName
}

// Resolve is an alias for ResolveApp for backwards compatibility.
func (r *Resolver) Resolve(appName string) string {
	return r.ResolveApp(appName)
}

// ResolveUrgency returns the canonical icon name for a given urgency level.
// Uses urgency-* keys from aliases (urgency-low, urgency-normal, urgency-critical, urgency-unknown).
// Empty values in config are skipped (allows themes to not override defaults).
func (r *Resolver) ResolveUrgency(urgency int) string {
	key := urgencyAliasKey(urgency)

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check custom aliases first (user-defined, highest priority)
	// Skip empty values - they mean "don't override"
	if name, ok := r.customAliases[key]; ok && name != "" {
		r.logger.Debug("urgency resolved from user config",
			"urgency", urgency, "key", key, "canonical", name, "source", "user")
		return name
	}

	// Check theme aliases (from current theme, middle priority)
	if name, ok := r.themeAliases[key]; ok && name != "" {
		r.logger.Debug("urgency resolved from theme",
			"urgency", urgency, "key", key, "canonical", name, "source", "theme")
		return name
	}

	// Check default aliases (embedded, lowest priority)
	if name, ok := r.aliases[key]; ok && name != "" {
		r.logger.Debug("urgency resolved from defaults",
			"urgency", urgency, "key", key, "canonical", name, "source", "defaults")
		return name
	}

	// Fallback to urgency key itself (urgency-low, urgency-normal, urgency-critical)
	// This allows gtk-icons to be keyed by urgency-normal etc.
	r.logger.Debug("urgency not found in aliases, using key as canonical",
		"urgency", urgency, "key", key)
	return key
}

// GetSymbol returns a symbol font glyph for the given canonical name.
// Returns empty string if no symbol is found.
// Empty values in config are skipped (allows themes to not override defaults).
func (r *Resolver) GetSymbol(canonicalName string) string {
	if canonicalName == "" {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(canonicalName))

	// Check custom symbols first (user-defined, highest priority)
	// Skip empty values - they mean "don't override"
	if symbol, ok := r.customSymbols[normalized]; ok && symbol != "" {
		r.logger.Debug("symbol found in user config",
			"name", canonicalName, "normalized", normalized, "symbol_codepoint", symbolToHex(symbol), "source", "user")
		return symbol
	}

	// Check theme symbols (middle priority)
	if symbol, ok := r.themeSymbols[normalized]; ok && symbol != "" {
		r.logger.Debug("symbol found in theme",
			"name", canonicalName, "normalized", normalized, "symbol_codepoint", symbolToHex(symbol), "source", "theme")
		return symbol
	}

	// Check default symbols (lowest priority)
	if symbol, ok := r.symbols[normalized]; ok && symbol != "" {
		r.logger.Debug("symbol found in defaults",
			"name", canonicalName, "normalized", normalized, "symbol_codepoint", symbolToHex(symbol), "source", "defaults")
		return symbol
	}

	r.logger.Debug("symbol not found",
		"name", canonicalName, "normalized", normalized)
	return ""
}

// GetGTKIcon returns a GTK icon name for the given canonical name.
// Returns empty string if no GTK icon mapping is found.
// Empty values in config are skipped (allows themes to not override defaults).
func (r *Resolver) GetGTKIcon(canonicalName string) string {
	if canonicalName == "" {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(canonicalName))

	// Check custom GTK icons first (user-defined, highest priority)
	// Skip empty values - they mean "don't override"
	if icon, ok := r.customGtkIcons[normalized]; ok && icon != "" {
		r.logger.Debug("GTK icon found in user config",
			"name", canonicalName, "normalized", normalized, "gtk_icon", icon, "source", "user")
		return icon
	}

	// Check theme GTK icons (middle priority)
	if icon, ok := r.themeGtkIcons[normalized]; ok && icon != "" {
		r.logger.Debug("GTK icon found in theme",
			"name", canonicalName, "normalized", normalized, "gtk_icon", icon, "source", "theme")
		return icon
	}

	// Check default GTK icons (lowest priority)
	if icon, ok := r.gtkIcons[normalized]; ok && icon != "" {
		r.logger.Debug("GTK icon found in defaults",
			"name", canonicalName, "normalized", normalized, "gtk_icon", icon, "source", "defaults")
		return icon
	}

	r.logger.Debug("GTK icon not found",
		"name", canonicalName, "normalized", normalized)
	return ""
}

// =============================================================================
// Convenience Methods (composing core methods)
// =============================================================================

// GetSymbolForApp resolves an app name and returns its symbol.
// Equivalent to: GetSymbol(ResolveApp(appName))
func (r *Resolver) GetSymbolForApp(appName string) string {
	canonical := r.ResolveApp(appName)
	return r.GetSymbol(canonical)
}

// GetGTKIconForApp resolves an app name and returns its GTK icon.
// Equivalent to: GetGTKIcon(ResolveApp(appName))
func (r *Resolver) GetGTKIconForApp(appName string) string {
	canonical := r.ResolveApp(appName)
	return r.GetGTKIcon(canonical)
}

// GetSymbolForUrgency returns a symbol for the given urgency level.
// Equivalent to: GetSymbol(ResolveUrgency(urgency))
func (r *Resolver) GetSymbolForUrgency(urgency int) string {
	canonical := r.ResolveUrgency(urgency)
	if symbol := r.GetSymbol(canonical); symbol != "" {
		return symbol
	}
	// Fallback to hardcoded urgency symbols
	return r.getDefaultUrgencySymbol(urgency)
}

// GetGTKIconForUrgency returns a GTK icon for the given urgency level.
// Equivalent to: GetGTKIcon(ResolveUrgency(urgency))
func (r *Resolver) GetGTKIconForUrgency(urgency int) string {
	canonical := r.ResolveUrgency(urgency)
	if icon := r.GetGTKIcon(canonical); icon != "" {
		return icon
	}
	// Fallback to hardcoded urgency GTK icons
	return r.getDefaultUrgencyGTKIcon(urgency)
}

// GetSymbolForCategory returns a symbol for a notification category.
// Categories follow the Desktop Notifications Specification format.
func (r *Resolver) GetSymbolForCategory(category string) string {
	if category == "" {
		return r.GetSymbol("notification")
	}

	// Try exact match first
	if symbol := r.GetSymbol(category); symbol != "" {
		r.logger.Debug("category symbol found (exact match)",
			"category", category, "symbol_codepoint", symbolToHex(symbol))
		return symbol
	}

	// Try category prefix (e.g., "email.arrived" -> "email")
	parts := strings.Split(category, ".")
	if len(parts) > 0 {
		if symbol := r.GetSymbol(parts[0]); symbol != "" {
			r.logger.Debug("category symbol found (prefix match)",
				"category", category, "prefix", parts[0], "symbol_codepoint", symbolToHex(symbol))
			return symbol
		}
	}

	// Default notification symbol
	defaultSymbol := r.GetSymbol("notification")
	r.logger.Debug("category symbol not found, using default",
		"category", category, "symbol_codepoint", symbolToHex(defaultSymbol))
	return defaultSymbol
}

// =============================================================================
// Backwards Compatibility (deprecated, use new API)
// =============================================================================

// GetNerdSymbol returns a symbol for the given name.
// Deprecated: Use GetSymbol instead.
func (r *Resolver) GetNerdSymbol(name string) string {
	return r.GetSymbol(name)
}

// GetNerdSymbolForCategory returns a symbol for a notification category.
// Deprecated: Use GetSymbolForCategory instead.
func (r *Resolver) GetNerdSymbolForCategory(category string) string {
	return r.GetSymbolForCategory(category)
}

// GetNerdSymbolForUrgency returns a symbol for the given urgency level.
// Deprecated: Use GetSymbolForUrgency instead.
func (r *Resolver) GetNerdSymbolForUrgency(urgency int) string {
	return r.GetSymbolForUrgency(urgency)
}

// GetUrgencyIconName returns the canonical icon name for a given urgency level.
// Deprecated: Use ResolveUrgency instead.
func (r *Resolver) GetUrgencyIconName(urgency int) string {
	return r.ResolveUrgency(urgency)
}

// FallbackNerdSymbolForUrgency returns a fallback symbol when nothing else resolves.
// Deprecated: Use Resolver.GetSymbolForUrgency which respects user config.
func FallbackNerdSymbolForUrgency(_ int) string {
	return DefaultFallbackSymbol
}

// =============================================================================
// Setters for loading data
// =============================================================================

// SetDefaultAliases sets the default aliases (typically from embedded TOML).
// If a key already exists, the first value is kept and a warning is logged.
func (r *Resolver) SetDefaultAliases(aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for appName, iconName := range aliases {
		normalized := strings.ToLower(strings.TrimSpace(appName))
		if existing, ok := r.aliases[normalized]; ok {
			slog.Warn("duplicate icon alias, keeping first",
				"app", appName,
				"existing", existing,
				"ignored", iconName,
			)
			continue
		}
		r.aliases[normalized] = iconName
	}
}

// SetDefaultSymbols sets the default symbols (from embedded TOML).
func (r *Resolver) SetDefaultSymbols(symbols map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for iconName, symbol := range symbols {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.symbols[normalized] = symbol
	}
}

// SetDefaultGtkIcons sets the default GTK icon mappings (from embedded TOML).
func (r *Resolver) SetDefaultGtkIcons(gtkIcons map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for iconName, gtkIcon := range gtkIcons {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.gtkIcons[normalized] = gtkIcon
	}
}

// AddAliases adds multiple custom alias mappings.
func (r *Resolver) AddAliases(aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for appName, iconName := range aliases {
		normalized := strings.ToLower(strings.TrimSpace(appName))
		r.customAliases[normalized] = iconName
	}
}

// AddSymbols adds custom symbol mappings.
func (r *Resolver) AddSymbols(symbols map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for iconName, symbol := range symbols {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.customSymbols[normalized] = symbol
	}
}

// AddGtkIcons adds custom GTK icon mappings.
func (r *Resolver) AddGtkIcons(gtkIcons map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for iconName, gtkIcon := range gtkIcons {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.customGtkIcons[normalized] = gtkIcon
	}
}

// SetUserAliases replaces all custom (user) aliases with the provided map.
// This is used for hot-reloading user aliases from the config file.
func (r *Resolver) SetUserAliases(aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear and rebuild custom aliases
	r.customAliases = make(map[string]string, len(aliases))
	for appName, iconName := range aliases {
		normalized := strings.ToLower(strings.TrimSpace(appName))
		r.customAliases[normalized] = iconName
	}
}

// SetUserSymbols replaces all custom (user) symbols with the provided map.
func (r *Resolver) SetUserSymbols(symbols map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.customSymbols = make(map[string]string, len(symbols))
	for iconName, symbol := range symbols {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.customSymbols[normalized] = symbol
	}
}

// SetUserGtkIcons replaces all custom (user) GTK icon mappings.
func (r *Resolver) SetUserGtkIcons(gtkIcons map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.customGtkIcons = make(map[string]string, len(gtkIcons))
	for iconName, gtkIcon := range gtkIcons {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.customGtkIcons[normalized] = gtkIcon
	}
}

// SetThemeAliases replaces all theme-level aliases with the provided map.
// This is called when the theme changes to apply theme-specific icon aliases.
// Theme aliases have middle priority (after user aliases, before defaults).
func (r *Resolver) SetThemeAliases(aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if aliases == nil {
		r.themeAliases = nil
		return
	}

	r.themeAliases = make(map[string]string, len(aliases))
	for appName, iconName := range aliases {
		normalized := strings.ToLower(strings.TrimSpace(appName))
		r.themeAliases[normalized] = iconName
	}
}

// SetThemeSymbols replaces all theme-level symbols with the provided map.
func (r *Resolver) SetThemeSymbols(symbols map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if symbols == nil {
		r.themeSymbols = nil
		return
	}

	r.themeSymbols = make(map[string]string, len(symbols))
	for iconName, symbol := range symbols {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.themeSymbols[normalized] = symbol
	}
}

// SetThemeGtkIcons replaces all theme-level GTK icon mappings.
func (r *Resolver) SetThemeGtkIcons(gtkIcons map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if gtkIcons == nil {
		r.themeGtkIcons = nil
		return
	}

	r.themeGtkIcons = make(map[string]string, len(gtkIcons))
	for iconName, gtkIcon := range gtkIcons {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.themeGtkIcons[normalized] = gtkIcon
	}
}

// =============================================================================
// Internal helpers
// =============================================================================

// urgencyAliasKey returns the alias key for a given urgency level.
func urgencyAliasKey(urgency int) string {
	switch urgency {
	case model.UrgencyLow:
		return "urgency-low"
	case model.UrgencyNormal:
		return "urgency-normal"
	case model.UrgencyCritical:
		return "urgency-critical"
	default:
		return "urgency-unknown"
	}
}

// getDefaultUrgencySymbol returns fallback symbol for urgency.
// Looks up from embedded config first, falls back to DefaultFallbackSymbol.
func (r *Resolver) getDefaultUrgencySymbol(urgency int) string {
	// Try to get from symbols map (loaded from embedded config)
	key := urgencyAliasKey(urgency)
	if symbol, ok := r.symbols[key]; ok && symbol != "" {
		return symbol
	}
	// Ultimate fallback
	return DefaultFallbackSymbol
}

// getDefaultUrgencyGTKIcon returns fallback GTK icon for urgency.
// Looks up from embedded config first, falls back to DefaultFallbackGTKIcon.
func (r *Resolver) getDefaultUrgencyGTKIcon(urgency int) string {
	// Try to get from gtkIcons map (loaded from embedded config)
	key := urgencyAliasKey(urgency)
	if icon, ok := r.gtkIcons[key]; ok && icon != "" {
		return icon
	}
	// Ultimate fallback
	return DefaultFallbackGTKIcon
}

// symbolToHex converts a symbol string to hex codepoint for logging
func symbolToHex(s string) string {
	if s == "" {
		return "(empty)"
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return "(empty)"
	}
	// Format: U+XXXXX 'glyph'
	return "U+" + strings.ToUpper(fmt.Sprintf("%04X", runes[0])) + " '" + s + "'"
}
