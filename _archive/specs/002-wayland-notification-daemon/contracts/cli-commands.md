# CLI Commands Contract

**Feature**: histuid - Wayland Notification Daemon
**Date**: 2025-12-27

This document defines the new and modified CLI commands for histui.

---

## New Commands

### `histui set`

Modify notification state.

```
USAGE:
    histui set <ulid> [flags]
    histui set --stdin [flags]

ARGUMENTS:
    <ulid>              Notification ULID to modify

FLAGS:
    --stdin             Read notification identifiers from stdin
    --format string     Input format when using --stdin (default: "auto")
                        Values: auto, ids, json
    --dismiss           Mark notification(s) as dismissed
    --undismiss         Clear dismissed state
    --seen              Mark notification(s) as seen
    --delete            Permanently delete notification(s) from history

EXAMPLES:
    # Dismiss a single notification
    histui set 01HW3X5A2B3C4D5E6F7G8H9J0K --dismiss

    # Bulk dismiss from IDs format
    histui get --format ids | histui set --stdin --dismiss

    # Bulk dismiss using dmenu selection
    histui get --format dmenu | fuzzel -d | histui set --stdin --dismiss

    # Dismiss from JSON pipeline
    histui get --format json | jq 'select(.urgency == 0)' | histui set --stdin --format json --dismiss

    # Delete old dismissed notifications
    histui get --filter "dismissed=true,time>=1w" --format ids | histui set --stdin --delete

EXIT CODES:
    0 - Success
    1 - Error (invalid ULID, notification not found, etc.)

STDIN FORMATS:
    - auto: Detect format automatically
    - ids:  One ULID per line (26 characters, base32)
    - json: NDJSON with "histui_id" field

    When format=auto:
    - If line is exactly 26 characters and valid base32, treat as ULID
    - Otherwise, scan line for ULID pattern and extract first match
    - If line is valid JSON with "histui_id" field, extract ULID

BEHAVIOR:
    - State changes are persisted immediately
    - If histuid is running, popups are updated in real-time
    - Multiple flags can be combined (--dismiss --seen)
    - Errors are reported to stderr; processing continues for remaining items
```

### `histui dnd`

Control Do Not Disturb mode.

```
USAGE:
    histui dnd [on|off|toggle]

SUBCOMMANDS:
    on      Enable Do Not Disturb mode
    off     Disable Do Not Disturb mode
    toggle  Toggle Do Not Disturb mode

    (no subcommand) Show current DnD state

FLAGS:
    -q, --quiet     No output, only set exit code

EXAMPLES:
    # Check DnD status
    histui dnd
    # Output: "Do Not Disturb: disabled"

    # Enable DnD
    histui dnd on
    # Output: "Do Not Disturb: enabled"

    # Toggle DnD for waybar
    histui dnd toggle

    # Check status in script
    if histui dnd --quiet; then
        echo "DnD is disabled"
    else
        echo "DnD is enabled"
    fi

EXIT CODES:
    0 - DnD is disabled (or command succeeded)
    1 - DnD is enabled (when using no subcommand with --quiet)
    2 - Error

BEHAVIOR:
    - State is persisted to ~/.local/share/histui/state.json
    - If histuid is running, it detects the change via file watching
    - Changes are reflected in waybar within 500ms
```

---

## Modified Commands

### `histui get` (Extended)

New flags and format options added.

```
NEW FLAGS:
    --format ids        Output bare ULIDs, one per line
    --filter string     Rich filter expression

    Existing flags continue to work:
    --format dmenu      dmenu-compatible output
    --format json       NDJSON output (one JSON object per line)
    --format yaml       YAML output
    --app-filter        Filter by app name (exact match)
    --urgency           Filter by urgency level
    --since             Filter by time (e.g., "1h", "1d")
    --limit             Maximum results

FILTER SYNTAX:
    --filter "field=value,field2>value2,..."

    Fields:
        app         Application name (=, ~)
        summary     Notification title (=, ~)
        body        Notification body (=, ~)
        urgency     Urgency level (=, >, >=, <, <=)
        time        Age of notification (>, >=, <, <=)
        seen        Seen status (=true, =false)
        dismissed   Dismissed status (=true, =false)
        source      Source adapter (=histuid, =dunst, =stdin)

    Operators:
        =   Exact match (or equality for numbers)
        ~   Regex match (strings only)
        >   Greater than
        >=  Greater than or equal
        <   Less than
        <=  Less than or equal

    Time values:
        5m   5 minutes
        2h   2 hours
        1d   1 day
        1w   1 week

    Combining:
        ,   AND (all conditions must match)
        |   OR within regex (e.g., app~slack|discord)

EXAMPLES:
    # Get IDs for piping
    histui get --format ids

    # Critical notifications from last hour
    histui get --filter "urgency=critical,time<1h"

    # Unseen notifications from slack or discord
    histui get --filter "app~slack|discord,seen=false"

    # Complex jq pipeline
    histui get --format json | jq 'select(.app_name == "Slack")' | histui set --stdin --format json --seen

OUTPUT FORMATS:
    ids:    One ULID per line
            01HW3X5A2B3C4D5E6F7G8H9J0K
            01HW3X5A2B3C4D5E6F7G8H9J0L

    dmenu:  ULID followed by notification summary
            01HW3X5A2B3C4D5E6F7G8H9J0K [Slack] New message from Alice
            01HW3X5A2B3C4D5E6F7G8H9J0L [Discord] Bob: Hey!

    json:   NDJSON (one object per line, for jq streaming)
            {"histui_id":"01HW3X5A...","app_name":"Slack","summary":"New message","urgency":1,...}
            {"histui_id":"01HW3X5B...","app_name":"Discord","summary":"Bob: Hey!","urgency":1,...}
```

### `histui status` (Extended)

Extended output to include DnD state.

```
NEW FIELDS IN JSON OUTPUT:
    dnd         Boolean indicating Do Not Disturb state
    dnd_at      Timestamp when DnD was enabled (if enabled)

EXAMPLE OUTPUT:
    {
        "text": "5",
        "tooltip": "5 notifications",
        "class": "notification",
        "alt": "enabled",
        "dnd": false
    }

    When DnD is enabled:
    {
        "text": "3",
        "tooltip": "3 notifications (DnD)",
        "class": "notification dnd",
        "alt": "paused",
        "dnd": true
    }

BACKWARD COMPATIBILITY:
    - Existing fields (text, tooltip, class, alt) unchanged
    - New "dnd" field added, safe to ignore
    - "class" gets additional "dnd" class when DnD is active
```

---

## Error Handling

All commands follow consistent error handling:

| Scenario | Exit Code | Stderr Output |
|----------|-----------|---------------|
| Success | 0 | (none) |
| Invalid ULID | 1 | `error: invalid ULID: <input>` |
| Notification not found | 1 | `error: notification not found: <ulid>` |
| Invalid filter syntax | 1 | `error: invalid filter: <details>` |
| File I/O error | 1 | `error: <details>` |
| DnD enabled (quiet mode) | 1 | (none) |

---

## Compatibility Notes

- All existing `histui` commands remain unchanged
- New flags are additive and don't break existing scripts
- `--filter` can be combined with legacy flags (`--app-filter`, `--urgency`, `--since`)
- NDJSON format allows streaming with `jq` without `-s` flag
