package db

import (
	"database/sql"
	"time"
)

// Image reference types
const (
	ImageRefIcon  = "icon"
	ImageRefImage = "image"
)

// SaveImage stores image data for a notification.
func (d *DB) SaveImage(histuiID string, refType string, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(`
		INSERT OR REPLACE INTO notification_images (histui_id, ref_type, data, created_at)
		VALUES (?, ?, ?, ?)
	`, histuiID, refType, data, time.Now().Unix())

	return err
}

// LoadImage retrieves image data for a notification.
func (d *DB) LoadImage(histuiID string, refType string) ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var data []byte
	err := d.conn.QueryRow(`
		SELECT data FROM notification_images WHERE histui_id = ? AND ref_type = ?
	`, histuiID, refType).Scan(&data)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return data, err
}

// LoadAllImages retrieves all images for a notification.
func (d *DB) LoadAllImages(histuiID string) (map[string][]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`
		SELECT ref_type, data FROM notification_images WHERE histui_id = ?
	`, histuiID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	images := make(map[string][]byte)
	for rows.Next() {
		var refType string
		var data []byte
		if err := rows.Scan(&refType, &data); err != nil {
			return images, err
		}
		images[refType] = data
	}

	return images, rows.Err()
}

// HasImage checks if a notification has an image of the given type.
func (d *DB) HasImage(histuiID string, refType string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.conn.QueryRow(`
		SELECT COUNT(*) FROM notification_images WHERE histui_id = ? AND ref_type = ?
	`, histuiID, refType).Scan(&count)

	return count > 0, err
}

// DeleteImage removes a specific image.
func (d *DB) DeleteImage(histuiID string, refType string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(`
		DELETE FROM notification_images WHERE histui_id = ? AND ref_type = ?
	`, histuiID, refType)
	return err
}

// DeleteAllImages removes all images for a notification.
// Note: This is usually not needed as CASCADE delete handles it.
func (d *DB) DeleteAllImages(histuiID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(`
		DELETE FROM notification_images WHERE histui_id = ?
	`, histuiID)
	return err
}

// GetImageRefs returns the ref types that exist for a notification.
func (d *DB) GetImageRefs(histuiID string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.conn.Query(`
		SELECT ref_type FROM notification_images WHERE histui_id = ?
	`, histuiID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return refs, err
		}
		refs = append(refs, ref)
	}

	return refs, rows.Err()
}

// ImageStats returns statistics about stored images.
type ImageStats struct {
	TotalImages int
	TotalBytes  int64
	IconCount   int
	ImageCount  int
}

// GetImageStats returns image storage statistics.
func (d *DB) GetImageStats() (*ImageStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var stats ImageStats

	err := d.conn.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(LENGTH(data)), 0),
			COALESCE(SUM(CASE WHEN ref_type = 'icon' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ref_type = 'image' THEN 1 ELSE 0 END), 0)
		FROM notification_images
	`).Scan(&stats.TotalImages, &stats.TotalBytes, &stats.IconCount, &stats.ImageCount)

	return &stats, err
}
