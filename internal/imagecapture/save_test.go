package imagecapture

import (
	"bytes"
	"errors"
	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/model"
	"path/filepath"
	"testing"
)

func TestNotifyAfterCommittedImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	writer, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reader, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	n, err := model.NewNotification("test")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.AddNotification(n); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	notified := make(chan struct{}, 1)
	done := make(chan error, 1)
	want := []byte("encoded image")
	go func() {
		done <- Save(func() []byte { close(started); <-release; return want }, func(data []byte) error { return writer.SaveImage(n.HistuiID, db.ImageRefImage, data) }, func() { notified <- struct{}{} })
	}()
	<-started
	data, err := reader.LoadImage(n.HistuiID, db.ImageRefImage)
	if err != nil || len(data) != 0 {
		t.Fatalf("before save: %q %v", data, err)
	}
	select {
	case <-notified:
		t.Fatal("notified before commit")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case <-notified:
	default:
		t.Fatal("missing completion notification")
	}
	data, err = reader.LoadImage(n.HistuiID, db.ImageRefImage)
	if err != nil || !bytes.Equal(data, want) {
		t.Fatalf("after notification: %q %v", data, err)
	}
}

func TestFailedOrEmptyImageDoesNotNotify(t *testing.T) {
	sentinel := errors.New("write failed")
	for _, empty := range []bool{false, true} {
		calls := 0
		err := Save(func() []byte {
			if empty {
				return nil
			}
			return []byte("data")
		}, func([]byte) error { calls++; return sentinel }, func() { t.Error("unexpected notification") })
		if empty {
			if err != nil || calls != 0 {
				t.Fatalf("empty encoding: %v calls=%d", err, calls)
			}
		} else if !errors.Is(err, sentinel) {
			t.Fatalf("lost error: %v", err)
		}
	}
}
