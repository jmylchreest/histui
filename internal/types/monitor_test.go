package types

import "testing"

func TestMonitorSelector_UnmarshalText(t *testing.T) {
	cases := []struct {
		in       string
		wantIdx  int
		wantName string
		wantErr  bool
	}{
		{"", 0, "", false},
		{"0", 0, "", false},
		{"1", 1, "", false},
		{"3", 3, "", false},
		{"DP-1", 0, "DP-1", false},
		{"HDMI-A-1", 0, "HDMI-A-1", false},
		{" DP-2 ", 0, "DP-2", false},
		{"-1", 0, "", true},
	}
	for _, c := range cases {
		var m MonitorSelector
		err := m.UnmarshalText([]byte(c.in))
		if (err != nil) != c.wantErr {
			t.Fatalf("%q: err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if c.wantErr {
			continue
		}
		if m.Index != c.wantIdx || m.Name != c.wantName {
			t.Errorf("%q: got {Index:%d Name:%q}, want {Index:%d Name:%q}", c.in, m.Index, m.Name, c.wantIdx, c.wantName)
		}
	}
}

func TestMonitorSelector_IsAutoAndString(t *testing.T) {
	if !(MonitorSelector{}).IsAuto() {
		t.Error("zero value should be auto")
	}
	if (MonitorSelector{Index: 1}).IsAuto() || (MonitorSelector{Name: "DP-1"}).IsAuto() {
		t.Error("explicit selectors should not be auto")
	}
	if got := (MonitorSelector{}).String(); got != "auto" {
		t.Errorf("String() = %q, want auto", got)
	}
	if got := (MonitorSelector{Index: 2}).String(); got != "2" {
		t.Errorf("String() = %q, want 2", got)
	}
	if got := (MonitorSelector{Name: "DP-1"}).String(); got != "DP-1" {
		t.Errorf("String() = %q, want DP-1", got)
	}
}
