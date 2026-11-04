package display

import "testing"

func TestNullMonitor(t *testing.T) {
	if nullMonitor() != nil {
		t.Fatal("auto monitor must be a native NULL")
	}
}
