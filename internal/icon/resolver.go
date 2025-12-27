// Package icon provides icon name resolution and fallback handling.
package icon

import (
	"strings"
	"sync"

	"github.com/jmylchreest/histui/internal/model"
)

// Resolver handles icon name resolution with aliases and fallbacks.
type Resolver struct {
	mu            sync.RWMutex
	aliases       map[string]string // app name -> icon name
	nerdSymbols   map[string]string // app/category name -> nerd font codepoint
	customAliases map[string]string // user-defined aliases
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
// It checks custom aliases first, then default aliases, returning
// the original name if no alias is found.
func (r *Resolver) Resolve(appName string) string {
	if appName == "" {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Normalize app name (lowercase, no leading/trailing spaces)
	normalized := strings.ToLower(strings.TrimSpace(appName))

	// Check custom aliases first
	if alias, ok := r.customAliases[normalized]; ok {
		return alias
	}

	// Check default aliases
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
func (r *Resolver) SetDefaultAliases(aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for appName, iconName := range aliases {
		normalized := strings.ToLower(strings.TrimSpace(appName))
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

// AddAlias adds a custom alias mapping.
func (r *Resolver) AddAlias(appName, iconName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	normalized := strings.ToLower(strings.TrimSpace(appName))
	r.customAliases[normalized] = iconName
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

// ListAliases returns all registered aliases (both default and custom).
func (r *Resolver) ListAliases() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string, len(r.aliases)+len(r.customAliases))
	for k, v := range r.aliases {
		result[k] = v
	}
	for k, v := range r.customAliases {
		result[k] = v // Custom aliases override defaults
	}
	return result
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

// FallbackIconForUrgency returns an appropriate fallback icon based on notification urgency.
// Use this when icon resolution fails and no notification-provided icon is available.
func FallbackIconForUrgency(urgency int) string {
	switch urgency {
	case model.UrgencyCritical:
		return "dialog-warning"
	case model.UrgencyLow:
		return "go-down-symbolic"
	default: // normal or unknown
		return "dialog-information"
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

// DefaultFallbackIcon returns the default fallback icon name (for normal urgency).
func DefaultFallbackIcon() string {
	return "dialog-information"
}

// DefaultNerdSymbol returns the default Nerd Font notification symbol.
func DefaultNerdSymbol() string {
	return "\U000f009a" // nf-md-bell
}
