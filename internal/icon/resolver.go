// Package icon provides icon name resolution and fallback handling.
package icon

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/jmylchreest/histui/internal/model"
)

// Resolver handles icon name resolution with aliases and fallbacks.
// Priority order for alias resolution:
//  1. customAliases (user-defined from config file)
//  2. themeAliases (from current theme's aliases.toml)
//  3. aliases (default/embedded aliases)
type Resolver struct {
	mu            sync.RWMutex
	aliases       map[string]string // app name -> icon name (default/embedded)
	nerdSymbols   map[string]string // app/category name -> nerd font codepoint
	customAliases map[string]string // user-defined aliases (highest priority)
	themeAliases  map[string]string // theme-provided aliases (middle priority)
}

// NewResolver creates a new icon resolver with default mappings.
func NewResolver() *Resolver {
	r := &Resolver{
		aliases:       make(map[string]string),
		nerdSymbols:   make(map[string]string),
		customAliases: make(map[string]string),
	}
	r.initDefaults()
	return r
}

// initDefaults populates fallback Nerd Font symbols.
// Primary symbols are loaded from embedded TOML via SetDefaultSymbols.
// These are only used if the TOML doesn't provide a symbol.
func (r *Resolver) initDefaults() {
	// Fallback Nerd Font symbols for categories/urgencies
	// Primary app symbols come from the generated TOML file
	r.nerdSymbols = map[string]string{
		// Fallback notification symbol
		"notification": "\U000f009a", // nf-md-bell

		// Generic categories from Desktop Notifications spec
		"im":       "\U000f0ce4", // nf-md-chat
		"device":   "\U000f03cf", // nf-md-harddisk
		"transfer": "\U000f01da", // nf-md-download
		"presence": "\U000f0061", // nf-md-account

		// Urgency fallbacks (matches histui urgency levels)
		"low":       "\U000f02fc", // nf-md-information (low priority)
		"normal":    "\U000f009a", // nf-md-bell (normal priority)
		"critical":  "\U000f0026", // nf-md-alert (critical/urgent)
		"undefined": "\U000f009c", // nf-md-bell-outline (unknown urgency)
	}
}

// Resolve returns the best icon name for the given app name.
// Priority order:
//  1. Custom aliases (user-defined from config file)
//  2. Theme aliases (from current theme's aliases.toml)
//  3. Default aliases (embedded/bundled)
//  4. Original name (for icon theme lookup)
func (r *Resolver) Resolve(appName string) string {
	if appName == "" {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Normalize app name (lowercase, no leading/trailing spaces)
	normalized := strings.ToLower(strings.TrimSpace(appName))

	// Check custom aliases first (user-defined, highest priority)
	if alias, ok := r.customAliases[normalized]; ok {
		return alias
	}

	// Check theme aliases (from current theme, middle priority)
	if alias, ok := r.themeAliases[normalized]; ok {
		return alias
	}

	// Check default aliases (embedded, lowest priority)
	if alias, ok := r.aliases[normalized]; ok {
		return alias
	}

	// Return original (as-is, for icon theme lookup)
	return appName
}

// GetNerdSymbol returns a Nerd Font symbol for the given name.
// Returns empty string if no symbol is found.
func (r *Resolver) GetNerdSymbol(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(name))
	if symbol, ok := r.nerdSymbols[normalized]; ok {
		return symbol
	}

	return ""
}

// GetNerdSymbolForCategory returns a Nerd Font symbol for a notification category.
// Categories follow the Desktop Notifications Specification format.
func (r *Resolver) GetNerdSymbolForCategory(category string) string {
	if category == "" {
		return r.nerdSymbols["notification"]
	}

	// Try exact match first
	if symbol := r.GetNerdSymbol(category); symbol != "" {
		return symbol
	}

	// Try category prefix (e.g., "email.arrived" -> "email")
	parts := strings.Split(category, ".")
	if len(parts) > 0 {
		if symbol := r.GetNerdSymbol(parts[0]); symbol != "" {
			return symbol
		}
	}

	// Default notification symbol
	return r.nerdSymbols["notification"]
}

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

// SetDefaultSymbols sets the default Nerd Font symbols (from embedded TOML).
func (r *Resolver) SetDefaultSymbols(symbols map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for iconName, symbol := range symbols {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.nerdSymbols[normalized] = symbol
	}
}

// AddSymbols adds custom Nerd Font symbol mappings.
func (r *Resolver) AddSymbols(symbols map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for iconName, symbol := range symbols {
		normalized := strings.ToLower(strings.TrimSpace(iconName))
		r.nerdSymbols[normalized] = symbol
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

// SetThemeAliases replaces all theme-level aliases with the provided map.
// This is called when the theme changes to apply theme-specific icon aliases.
// Theme aliases have middle priority (after user aliases, before defaults).
func (r *Resolver) SetThemeAliases(aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear and rebuild theme aliases
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

// FallbackNerdSymbolForUrgency returns a Nerd Font symbol based on notification urgency.
func FallbackNerdSymbolForUrgency(urgency int) string {
	switch urgency {
	case model.UrgencyCritical:
		return "\U000f0026" // nf-md-alert (F0026)
	case model.UrgencyLow:
		return "\U000f02fc" // nf-md-information
	default: // normal or unknown
		return "\U000f009a" // nf-md-bell
	}
}

// UrgencyAliasKey returns the alias key for a given urgency level.
// These keys are used in aliases.toml: urgency-low, urgency-normal, urgency-critical, urgency-unknown.
func UrgencyAliasKey(urgency int) string {
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

// GetUrgencyIconName returns the icon name for a given urgency level from theme aliases.
// Returns empty string if no urgency icon is configured.
func (r *Resolver) GetUrgencyIconName(urgency int) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := UrgencyAliasKey(urgency)

	// Check custom aliases first (user-defined, highest priority)
	if name, ok := r.customAliases[key]; ok {
		return name
	}

	// Check theme aliases (from current theme, middle priority)
	if name, ok := r.themeAliases[key]; ok {
		return name
	}

	// Check default aliases (embedded, lowest priority)
	if name, ok := r.aliases[key]; ok {
		return name
	}

	return ""
}
