---
title: Filtering Notifications
description: Filter syntax and operators for querying notifications
sidebar_position: 5
---

# Filtering Notifications

histui provides powerful filtering capabilities for querying your notification history.

## Filter Syntax

Filters use a simple `key:value` syntax:

```bash
histui get --filter "app:firefox"
```

## Available Filters

| Filter | Description | Example |
|--------|-------------|---------|
| `app` | Application name | `app:firefox` |
| `urgency` | Urgency level | `urgency:critical` |
| `summary` | Match summary text | `summary:download` |
| `body` | Match body text | `body:error` |

## Combining Filters

Multiple filters are combined with AND logic:

```bash
histui get --filter "app:firefox urgency:critical"
```

## Time-Based Filtering

Use the `--since` flag for time-based filtering:

```bash
# Last hour
histui get --since 1h

# Last 30 minutes
histui get --since 30m

# Last 7 days
histui get --since 168h
```

## Examples

### Critical Firefox notifications from the last hour

```bash
histui get --filter "app:firefox urgency:critical" --since 1h
```

### All download-related notifications

```bash
histui get --filter "summary:download"
```
