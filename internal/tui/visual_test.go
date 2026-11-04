package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jmylchreest/histui/internal/model"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

// TestVisualFrames snapshots the actual View output, including ANSI colour.
// No database, D-Bus, user config, clipboard or background commands are opened.
// Do not parallelize: Lip Gloss's default renderer is process-global.
func TestVisualFrames(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
	for _, name := range []string{"empty", "list", "preview", "narrow", "search-empty", "dismissed", "detail", "detail-scrolled", "detail-image", "detail-image-scrolled", "detail-image-next", "help"} {
		t.Run(name, func(t *testing.T) {
			width, height := 100, 30
			if name == "narrow" {
				width, height = 48, 20
			}
			m := New(nil, nil, nil, nil)
			resized, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
			m = resized.(Model)
			m.previewActive = name == "preview"
			m.filterMode = FilterAll
			m.daemonStatus = "[offline]"
			// A fixed future timestamp keeps RelativeTime deterministic without changing
			// the production clock. These synthetic fixtures are valid until 2100.
			m.notifications = []model.Notification{
				{HistuiID: "fixture-1", AppName: "Build", Summary: "Build finished", Body: "All checks passed. <b>Ready for review.</b>", Timestamp: 4102444800, Urgency: 1, UrgencyName: "normal", Extensions: &model.Extensions{StackTag: "build", Progress: 75}},
				{HistuiID: "fixture-2", AppName: "Build", Summary: "Previous build", Body: "A stacked notification", Timestamp: 4102444800, Urgency: 0, UrgencyName: "low", Extensions: &model.Extensions{StackTag: "build", Progress: -1}},
				{HistuiID: "fixture-3", AppName: "Calendar", Summary: "Meeting dismissed", Body: "Synthetic notification", Timestamp: 4102444800, Urgency: 2, UrgencyName: "critical", HistuiDismissedAt: 1},
			}
			m.stackTagCounts = map[string]int{"build": 2}
			m.activeIDs = map[string]bool{"fixture-1": true}
			if name == "empty" {
				m.notifications = nil
			}
			if name == "dismissed" {
				m.filterMode = FilterDismissed
			}
			if name == "search-empty" {
				m.mode = ModeSearch
				m.searchQuery = "no-such-notification"
				m.searchInput.SetValue(m.searchQuery)
				m.searchInput.Blur()
			}
			m.list.SetItems(m.buildListItems())
			if strings.HasPrefix(name, "detail") {
				m.mode = ModeDetail
				m.selected = &m.notifications[0]
				m.selected.Body = strings.Repeat("Line of notification content for scrolling and wrapping.\n", 35)
				if strings.HasPrefix(name, "detail-image") {
					img := image.NewRGBA(image.Rect(0, 0, 16, 16))
					for y := 0; y < 16; y++ {
						for x := 0; x < 16; x++ {
							img.SetRGBA(x, y, color.RGBA{uint8(x * 16), uint8(y * 16), 128, 255})
						}
					}
					var data bytes.Buffer
					require.NoError(t, png.Encode(&data, img))
					for y := 0; y < 16; y++ {
						for x := 0; x < 16; x++ {
							img.SetRGBA(x, y, color.RGBA{128, uint8(255 - y*16), uint8(255 - x*16), 255})
						}
					}
					var second bytes.Buffer
					require.NoError(t, png.Encode(&second, img))
					m.detailImages = []imageSource{{label: "synthetic image", data: data.Bytes(), size: int64(data.Len())}, {label: "second image", data: second.Bytes(), size: int64(second.Len())}}
				}
				m.viewport.SetContent(m.renderDetail(*m.selected, m.calculateDetailTextWidth()))
				if strings.HasSuffix(name, "scrolled") {
					for range 12 {
						next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
						m = next.(Model)
					}
					require.Greater(t, m.viewport.YOffset, 0)
				}
				if name == "detail-image-next" {
					next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
					m = next.(Model)
					require.Equal(t, 1, m.detailImageIndex)
				}
			}
			if name == "help" {
				m.mode = ModeHelp
			}
			got := []byte(m.View())
			require.NotEmpty(t, got)
			path := filepath.Join("testdata", "visual", name+".ansi")
			if dir := os.Getenv("HISTUI_VISUAL_ARTIFACTS"); dir != "" {
				require.NoError(t, os.MkdirAll(dir, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, name+".ansi"), got, 0644))
			}
			if os.Getenv("UPDATE_VISUAL") == "1" {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, got, 0644))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err, "baseline missing; explicitly review UPDATE_VISUAL=1 output")
			require.Equal(t, string(want), string(got), "visual frame changed: review ANSI diff and screenshot before updating")
		})
	}
}
