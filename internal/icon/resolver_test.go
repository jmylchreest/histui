package icon

import "testing"

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
		{"zapzap", "whatsapp"},
		{"ZapZap", "whatsapp"},       // case insensitive
		{"  zapzap  ", "whatsapp"},   // trims spaces
		{"telegram-desktop", "telegram"},
		{"firefox-esr", "firefox"},
		{"firefox", "firefox"},       // no alias, returns as-is
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
	r.AddAlias("zapzap", "custom-whatsapp")

	if got := r.Resolve("zapzap"); got != "custom-whatsapp" {
		t.Errorf("Resolve(zapzap) = %q, want custom-whatsapp", got)
	}

	// Add new custom alias
	r.AddAlias("myapp", "myicon")
	if got := r.Resolve("myapp"); got != "myicon" {
		t.Errorf("Resolve(myapp) = %q, want myicon", got)
	}
}

func TestResolverNerdSymbols(t *testing.T) {
	r := NewResolver()

	// Known symbol
	if got := r.GetNerdSymbol("discord"); got == "" {
		t.Error("GetNerdSymbol(discord) returned empty string")
	}

	// Unknown symbol
	if got := r.GetNerdSymbol("unknown"); got != "" {
		t.Errorf("GetNerdSymbol(unknown) = %q, want empty string", got)
	}

	// Category lookup
	if got := r.GetNerdSymbolForCategory("email.arrived"); got == "" {
		t.Error("GetNerdSymbolForCategory(email.arrived) returned empty string")
	}

	// Empty category returns default notification symbol
	if got := r.GetNerdSymbolForCategory(""); got == "" {
		t.Error("GetNerdSymbolForCategory('') returned empty string")
	}
}

func TestListAliases(t *testing.T) {
	// Use NewResolverWithAliases to get embedded default aliases
	r, err := NewResolverWithAliases()
	if err != nil {
		t.Fatalf("NewResolverWithAliases() error: %v", err)
	}
	r.AddAlias("custom", "icon")

	aliases := r.ListAliases()
	if len(aliases) == 0 {
		t.Error("ListAliases returned empty map")
	}

	if aliases["custom"] != "icon" {
		t.Error("ListAliases missing custom alias")
	}

	if aliases["zapzap"] != "whatsapp" {
		t.Error("ListAliases missing default alias")
	}
}
