# histui Notification Schema Reference

This document defines the notification data model for histui's history store, based on the [freedesktop.org Desktop Notifications Specification 1.3](https://specifications.freedesktop.org/notification/latest/) and common daemon extensions.

## Notification Standards Landscape

The freedesktop.org Desktop Notifications Specification has been the **universal standard** for Linux desktop notifications since ~2006-2007 (originally from the Galago Project). All modern notification daemons implement this spec:

| Daemon | Compatibility | Notes |
|--------|---------------|-------|
| dunst | Freedesktop | + custom x-dunst-* hints |
| mako | Freedesktop | Wayland-native |
| swaync | Freedesktop | GTK-based notification center |
| KDE Plasma | Freedesktop | Via KNotify/D-Bus |
| GNOME | Freedesktop | Via notification-daemon |
| XFCE | Freedesktop | Via xfce4-notifyd |
| Notify-OSD | Freedesktop | + x-canonical-* hints (Ubuntu) |

**No legacy formats need to be supported** - all variations are vendor hint extensions on top of the same D-Bus protocol.

## Storage Format

History is stored as **JSONL** (JSON Lines) - one notification per line for efficient append operations and streaming reads.

```
~/.local/share/histui/history.jsonl
```

## Core Notification Schema

Each notification record contains:

```jsonc
{
  // === histui metadata (added by histui) ===
  "histui_id": "01ARZ3NDEKTSV4RRFFQ69G5FAV", // ULID - sortable unique ID (string)
  "histui_source": "dunst",            // Source adapter: "dunst", "stdin", "dbus", etc.
  "histui_imported_at": 1703577600,    // Unix timestamp when histui received this
  "histui_dedupe_hash": "sha256...",   // For deduplication across sources (optional)

  // === Freedesktop Standard Fields ===
  "id": 12345,                         // Original notification ID from daemon (uint32)
  "app_name": "firefox",               // Application name (string)
  "app_icon": "firefox",               // Icon name or path (string)
  "summary": "Download Complete",      // Brief title (string)
  "body": "myfile.zip has finished",   // Detailed text, may contain markup (string)
  "timestamp": 1703577500,             // Unix timestamp of notification (int64)
  "expire_timeout": 5000,              // Timeout in ms, -1=default, 0=never (int32)
  "replaces_id": 0,                    // ID of notification this replaces (uint32)

  // === Urgency ===
  "urgency": 1,                        // 0=low, 1=normal, 2=critical (byte)
  "urgency_name": "normal",            // Human-readable: "low", "normal", "critical"

  // === Category ===
  "category": "transfer.complete",     // Notification type (string, optional)

  // === Actions ===
  "actions": [                         // Action pairs: [id, label, id, label, ...]
    {"id": "default", "label": "Open"},
    {"id": "dismiss", "label": "Dismiss"}
  ],
  "default_action_name": "default",    // Default action ID (string)

  // === Hints (standard) ===
  "hints": {
    "desktop-entry": "firefox",        // .desktop file name (string)
    "transient": false,                // Bypass persistence (boolean)
    "resident": false,                 // Keep after action invoked (boolean)
    "action-icons": false,             // Interpret action IDs as icons (boolean)
    "suppress-sound": false,           // Don't play sound (boolean)
    "sound-file": "/path/to/sound.wav",// Sound file path (string)
    "sound-name": "message-new-email", // Themed sound name (string)
    "x": 100,                          // Screen X position (int32)
    "y": 50                            // Screen Y position (int32)
  },

  // === Image Data ===
  "icon_path": "/usr/share/icons/...", // Resolved icon file path (string)
  "image_path": "/tmp/notification.png", // Image hint path (string)
  "image_data": {                      // Raw image data (if captured)
    "width": 48,
    "height": 48,
    "rowstride": 192,
    "has_alpha": true,
    "bits_per_sample": 8,
    "channels": 4,
    "data_base64": "iVBORw0KGgo..."   // Base64-encoded pixel data
  },

  // === Daemon-Specific Extensions ===
  "extensions": {
    // Dunst-specific
    "stack_tag": "volume",             // x-dunst-stack-tag (string)
    "progress": 75,                    // Progress bar value 0-100, -1=none (int)
    "message": "<i>app</i>: summary",  // Dunst formatted message (string)
    "urls": "https://example.com",     // Extracted URLs (string)
    "foreground": "#ffffff",           // fgcolor hint (string)
    "background": "#000000",           // bgcolor hint (string)
    "frame_color": "#888888",          // frcolor hint (string)
    "highlight": "#ff0000",            // hlcolor hint (string)

    // Ubuntu/Canonical
    "synchronous": "volume",           // x-canonical-private-synchronous (string)

    // Generic vendor hints (x-vendor-name format)
    "vendor_hints": {
      "x-kde-something": "value"
    }
  },

  // === Close Event (if captured) ===
  "closed": {
    "at": 1703577505,                  // When closed (Unix timestamp)
    "reason": 2,                       // 1=expired, 2=dismissed, 3=API, 4=undefined
    "reason_name": "dismissed"         // Human-readable reason
  }
}
```

## Field Reference

### histui Metadata Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `histui_id` | string | Yes | [ULID](https://github.com/ulid/spec) - lexicographically sortable unique ID |
| `histui_source` | string | Yes | Input adapter identifier |
| `histui_imported_at` | int64 | Yes | Unix timestamp when imported |
| `histui_dedupe_hash` | string | No | SHA256 of (app_name + summary + body + timestamp) |

**Why ULID over UUID?**
- ULIDs are **lexicographically sortable by creation time** - perfect for time-series data like notifications
- 128-bit compatible with UUID (can be stored in UUID columns if needed)
- Encodes timestamp in first 48 bits - enables efficient range queries
- Format: `01ARZ3NDEKTSV4RRFFQ69G5FAV` (26 characters, Crockford Base32)

### Freedesktop Standard Fields

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `id` | uint32 | Yes | Notify() return | Daemon-assigned notification ID |
| `app_name` | string | Yes | Notify() param | Sending application name |
| `app_icon` | string | No | Notify() param | Icon name or file:// URI |
| `summary` | string | Yes | Notify() param | Brief notification title |
| `body` | string | No | Notify() param | Detailed text (may contain markup) |
| `timestamp` | int64 | Yes | Daemon | Unix timestamp (seconds or microseconds) |
| `expire_timeout` | int32 | No | Notify() param | -1=default, 0=never, >0=milliseconds |
| `replaces_id` | uint32 | No | Notify() param | ID of notification being replaced |

### Urgency

| Value | Name | Description |
|-------|------|-------------|
| 0 | low | Background information (e.g., "Joe signed on") |
| 1 | normal | Standard notifications (e.g., "New mail") |
| 2 | critical | Requires attention, should not auto-expire |

### Standard Categories

| Category | Description |
|----------|-------------|
| `device` | Generic device notification |
| `device.added` | Device connected (USB, etc.) |
| `device.error` | Device error |
| `device.removed` | Device disconnected |
| `email` | Generic email notification |
| `email.arrived` | New email received |
| `email.bounced` | Email bounced |
| `im` | Generic instant message |
| `im.error` | IM error |
| `im.received` | IM received |
| `network` | Generic network notification |
| `network.connected` | Network connected |
| `network.disconnected` | Network disconnected |
| `network.error` | Network error |
| `presence` | Generic presence change |
| `presence.offline` | Contact went offline |
| `presence.online` | Contact came online |
| `transfer` | Generic file transfer |
| `transfer.complete` | Transfer completed |
| `transfer.error` | Transfer failed |

### Standard Hints

| Hint | Type | Description |
|------|------|-------------|
| `action-icons` | boolean | Interpret action IDs as icon names |
| `category` | string | Notification category (see above) |
| `desktop-entry` | string | .desktop filename (without .desktop extension) |
| `image-data` | (iiibiiay) | Raw image data structure |
| `image-path` | string | Path or URI to image file |
| `resident` | boolean | Don't remove notification after action invoked |
| `sound-file` | string | Path to sound file |
| `sound-name` | string | Themed sound name from freedesktop.org spec |
| `suppress-sound` | boolean | Don't play any sound |
| `transient` | boolean | Bypass persistence, don't store in history |
| `urgency` | byte | Urgency level (0, 1, or 2) |
| `x` | int32 | X coordinate for positioning |
| `y` | int32 | Y coordinate for positioning |

### Image Data Structure (iiibiiay)

Raw image data from the `image-data` or `icon_data` hints:

| Field | Type | Description |
|-------|------|-------------|
| `width` | int32 | Image width in pixels |
| `height` | int32 | Image height in pixels |
| `rowstride` | int32 | Bytes per row (may include padding) |
| `has_alpha` | bool | True if alpha channel present |
| `bits_per_sample` | int32 | Always 8 |
| `channels` | int32 | 4 if alpha, otherwise 3 |
| `data` | byte[] | RGB(A) pixel data, row-major order |

**Note**: In JSONL storage, `data` is base64-encoded to preserve binary data.

### Dunst-Specific Extensions

From `dunstctl history` output and dunstify hints:

| Field | Type | Description |
|-------|------|-------------|
| `stack_tag` | string | Groups notifications for replacement (x-dunst-stack-tag) |
| `progress` | int | Progress bar value 0-100, -1 if none |
| `message` | string | Pre-formatted message with markup |
| `urls` | string | URLs extracted from notification |
| `foreground` | string | Text color (fgcolor hint) |
| `background` | string | Background color (bgcolor hint) |
| `frame_color` | string | Frame color (frcolor hint) |
| `highlight` | string | Highlight color (hlcolor hint) |

### Stack Tag Hints (Cross-Daemon)

Multiple hint names map to the same concept:

| Hint Name | Daemon | Description |
|-----------|--------|-------------|
| `x-dunst-stack-tag` | Dunst | Primary dunst stack tag |
| `x-canonical-private-synchronous` | Ubuntu/Notify-OSD | Ubuntu stack tag |
| `synchronous` | Various | Short form |
| `private-synchronous` | Various | Alternative form |

### Close Reasons

| Value | Name | Description |
|-------|------|-------------|
| 1 | expired | Notification timed out |
| 2 | dismissed | User dismissed it |
| 3 | closed | Closed via CloseNotification() API |
| 4 | undefined | Undefined/reserved reason |

## Adapter-Specific Mapping

### Dunst Adapter

Maps `dunstctl history` JSON to schema:

| Dunst Field | Schema Field |
|-------------|--------------|
| `id.data` | `id` |
| `appname.data` | `app_name` |
| `summary.data` | `summary` |
| `body.data` | `body` |
| `timestamp.data` | `timestamp` (microseconds since boot) |
| `timeout.data` | `expire_timeout` (microseconds) |
| `urgency.data` | `urgency_name` → convert to `urgency` int |
| `category.data` | `category` |
| `icon_path.data` | `icon_path` |
| `default_action_name.data` | `default_action_name` |
| `stack_tag.data` | `extensions.stack_tag` |
| `progress.data` | `extensions.progress` |
| `message.data` | `extensions.message` |
| `urls.data` | `extensions.urls` |

**Timestamp handling**: Dunst timestamps are microseconds since boot. Convert to Unix timestamp:
```
unix_ts = boot_time + (dunst_timestamp / 1_000_000)
```

### Stdin Adapter

Accepts JSON in either:
1. **histui native format** (this schema)
2. **dunstctl format** (auto-detected and converted)
3. **Simple format** (minimal fields):
   ```json
   {"app_name": "app", "summary": "title", "body": "text"}
   ```

### D-Bus Adapter (Future)

Captures from `org.freedesktop.Notifications.Notify` method calls:
- All parameters mapped directly to schema
- `hints` dict parsed for standard and vendor hints
- Raw image data optionally captured and base64-encoded
- `NotificationClosed` signal populates `closed` object
- `ActionInvoked` signal logged separately (future)

## Schema Evolution

Version the schema in the JSONL file header (first line):

```json
{"histui_schema_version": 1, "created_at": 1703577600}
```

Future versions:
- **v1**: Initial schema (this document)
- **v2+**: Add fields as needed, old fields remain for compatibility

## Example Records

### Simple Notification
```json
{"histui_id":"01HQGXK5P0000000000000000","histui_source":"dunst","histui_imported_at":1703577600,"id":123,"app_name":"firefox","summary":"Download Complete","body":"myfile.zip","timestamp":1703577500,"urgency":1,"urgency_name":"normal"}
```

### Progress Notification
```json
{"histui_id":"01HQGXK5P1000000000000001","histui_source":"dunst","histui_imported_at":1703577601,"id":124,"app_name":"pavucontrol","summary":"Volume","body":"","timestamp":1703577501,"urgency":1,"urgency_name":"normal","extensions":{"stack_tag":"volume","progress":75}}
```

### Rich Notification with Image
```json
{"histui_id":"01HQGXK5P2000000000000002","histui_source":"dbus","histui_imported_at":1703577602,"id":125,"app_name":"slack","summary":"New Message","body":"<b>John:</b> Hello!","timestamp":1703577502,"urgency":1,"urgency_name":"normal","category":"im.received","icon_path":"/usr/share/icons/slack.png","actions":[{"id":"reply","label":"Reply"},{"id":"dismiss","label":"Dismiss"}],"hints":{"desktop-entry":"slack"}}
```

## Implementation Notes

1. **Timestamps**: Always store as Unix timestamps (seconds). Convert daemon-specific formats on import.

2. **Deduplication**: Use `histui_dedupe_hash` to avoid storing the same notification from multiple sources.

3. **Image Storage**: For MVP, store only `icon_path` and `image_path` (file references). Raw `image_data` storage is optional and increases file size significantly.

4. **Markup**: Preserve original markup in `body`. Output adapters handle stripping/rendering as needed.

5. **Unknown Hints**: Store in `extensions.vendor_hints` to preserve for round-tripping.

6. **Transient Notifications**: If `hints.transient` is true, the notification should NOT be persisted (skip writing to JSONL).
