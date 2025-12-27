package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var setOpts struct {
	// Input options
	stdin     bool // Read IDs from stdin
	stdinJSON bool // Parse stdin as JSON and extract histui_id

	// Actions (mutually exclusive)
	dismiss   bool
	undismiss bool
	seen      bool
	delete    bool
}

var setCmd = &cobra.Command{
	Use:   "set [id...]",
	Short: "Modify notification state",
	Long: `Modify notification state (dismiss, undismiss, seen, delete).

IDs can be provided as positional arguments or via stdin (--stdin).
When using --stdin, each line is scanned for a ULID pattern.

Examples:
  # Dismiss a specific notification
  histui set 01HZ3X2J5YFMK2V3P4Q6R7S8T9 --dismiss

  # Dismiss multiple notifications
  histui set ID1 ID2 ID3 --dismiss

  # Dismiss notifications from pipe (ids format)
  histui get --filter "app=discord" --format ids | histui set --stdin --dismiss

  # Dismiss from JSON output
  histui get --format json | histui set --stdin-json --dismiss

  # Mark all from an app as seen
  histui get --filter "app=slack" --format ids | histui set --stdin --seen

  # Delete old notifications
  histui get --filter "timestamp<7d" --format ids | histui set --stdin --delete`,
	RunE: runSet,
}

func init() {
	rootCmd.AddCommand(setCmd)

	// Input flags
	setCmd.Flags().BoolVar(&setOpts.stdin, "stdin", false,
		"Read IDs from stdin (one per line, or scans for ULID pattern)")
	setCmd.Flags().BoolVar(&setOpts.stdinJSON, "stdin-json", false,
		"Read JSON from stdin and extract histui_id field")

	// Action flags
	setCmd.Flags().BoolVar(&setOpts.dismiss, "dismiss", false,
		"Mark notification(s) as dismissed")
	setCmd.Flags().BoolVar(&setOpts.undismiss, "undismiss", false,
		"Clear dismissed state from notification(s)")
	setCmd.Flags().BoolVar(&setOpts.seen, "seen", false,
		"Mark notification(s) as seen")
	setCmd.Flags().BoolVar(&setOpts.delete, "delete", false,
		"Permanently delete notification(s) from history")
}

func runSet(cmd *cobra.Command, args []string) error {
	// Validate that exactly one action is specified
	actionCount := 0
	if setOpts.dismiss {
		actionCount++
	}
	if setOpts.undismiss {
		actionCount++
	}
	if setOpts.seen {
		actionCount++
	}
	if setOpts.delete {
		actionCount++
	}

	if actionCount == 0 {
		return fmt.Errorf("must specify an action: --dismiss, --undismiss, --seen, or --delete")
	}
	if actionCount > 1 {
		return fmt.Errorf("only one action can be specified at a time")
	}

	// Collect IDs
	ids := args

	// Read from stdin if requested
	if setOpts.stdin || setOpts.stdinJSON {
		stdinIDs, err := readIDsFromStdin()
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		ids = append(ids, stdinIDs...)
	}

	if len(ids) == 0 {
		return fmt.Errorf("no notification IDs provided")
	}

	// Remove duplicates
	ids = uniqueStrings(ids)

	// Perform the action
	var successCount, failCount int
	for _, id := range ids {
		err := performAction(id)
		if err != nil {
			logger.Warn("failed to update notification", "id", id, "error", err)
			failCount++
		} else {
			successCount++
		}
	}

	// Report results
	action := "updated"
	if setOpts.dismiss {
		action = "dismissed"
	} else if setOpts.undismiss {
		action = "undismissed"
	} else if setOpts.seen {
		action = "marked as seen"
	} else if setOpts.delete {
		action = "deleted"
	}

	if failCount > 0 {
		fmt.Fprintf(os.Stderr, "%s %d notifications, %d failed\n", action, successCount, failCount)
	} else {
		fmt.Printf("%s %d notifications\n", action, successCount)
	}

	return nil
}

// readIDsFromStdin reads IDs from stdin.
func readIDsFromStdin() ([]string, error) {
	var ids []string
	scanner := bufio.NewScanner(os.Stdin)

	if setOpts.stdinJSON {
		// Parse as JSON array or newline-delimited JSON
		return readJSONFromStdin(scanner)
	}

	// Read lines and extract ULIDs
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to extract ULID from line
		id := extractULID(line)
		if id != "" {
			ids = append(ids, id)
		}
	}

	return ids, scanner.Err()
}

// ULID regex pattern: 26 characters, alphanumeric (0-9, A-Z excluding I, L, O, U)
var ulidPattern = regexp.MustCompile(`\b[0-9A-HJ-KM-NP-TV-Z]{26}\b`)

// extractULID attempts to extract a ULID from a line.
// Handles:
//   - Bare ULID: "01HZ3X2J5YFMK2V3P4Q6R7S8T9"
//   - Dmenu output: "1 | 5m | App | Summary" (returns empty, use --stdin-json for this)
//   - Any line containing a ULID pattern
func extractULID(line string) string {
	// First try the whole line as a bare ULID
	line = strings.TrimSpace(line)
	if ulidPattern.MatchString(line) && len(line) == 26 {
		return line
	}

	// Search for ULID pattern in line
	match := ulidPattern.FindString(line)
	if match != "" {
		return match
	}

	return ""
}

// readJSONFromStdin parses JSON from stdin and extracts histui_id fields.
func readJSONFromStdin(scanner *bufio.Scanner) ([]string, error) {
	var ids []string

	// Read all content
	var content strings.Builder
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(content.String())
	if text == "" {
		return nil, nil
	}

	// Try parsing as JSON array
	if strings.HasPrefix(text, "[") {
		var items []map[string]any
		if err := json.Unmarshal([]byte(text), &items); err == nil {
			for _, item := range items {
				if id, ok := item["histui_id"].(string); ok && id != "" {
					ids = append(ids, id)
				}
			}
			return ids, nil
		}
	}

	// Try parsing as newline-delimited JSON (NDJSON)
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err == nil {
			if id, ok := item["histui_id"].(string); ok && id != "" {
				ids = append(ids, id)
			}
		}
	}

	return ids, nil
}

// performAction performs the selected action on a notification.
func performAction(id string) error {
	n := historyStore.GetByID(id)
	if n == nil {
		return fmt.Errorf("notification not found: %s", id)
	}

	switch {
	case setOpts.dismiss:
		n.MarkDismissed()
	case setOpts.undismiss:
		n.Undismiss()
	case setOpts.seen:
		n.MarkSeen()
	case setOpts.delete:
		return historyStore.Delete(id)
	}

	return historyStore.Update(*n)
}

// uniqueStrings removes duplicates from a string slice.
func uniqueStrings(input []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(input))
	for _, s := range input {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}
