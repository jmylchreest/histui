package icon

import (
	"testing"

	"github.com/jmylchreest/histui/internal/model"
)

func TestResolverDefaultAliases(t *testing.T) {
	// Use NewResolverWithAliases to get embedded default aliases
	r, err := NewResolverWithAliases()
	if err != nil {
		t.Fatalf("NewResolverWithAliases() error: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"whatsapp-desktop", "whatsapp"},
		{"WhatsApp-Desktop", "whatsapp"}, // case insensitive
		{"  whatsapp-desktop  ", "whatsapp"}, // trims spaces
		{"discord-canary", "discord"},
		{"firefox-esr", "firefox"},
		{"firefox", "firefox"},         // no alias, returns as-is
		{"unknown-app", "unknown-app"}, // no alias, returns as-is
		{"", ""},                       // empty returns empty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := r.Resolve(tt.input)
			if got != tt.expected {
				t.Errorf("Resolve(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolverCustomAliases(t *testing.T) {
	r := NewResolver()

	// Add custom alias that overrides default
	r.AddAliases(map[string]string{"zapzap": "custom-whatsapp"})

	if got := r.Resolve("zapzap"); got != "custom-whatsapp" {
		t.Errorf("Resolve(zapzap) = %q, want custom-whatsapp", got)
	}

	// Add new custom alias
	r.AddAliases(map[string]string{"myapp": "myicon"})
	if got := r.Resolve("myapp"); got != "myicon" {
		t.Errorf("Resolve(myapp) = %q, want myicon", got)
	}
}

func TestResolverNerdSymbols(t *testing.T) {
	// Use NewResolverWithAliases to get embedded default symbols
	r, err := NewResolverWithAliases()
	if err != nil {
		t.Fatalf("NewResolverWithAliases() error: %v", err)
	}

	// Urgency symbol from embedded defaults
	if got := r.GetNerdSymbol("urgency-critical"); got == "" {
		t.Error("GetNerdSymbol(urgency-critical) returned empty string")
	}

	// Unknown symbol returns empty
	if got := r.GetNerdSymbol("unknown-nonexistent"); got != "" {
		t.Errorf("GetNerdSymbol(unknown-nonexistent) = %q, want empty string", got)
	}

	// Category lookup - "im" is a known category from embedded defaults
	if got := r.GetNerdSymbolForCategory("im.message"); got == "" {
		t.Error("GetNerdSymbolForCategory(im.message) returned empty string")
	}

	// Empty category falls back to "notification" symbol lookup, which returns
	// empty if not configured (the hardcoded DefaultFallbackSymbol is used elsewhere)
	// This tests that the method doesn't panic on empty input
	_ = r.GetNerdSymbolForCategory("")
}

func TestResolverGetNerdSymbolForUrgency(t *testing.T) {
	// Use NewResolverWithAliases to get embedded default symbols
	r, err := NewResolverWithAliases()
	if err != nil {
		t.Fatalf("NewResolverWithAliases() error: %v", err)
	}

	// Test that urgency symbols are not empty (loaded from embedded config)
	if got := r.GetNerdSymbolForUrgency(model.UrgencyLow); got == "" {
		t.Error("GetNerdSymbolForUrgency(low) returned empty string")
	}
	if got := r.GetNerdSymbolForUrgency(model.UrgencyNormal); got == "" {
		t.Error("GetNerdSymbolForUrgency(normal) returned empty string")
	}
	if got := r.GetNerdSymbolForUrgency(model.UrgencyCritical); got == "" {
		t.Error("GetNerdSymbolForUrgency(critical) returned empty string")
	}

	// Test custom urgency symbol override
	customSymbol := "\U000f02fc" // nf-md-information
	r.AddSymbols(map[string]string{"urgency-normal": customSymbol})

	got := r.GetNerdSymbolForUrgency(model.UrgencyNormal)
	if got != customSymbol {
		t.Errorf("GetNerdSymbolForUrgency(urgency-normal) after AddSymbols = %q, want %q", got, customSymbol)
	}
}

func TestResolverFallbackSymbol(t *testing.T) {
	// Create resolver without loading any config
	r := NewResolver()

	// Without config, urgency should fall back to DefaultFallbackSymbol
	got := r.GetNerdSymbolForUrgency(model.UrgencyNormal)
	if got != DefaultFallbackSymbol {
		t.Errorf("GetNerdSymbolForUrgency without config = %q, want %q", got, DefaultFallbackSymbol)
	}

	// FallbackNerdSymbolForUrgency should also return the default
	got = FallbackNerdSymbolForUrgency(model.UrgencyCritical)
	if got != DefaultFallbackSymbol {
		t.Errorf("FallbackNerdSymbolForUrgency = %q, want %q", got, DefaultFallbackSymbol)
	}
}

func TestDefaultFallbackConstants(t *testing.T) {
	// Ensure constants are defined and non-empty
	if DefaultFallbackSymbol == "" {
		t.Error("DefaultFallbackSymbol is empty")
	}
	if DefaultFallbackGTKIcon == "" {
		t.Error("DefaultFallbackGTKIcon is empty")
	}

	// Verify expected values
	if DefaultFallbackSymbol != "󰵛" {
		t.Errorf("DefaultFallbackSymbol = %q, want 󰵛", DefaultFallbackSymbol)
	}
	if DefaultFallbackGTKIcon != "notification-symbolic" {
		t.Errorf("DefaultFallbackGTKIcon = %q, want notification-symbolic", DefaultFallbackGTKIcon)
	}
}

func TestGetEmbeddedAliasesStats(t *testing.T) {
	stats, err := GetEmbeddedAliasesStats()
	if err != nil {
		t.Fatalf("GetEmbeddedAliasesStats() error: %v", err)
	}

	// Should have aliases
	if stats.Aliases == 0 {
		t.Error("stats.Aliases is 0, expected some aliases")
	}

	// Should have apps
	if stats.Apps == 0 {
		t.Error("stats.Apps is 0, expected some apps")
	}

	// Should have categories
	if stats.Categories == 0 {
		t.Error("stats.Categories is 0, expected some categories")
	}

	t.Logf("Aliases stats: aliases=%d, apps=%d, categories=%d",
		stats.Aliases, stats.Apps, stats.Categories)
}
