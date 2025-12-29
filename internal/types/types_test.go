package types

import (
	"testing"
	"time"
)

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"seconds", "5s", 5 * time.Second, false},
		{"minutes", "2m", 2 * time.Minute, false},
		{"hours", "1h", time.Hour, false},
		{"complex", "1h30m", 90 * time.Minute, false},
		{"zero", "0", 0, false},
		{"milliseconds_int", "5000", 5 * time.Second, false},
		{"ms_suffix", "500ms", 500 * time.Millisecond, false},
		{"never", "never", -1 * time.Millisecond, false},
		{"never_upper", "NEVER", -1 * time.Millisecond, false},
		{"negative", "-1s", -1 * time.Second, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && d.Duration() != tt.want {
				t.Errorf("UnmarshalText() = %v, want %v", d.Duration(), tt.want)
			}
		})
	}
}

func TestDuration_MarshalText(t *testing.T) {
	tests := []struct {
		name string
		d    Duration
		want string
	}{
		{"5_seconds", Duration(5 * time.Second), "5s"},
		{"2_minutes", Duration(2 * time.Minute), "2m0s"},
		{"zero", Duration(0), "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.d.MarshalText()
			if err != nil {
				t.Errorf("MarshalText() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("MarshalText() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestByteSize_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ByteSize
		wantErr bool
	}{
		{"bytes", "100", 100, false},
		{"kilobytes", "100 KB", 100 * 1000, false},
		{"kibibytes", "100 KiB", 100 * 1024, false},
		{"megabytes", "1 MB", 1000 * 1000, false},
		{"mebibytes", "1 MiB", 1024 * 1024, false},
		{"never", "never", -1, false},
		{"never_upper", "NEVER", -1, false},
		{"minus_one", "-1", -1, false},
		{"always", "always", 0, false},
		{"always_upper", "ALWAYS", 0, false},
		{"zero", "0", 0, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b ByteSize
			err := b.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && b != tt.want {
				t.Errorf("UnmarshalText() = %v, want %v", b, tt.want)
			}
		})
	}
}

func TestByteSize_MarshalText(t *testing.T) {
	tests := []struct {
		name string
		b    ByteSize
		want string
	}{
		{"never", ByteSize(-1), "never"},
		{"always", ByteSize(0), "always"},
		{"100_bytes", ByteSize(100), "100 B"},
		{"1_kib", ByteSize(1024), "1.0 KiB"},
		{"1_mib", ByteSize(1024 * 1024), "1.0 MiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.b.MarshalText()
			if err != nil {
				t.Errorf("MarshalText() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("MarshalText() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestByteSize_ShouldShow(t *testing.T) {
	tests := []struct {
		name     string
		b        ByteSize
		dataSize int64
		want     bool
	}{
		{"never_any_size", ByteSize(-1), 1000000, false},
		{"always_any_size", ByteSize(0), 1, true},
		{"always_zero", ByteSize(0), 0, true},
		{"threshold_above", ByteSize(100 * KiB), 200 * int64(KiB), true},
		{"threshold_equal", ByteSize(100 * KiB), int64(100 * KiB), true},
		{"threshold_below", ByteSize(100 * KiB), 50 * int64(KiB), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.ShouldShow(tt.dataSize); got != tt.want {
				t.Errorf("ShouldShow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestByteSize_IsNever(t *testing.T) {
	if !ByteSize(-1).IsNever() {
		t.Error("IsNever() should return true for -1")
	}
	if ByteSize(0).IsNever() {
		t.Error("IsNever() should return false for 0")
	}
	if ByteSize(100).IsNever() {
		t.Error("IsNever() should return false for 100")
	}
}

func TestByteSize_IsAlways(t *testing.T) {
	if ByteSize(-1).IsAlways() {
		t.Error("IsAlways() should return false for -1")
	}
	if !ByteSize(0).IsAlways() {
		t.Error("IsAlways() should return true for 0")
	}
	if ByteSize(100).IsAlways() {
		t.Error("IsAlways() should return false for 100")
	}
}
