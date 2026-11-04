package db

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmylchreest/histui/internal/model"
)

func testNotification(id string) *model.Notification {
	return &model.Notification{HistuiID: id, Summary: id, AppName: "test", Timestamp: 100,
		HistuiSource: "test", Extensions: &model.Extensions{StackTag: "stack"}}
}

// The subprocess is intentional: Go's race detector cannot detect ordering or
// SQLite lock conflicts between a daemon and an independently running TUI.
func TestWriterProcess(t *testing.T) {
	path := os.Getenv("HISTUI_TEST_WRITER_DB")
	if path == "" {
		return
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		switch s.Text() {
		case "row":
			err = d.AddNotification(testNotification("late-image"))
		case "image":
			err = d.SaveImage("late-image", ImageRefImage, []byte("image-content"))
		case "burst":
			fmt.Println("started")
			for i := 0; i < 100 && err == nil; i++ {
				err = d.AddNotification(testNotification(fmt.Sprintf("burst-%03d", i)))
			}
		default:
			t.Fatalf("unknown writer command %q", s.Text())
		}
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("done")
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
}

type testWriter struct {
	in  io.WriteCloser
	out *bufio.Scanner
}

func startWriter(t *testing.T, path string) *testWriter {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWriterProcess$")
	cmd.Env = append(os.Environ(), "HISTUI_TEST_WRITER_DB="+path)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	w := &testWriter{in: in, out: bufio.NewScanner(out)}
	t.Cleanup(func() {
		in.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("writer: %v; %s", err, stderr.String())
		}
		cancel()
	})
	return w
}

func (w *testWriter) command(t *testing.T, command string) {
	t.Helper()
	if _, err := fmt.Fprintln(w.in, command); err != nil {
		t.Fatal(err)
	}
	w.expect(t, "done")
}

func (w *testWriter) expect(t *testing.T, value string) {
	t.Helper()
	if !w.out.Scan() || w.out.Text() != value {
		t.Fatalf("writer reply %q, want %q: %v", w.out.Text(), value, w.out.Err())
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	return d
}

func TestSQLiteEnvironment(t *testing.T) {
	d := openTestDB(t)
	var version, journal string
	var busy, foreignKeys int
	for query, dest := range map[string]any{
		"SELECT sqlite_version()": &version,
		"PRAGMA journal_mode":     &journal,
		"PRAGMA busy_timeout":     &busy,
		"PRAGMA foreign_keys":     &foreignKeys,
	} {
		if err := d.conn.QueryRow(query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("SQLite=%s journal=%s busy_timeout=%d foreign_keys=%d", version, journal, busy, foreignKeys)
	if journal != "wal" || busy != 30000 || foreignKeys != 1 {
		t.Fatal("database configuration differs from expected contract")
	}
}

func TestCrossProcessDelayedImageAndConcurrentReads(t *testing.T) {
	d := openTestDB(t)
	w := startWriter(t, d.Path())
	w.command(t, "row")
	if n, err := d.GetNotification("late-image"); err != nil || n == nil {
		t.Fatalf("notification not visible: %v", err)
	}
	if image, err := d.LoadImage("late-image", ImageRefImage); err != nil || image != nil {
		t.Fatalf("premature image: %q %v", image, err)
	}
	w.command(t, "image")
	if image, err := d.LoadImage("late-image", ImageRefImage); err != nil || string(image) != "image-content" {
		t.Fatalf("late image not visible: %q %v", image, err)
	}
	if _, err := fmt.Fprintln(w.in, "burst"); err != nil {
		t.Fatal(err)
	}
	w.expect(t, "started")
	// Reads run while the other process commits. All aggregates in this one
	// statement must agree, even when consecutive calls see different versions.
	for i := 0; i < 100; i++ {
		c, err := d.GetDetailedCounts()
		if err != nil {
			t.Fatal(err)
		}
		if c.Total != c.Pending+c.Missed+c.Dismissed {
			t.Fatalf("inconsistent aggregate: %+v", c)
		}
		if _, err := d.Filter(FilterOptions{Limit: 20, OrderDesc: true}); err != nil {
			t.Fatal(err)
		}
	}
	w.expect(t, "done")
	if n, err := d.Count(); err != nil || n != 101 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if counts, err := d.GetStackTagCounts(); err != nil || counts["stack"] != 101 {
		t.Fatalf("stack counts=%v err=%v", counts, err)
	}
	var integrity string
	if err := d.conn.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q err=%v", integrity, err)
	}
}

func TestCrossProcessReadSnapshot(t *testing.T) {
	d := openTestDB(t)
	w := startWriter(t, d.Path())
	tx, err := d.conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count); err != nil || count != 0 {
		t.Fatalf("initial count=%d err=%v", count, err)
	}
	w.command(t, "row")
	if err := tx.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count); err != nil || count != 0 {
		t.Fatalf("snapshot changed: count=%d err=%v", count, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if count, err = d.Count(); err != nil || count != 1 {
		t.Fatalf("new snapshot count=%d err=%v", count, err)
	}
}

func TestWALCheckpointWithActiveReader(t *testing.T) {
	d := openTestDB(t)
	reader, err := Open(d.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := d.AddNotification(testNotification("wal")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec("PRAGMA busy_timeout=10"); err != nil {
		t.Fatal(err)
	}
	var busy, log, checkpointed int
	if err := d.conn.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &checkpointed); err != nil {
		t.Fatal(err)
	}
	if busy != 1 {
		t.Fatalf("expected reader to prevent truncate, got busy=%d log=%d checkpointed=%d", busy, log, checkpointed)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := d.conn.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &checkpointed); err != nil {
		t.Fatal(err)
	}
	if busy != 0 || log != 0 {
		t.Fatalf("truncate failed after reader release: busy=%d log=%d", busy, log)
	}
}
