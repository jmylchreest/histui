package db

// Counts holds basic notification counts for waybar status.
type Counts struct {
	Displayed int // Seen but not dismissed (pending)
	History   int // Dismissed
	Waiting   int // Not seen and not dismissed (missed)
}

// GetCounts returns basic notification counts.
func (d *DB) GetCounts() (*Counts, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var counts Counts
	err := d.conn.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN seen_at > 0 AND dismissed_at = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN dismissed_at > 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN seen_at = 0 AND dismissed_at = 0 THEN 1 ELSE 0 END), 0)
		FROM notifications
	`).Scan(&counts.Displayed, &counts.History, &counts.Waiting)

	return &counts, err
}

// AppCount holds count for a specific app.
type AppCount struct {
	AppName string
	Count   int
}

// DetailedCounts holds comprehensive notification statistics.
type DetailedCounts struct {
	// Pending: seen but not dismissed
	Pending         int
	PendingCritical int
	PendingNormal   int
	PendingLow      int

	// Missed: not seen and not dismissed
	Missed         int
	MissedCritical int
	MissedNormal   int
	MissedLow      int

	// Dismissed
	Dismissed int

	// Total tracked
	Total int

	// Top apps by notification count
	TopApps []AppCount
}

// GetDetailedCounts returns comprehensive notification statistics.
func (d *DB) GetDetailedCounts() (*DetailedCounts, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var counts DetailedCounts

	// Get all counts in a single query
	err := d.conn.QueryRow(`
		SELECT
			-- Pending (seen, not dismissed)
			COALESCE(SUM(CASE WHEN seen_at > 0 AND dismissed_at = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN seen_at > 0 AND dismissed_at = 0 AND urgency = 2 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN seen_at > 0 AND dismissed_at = 0 AND urgency = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN seen_at > 0 AND dismissed_at = 0 AND urgency = 0 THEN 1 ELSE 0 END), 0),
			-- Missed (not seen, not dismissed)
			COALESCE(SUM(CASE WHEN seen_at = 0 AND dismissed_at = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN seen_at = 0 AND dismissed_at = 0 AND urgency = 2 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN seen_at = 0 AND dismissed_at = 0 AND urgency = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN seen_at = 0 AND dismissed_at = 0 AND urgency = 0 THEN 1 ELSE 0 END), 0),
			-- Dismissed
			COALESCE(SUM(CASE WHEN dismissed_at > 0 THEN 1 ELSE 0 END), 0),
			-- Total
			COUNT(*)
		FROM notifications
	`).Scan(
		&counts.Pending, &counts.PendingCritical, &counts.PendingNormal, &counts.PendingLow,
		&counts.Missed, &counts.MissedCritical, &counts.MissedNormal, &counts.MissedLow,
		&counts.Dismissed, &counts.Total,
	)
	if err != nil {
		return nil, err
	}

	// Get top 5 apps
	counts.TopApps, err = d.getTopApps(5)
	if err != nil {
		// Non-fatal, just return what we have
		counts.TopApps = nil
	}

	return &counts, nil
}

// getTopApps returns the top N apps by notification count.
func (d *DB) getTopApps(limit int) ([]AppCount, error) {
	rows, err := d.conn.Query(`
		SELECT app_name, COUNT(*) as count
		FROM notifications
		GROUP BY app_name
		ORDER BY count DESC, app_name ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var apps []AppCount
	for rows.Next() {
		var app AppCount
		if err := rows.Scan(&app.AppName, &app.Count); err != nil {
			return apps, err
		}
		apps = append(apps, app)
	}

	return apps, rows.Err()
}

// GetHighestActiveUrgency returns the highest urgency among active (non-dismissed) notifications.
func (d *DB) GetHighestActiveUrgency() (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var urgency int
	err := d.conn.QueryRow(`
		SELECT COALESCE(MAX(urgency), -1)
		FROM notifications
		WHERE dismissed_at = 0
	`).Scan(&urgency)

	return urgency, err
}

// GetActiveCount returns count of non-dismissed notifications.
func (d *DB) GetActiveCount() (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.conn.QueryRow(`
		SELECT COUNT(*) FROM notifications WHERE dismissed_at = 0
	`).Scan(&count)

	return count, err
}

// GetCountsByUrgency returns counts grouped by urgency level.
type UrgencyCounts struct {
	Low      int
	Normal   int
	Critical int
}

func (d *DB) GetCountsByUrgency() (*UrgencyCounts, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var counts UrgencyCounts
	err := d.conn.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN urgency = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN urgency = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN urgency = 2 THEN 1 ELSE 0 END), 0)
		FROM notifications
		WHERE dismissed_at = 0
	`).Scan(&counts.Low, &counts.Normal, &counts.Critical)

	return &counts, err
}

// GetAppNames returns all unique app names.
func (d *DB) GetAppNames() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`
		SELECT DISTINCT app_name FROM notifications ORDER BY app_name
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var apps []string
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return apps, err
		}
		apps = append(apps, app)
	}

	return apps, rows.Err()
}

// SearchNotifications searches notifications by text in summary or body.
func (d *DB) SearchNotifications(query string, limit int) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	searchQuery := "%" + query + "%"
	rows, err := d.conn.Query(`
		SELECT histui_id FROM notifications
		WHERE summary LIKE ? OR body LIKE ? OR app_name LIKE ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, searchQuery, searchQuery, searchQuery, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return ids, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// ExistsWithHash checks if a notification with the given content hash exists.
func (d *DB) ExistsWithHash(contentHash string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.conn.QueryRow(`
		SELECT COUNT(*) FROM notifications WHERE content_hash = ?
	`, contentHash).Scan(&count)

	return count > 0, err
}
