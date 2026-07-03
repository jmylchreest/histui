// Package types provides shared custom types for configuration parsing.
// These types support human-readable string formats via encoding.TextUnmarshaler.
// This package is CGO-free and can be used by both histui and histuid.
package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// Duration is a time.Duration that can be unmarshaled from human-readable strings.
// Supports formats like "5s", "10s", "1m", "1h30m", or integer milliseconds for backwards compatibility.
//
// Special values for timeout configuration:
//   - "0" or "0s" = honor the client's requested timeout
//   - "-1", "-1s", or "never" = notification never expires
//   - positive value (e.g., "10s") = override with this timeout
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler for TOML parsing.
func (d *Duration) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))

	// Handle "never" as alias for -1 (never expire)
	if strings.EqualFold(s, "never") {
		*d = Duration(-1 * time.Millisecond)
		return nil
	}

	// Try parsing as duration string first (e.g., "5s", "1m", "1h30m", "-1s")
	if dur, err := time.ParseDuration(s); err == nil {
		*d = Duration(dur)
		return nil
	}

	// Try parsing as integer (milliseconds) for backwards compatibility
	var ms int64
	if _, err := fmt.Sscanf(s, "%d", &ms); err == nil {
		*d = Duration(time.Duration(ms) * time.Millisecond)
		return nil
	}

	return fmt.Errorf("invalid duration %q: must be like '5s', '1m', '1h30m', 'never', or milliseconds", s)
}

// MarshalText implements encoding.TextMarshaler for TOML output.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Milliseconds returns the duration in milliseconds.
func (d Duration) Milliseconds() int {
	return int(time.Duration(d).Milliseconds())
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// ByteSize represents a byte size that can be unmarshaled from human-readable strings.
// Uses github.com/dustin/go-humanize for parsing, supporting formats like:
//   - "100 KB", "1 MB", "1.5 GB" (SI units, base 1000)
//   - "100 KiB", "1 MiB" (IEC units, base 1024)
//   - Plain integers (bytes)
//
// Special values for image_data_preview_size configuration:
//   - "-1" or "never" = never show image-data in body
//   - "0" or "always" = always show image-data in body
//   - positive value (e.g., "100KB") = minimum size threshold
type ByteSize int64

// Byte size constants - IEC binary units (base 1024)
const (
	KiB ByteSize = 1024
	MiB ByteSize = 1024 * KiB
	GiB ByteSize = 1024 * MiB
	TiB ByteSize = 1024 * GiB
)

// Byte size constants - SI decimal units (base 1000)
const (
	KB ByteSize = 1000
	MB ByteSize = 1000 * KB
	GB ByteSize = 1000 * MB
	TB ByteSize = 1000 * GB
)

// UnmarshalText implements encoding.TextUnmarshaler for TOML parsing.
func (b *ByteSize) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))

	// Handle special aliases
	if strings.EqualFold(s, "never") || s == "-1" {
		*b = -1
		return nil
	}
	if strings.EqualFold(s, "always") || s == "0" {
		*b = 0
		return nil
	}

	// Use go-humanize to parse the byte size
	// Supports: "100 KB", "1 MB", "100 KiB", "1 MiB", etc.
	bytes, err := humanize.ParseBytes(s)
	if err != nil {
		return fmt.Errorf("invalid byte size %q: %w", s, err)
	}

	*b = ByteSize(bytes)
	return nil
}

// MarshalText implements encoding.TextMarshaler for TOML output.
func (b ByteSize) MarshalText() ([]byte, error) {
	if b < 0 {
		return []byte("never"), nil
	}
	if b == 0 {
		return []byte("always"), nil
	}
	// Use IEC units (KiB, MiB, GiB) for output since we use base 1024
	return []byte(humanize.IBytes(uint64(b))), nil
}

// Bytes returns the size in bytes.
func (b ByteSize) Bytes() int64 {
	return int64(b)
}

// IsNever returns true if image-data should never be shown.
func (b ByteSize) IsNever() bool {
	return b < 0
}

// IsAlways returns true if image-data should always be shown.
func (b ByteSize) IsAlways() bool {
	return b == 0
}

// ShouldShow returns true if data of the given size should be shown.
func (b ByteSize) ShouldShow(dataSize int64) bool {
	if b < 0 {
		return false // never
	}
	if b == 0 {
		return true // always
	}
	return dataSize >= int64(b) // size threshold
}

// MonitorSelector chooses which output notifications are shown on. It accepts
// either a 1-based index or a connector name such as "DP-1" in the same config
// value. The zero value means "auto" (let the compositor choose, usually the
// focused output).
//
// Config examples:
//   - monitor = 0        -> auto
//   - monitor = 1        -> first output by index (1-based)
//   - monitor = "DP-1"   -> output whose connector (or description) matches
type MonitorSelector struct {
	Index int    // 1-based; 0 = auto. Ignored when Name is set.
	Name  string // connector/description name; empty = select by Index.
}

// IsAuto reports whether no specific output was requested.
func (m MonitorSelector) IsAuto() bool {
	return m.Name == "" && m.Index <= 0
}

// UnmarshalText implements encoding.TextUnmarshaler. A value that parses as an
// integer is treated as a 1-based index (0 = auto); anything else is treated as
// a connector name.
func (m *MonitorSelector) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	*m = MonitorSelector{}
	if s == "" {
		return nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 {
			return fmt.Errorf("invalid monitor %q: index must be 0 (auto) or positive", s)
		}
		m.Index = n
		return nil
	}
	m.Name = s
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (m MonitorSelector) MarshalText() ([]byte, error) {
	if m.Name != "" {
		return []byte(m.Name), nil
	}
	return []byte(strconv.Itoa(m.Index)), nil
}

// String returns a human-readable form for logging.
func (m MonitorSelector) String() string {
	if m.Name != "" {
		return m.Name
	}
	if m.Index <= 0 {
		return "auto"
	}
	return strconv.Itoa(m.Index)
}
