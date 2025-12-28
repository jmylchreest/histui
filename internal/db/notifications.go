package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmylchreest/histui/internal/model"
)

// AddNotification inserts a notification. Ignores if content_hash already exists.
func (d *DB) AddNotification(n *model.Notification) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure content hash exists
	n.EnsureContentHash()

	// Set imported timestamp if not set
	if n.HistuiImportedAt == 0 {
		n.HistuiImportedAt = time.Now().Unix()
	}

	extensions, err := json.Marshal(n.Extensions)
	if err != nil {
		extensions = []byte("{}")
	}

	hints, err := json.Marshal(n.OriginalHints)
	if err != nil {
		hints = []byte("{}")
	}

	_, err = d.conn.Exec(`
		INSERT OR IGNORE INTO notifications (
			histui_id, dbus_id, app_name, summary, body, icon_path,
			urgency, category, timestamp, expire_timeout,
			source, imported_at, seen_at, acted_at, dismissed_at,
			replayed, replayed_at, content_hash, extensions, original_hints
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		n.HistuiID, n.ID, n.AppName, n.Summary, n.Body, n.IconPath,
		n.Urgency, n.Category, n.Timestamp, n.ExpireTimeout,
		n.HistuiSource, n.HistuiImportedAt, n.HistuiSeenAt, n.HistuiActedAt, n.HistuiDismissedAt,
		boolToInt(n.HistuiReplayed), n.HistuiReplayedAt, n.ContentHash, string(extensions), string(hints),
	)

	return err
}

// AddBatch inserts multiple notifications efficiently.
func (d *DB) AddBatch(ns []model.Notification) error {
	if len(ns) == 0 {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO notifications (
			histui_id, dbus_id, app_name, summary, body, icon_path,
			urgency, category, timestamp, expire_timeout,
			source, imported_at, seen_at, acted_at, dismissed_at,
			replayed, replayed_at, content_hash, extensions, original_hints
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()

	for i := range ns {
		n := &ns[i]
		n.EnsureContentHash()

		if n.HistuiImportedAt == 0 {
			n.HistuiImportedAt = now
		}

		extensions, _ := json.Marshal(n.Extensions)
		hints, _ := json.Marshal(n.OriginalHints)

		_, err = stmt.Exec(
			n.HistuiID, n.ID, n.AppName, n.Summary, n.Body, n.IconPath,
			n.Urgency, n.Category, n.Timestamp, n.ExpireTimeout,
			n.HistuiSource, n.HistuiImportedAt, n.HistuiSeenAt, n.HistuiActedAt, n.HistuiDismissedAt,
			boolToInt(n.HistuiReplayed), n.HistuiReplayedAt, n.ContentHash, string(extensions), string(hints),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetNotification retrieves a notification by ID.
func (d *DB) GetNotification(histuiID string) (*model.Notification, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	row := d.conn.QueryRow(`
		SELECT histui_id, dbus_id, app_name, summary, body, icon_path,
			urgency, category, timestamp, expire_timeout,
			source, imported_at, seen_at, acted_at, dismissed_at,
			replayed, replayed_at, content_hash, extensions, original_hints
		FROM notifications WHERE histui_id = ?
	`, histuiID)

	return scanNotification(row)
}

// UpdateNotification updates an existing notification.
func (d *DB) UpdateNotification(n *model.Notification) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	extensions, _ := json.Marshal(n.Extensions)
	hints, _ := json.Marshal(n.OriginalHints)

	_, err := d.conn.Exec(`
		UPDATE notifications SET
			dbus_id = ?, app_name = ?, summary = ?, body = ?, icon_path = ?,
			urgency = ?, category = ?, timestamp = ?, expire_timeout = ?,
			source = ?, seen_at = ?, acted_at = ?, dismissed_at = ?,
			replayed = ?, replayed_at = ?, extensions = ?, original_hints = ?
		WHERE histui_id = ?
	`,
		n.ID, n.AppName, n.Summary, n.Body, n.IconPath,
		n.Urgency, n.Category, n.Timestamp, n.ExpireTimeout,
		n.HistuiSource, n.HistuiSeenAt, n.HistuiActedAt, n.HistuiDismissedAt,
		boolToInt(n.HistuiReplayed), n.HistuiReplayedAt, string(extensions), string(hints),
		n.HistuiID,
	)

	return err
}

// DeleteNotification removes a notification (cascade deletes images).
func (d *DB) DeleteNotification(histuiID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("DELETE FROM notifications WHERE histui_id = ?", histuiID)
	return err
}

// DismissNotification marks a notification as dismissed.
func (d *DB) DismissNotification(histuiID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(
		"UPDATE notifications SET dismissed_at = ? WHERE histui_id = ?",
		time.Now().Unix(), histuiID,
	)
	return err
}

// DismissBatch marks multiple notifications as dismissed.
func (d *DB) DismissBatch(histuiIDs []string) error {
	if len(histuiIDs) == 0 {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	stmt, err := tx.Prepare("UPDATE notifications SET dismissed_at = ? WHERE histui_id = ? AND dismissed_at = 0")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, id := range histuiIDs {
		if _, err := stmt.Exec(now, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DismissAll marks all non-dismissed notifications as dismissed.
// Returns the number of notifications dismissed.
func (d *DB) DismissAll() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.conn.Exec(
		"UPDATE notifications SET dismissed_at = ? WHERE dismissed_at = 0",
		time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// MarkSeen marks a notification as seen.
func (d *DB) MarkSeen(histuiID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(
		"UPDATE notifications SET seen_at = ? WHERE histui_id = ? AND seen_at = 0",
		time.Now().Unix(), histuiID,
	)
	return err
}

// MarkReplayed marks a notification as replayed.
func (d *DB) MarkReplayed(histuiID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().Unix()
	_, err := d.conn.Exec(
		"UPDATE notifications SET replayed = 1, replayed_at = ? WHERE histui_id = ?",
		now, histuiID,
	)
	return err
}

// FilterOptions specifies criteria for filtering notifications.
type FilterOptions struct {
	Since       int64  // Only notifications after this Unix timestamp
	AppName     string // Exact match on app name
	Urgency     *int   // Filter by urgency level (nil=any)
	Dismissed   *bool  // Filter by dismissed state (nil=any)
	Seen        *bool  // Filter by seen state (nil=any)
	Limit       int    // Maximum results (0=unlimited)
	Offset      int    // Offset for pagination
	OrderBy     string // Field to sort by (default: timestamp)
	OrderDesc   bool   // Sort descending (default: true)
}

// Filter returns notifications matching criteria.
func (d *DB) Filter(opts FilterOptions) ([]model.Notification, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
		SELECT histui_id, dbus_id, app_name, summary, body, icon_path,
			urgency, category, timestamp, expire_timeout,
			source, imported_at, seen_at, acted_at, dismissed_at,
			replayed, replayed_at, content_hash, extensions, original_hints
		FROM notifications WHERE 1=1
	`
	args := []any{}

	if opts.Since > 0 {
		query += " AND timestamp >= ?"
		args = append(args, opts.Since)
	}

	if opts.AppName != "" {
		query += " AND app_name = ?"
		args = append(args, opts.AppName)
	}

	if opts.Urgency != nil {
		query += " AND urgency = ?"
		args = append(args, *opts.Urgency)
	}

	if opts.Dismissed != nil {
		if *opts.Dismissed {
			query += " AND dismissed_at > 0"
		} else {
			query += " AND dismissed_at = 0"
		}
	}

	if opts.Seen != nil {
		if *opts.Seen {
			query += " AND seen_at > 0"
		} else {
			query += " AND seen_at = 0"
		}
	}

	// Order
	orderBy := "timestamp"
	if opts.OrderBy != "" {
		orderBy = opts.OrderBy
	}
	order := "DESC"
	if !opts.OrderDesc {
		order = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, order)

	// Limit and offset
	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanNotifications(rows)
}

// All returns all notifications ordered by timestamp descending.
func (d *DB) All() ([]model.Notification, error) {
	return d.Filter(FilterOptions{OrderDesc: true})
}

// Count returns total notification count.
func (d *DB) Count() (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count)
	return count, err
}

// Prune removes oldest notifications beyond limit, returns count removed.
func (d *DB) Prune(keepCount int) (int, error) {
	if keepCount <= 0 {
		return 0, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Delete notifications beyond the keep limit (oldest first)
	result, err := d.conn.Exec(`
		DELETE FROM notifications WHERE histui_id IN (
			SELECT histui_id FROM notifications
			ORDER BY timestamp DESC
			LIMIT -1 OFFSET ?
		)
	`, keepCount)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// PruneOlderThan removes notifications older than the given timestamp.
func (d *DB) PruneOlderThan(before int64) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.conn.Exec("DELETE FROM notifications WHERE timestamp < ?", before)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// Clear removes all notifications.
func (d *DB) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("DELETE FROM notifications")
	return err
}

// scanNotification scans a single notification from a row.
func scanNotification(row *sql.Row) (*model.Notification, error) {
	var n model.Notification
	var replayed int
	var extensions, hints string

	err := row.Scan(
		&n.HistuiID, &n.ID, &n.AppName, &n.Summary, &n.Body, &n.IconPath,
		&n.Urgency, &n.Category, &n.Timestamp, &n.ExpireTimeout,
		&n.HistuiSource, &n.HistuiImportedAt, &n.HistuiSeenAt, &n.HistuiActedAt, &n.HistuiDismissedAt,
		&replayed, &n.HistuiReplayedAt, &n.ContentHash, &extensions, &hints,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	n.HistuiReplayed = replayed != 0
	n.UrgencyName = model.UrgencyNames[n.Urgency]

	// Parse JSON fields
	if extensions != "" && extensions != "{}" {
		_ = json.Unmarshal([]byte(extensions), &n.Extensions)
	}
	if hints != "" && hints != "{}" {
		_ = json.Unmarshal([]byte(hints), &n.OriginalHints)
	}

	return &n, nil
}

// scanNotifications scans multiple notifications from rows.
func scanNotifications(rows *sql.Rows) ([]model.Notification, error) {
	var notifications []model.Notification

	for rows.Next() {
		var n model.Notification
		var replayed int
		var extensions, hints string

		err := rows.Scan(
			&n.HistuiID, &n.ID, &n.AppName, &n.Summary, &n.Body, &n.IconPath,
			&n.Urgency, &n.Category, &n.Timestamp, &n.ExpireTimeout,
			&n.HistuiSource, &n.HistuiImportedAt, &n.HistuiSeenAt, &n.HistuiActedAt, &n.HistuiDismissedAt,
			&replayed, &n.HistuiReplayedAt, &n.ContentHash, &extensions, &hints,
		)
		if err != nil {
			return notifications, err
		}

		n.HistuiReplayed = replayed != 0
		n.UrgencyName = model.UrgencyNames[n.Urgency]

		if extensions != "" && extensions != "{}" {
			_ = json.Unmarshal([]byte(extensions), &n.Extensions)
		}
		if hints != "" && hints != "{}" {
			_ = json.Unmarshal([]byte(hints), &n.OriginalHints)
		}

		notifications = append(notifications, n)
	}

	return notifications, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
