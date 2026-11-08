package pages

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/applog"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/icons"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type logRow struct {
	entry   applog.Entry
	json    string
	details string
	search  string
}

// LogViewer owns one window's filters, selection and detached log snapshots.
// Logging workers only touch the shared buffer; widget state stays on the UI thread.
type LogViewer struct {
	source           *applog.RingBuffer
	sequence         uint64
	cutoff           uint64
	entries          []logRow
	rows             []int
	query            string
	level            string
	allLabel         string
	filteredQuery    string
	filteredLevel    string
	filteredSequence uint64
	filteredCutoff   uint64
	paused           bool
	selection        int
	selected         *logRow
	list             ui.ListState
	split            float32
	search           ui.Handle
}

func NewLogViewer(source *applog.RingBuffer) *LogViewer {
	return &LogViewer{
		source:    source,
		selection: -1,
		split:     62,
		list:      ui.ListState{FollowEnd: true},
	}
}

func (viewer *LogViewer) refresh() {
	if viewer.paused || viewer.source == nil ||
		viewer.source.Sequence() == viewer.sequence {
		return
	}
	entries := viewer.source.Snapshot()
	viewer.entries = make([]logRow, 0, len(entries))
	for _, entry := range entries {
		encoded := entry.JSON()
		var formatted bytes.Buffer
		if err := json.Indent(&formatted, encoded, "", "  "); err != nil {
			formatted.Write(encoded)
		}
		row := logRow{
			entry:   entry,
			json:    string(encoded),
			details: strings.TrimSpace(formatted.String()),
			search:  strings.ToLower(string(encoded)),
		}
		viewer.entries = append(viewer.entries, row)
		viewer.sequence = entry.Sequence
	}
}

func (viewer *LogViewer) filter() {
	if viewer.filteredQuery == viewer.query &&
		viewer.filteredLevel == viewer.level &&
		viewer.filteredSequence == viewer.sequence &&
		viewer.filteredCutoff == viewer.cutoff {
		return
	}
	viewer.filteredQuery, viewer.filteredLevel = viewer.query, viewer.level
	viewer.filteredSequence, viewer.filteredCutoff = viewer.sequence, viewer.cutoff
	query := strings.ToLower(strings.TrimSpace(viewer.query))
	minimum, restricted := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}[viewer.level]
	viewer.rows = viewer.rows[:0]
	viewer.selection = -1
	for index, row := range viewer.entries {
		if row.entry.Sequence <= viewer.cutoff ||
			restricted && row.entry.Level < minimum ||
			!strings.Contains(row.search, query) {
			continue
		}
		if viewer.selected != nil &&
			viewer.selected.entry.Sequence == row.entry.Sequence {
			viewer.selection = len(viewer.rows)
		}
		viewer.rows = append(viewer.rows, index)
	}
}

func (viewer *LogViewer) copyFiltered(c *ui.Context) {
	var content strings.Builder
	for _, index := range viewer.rows {
		content.WriteString(viewer.entries[index].json)
	}
	c.Services().WriteClipboard(content.String())
}

func (viewer *LogViewer) View(c *ui.Context) {
	components.ApplyTheme(c)
	all := locales.LogAllLevels()
	if viewer.level == "" || viewer.level == viewer.allLabel {
		viewer.level = all
	}
	viewer.allLabel = all
	viewer.refresh()
	viewer.filter()
	if !viewer.paused {
		c.After(250 * time.Millisecond)
	}
	ui.Column(c.Key("application-logs")).Fill().Children(func() {
		ui.Row(c.Key("tools")).
			Wrap().
			Gap(8).
			Padding(10, 12).
			AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).
			BorderColor(c.Theme().Border).
			Children(func() {
				ui.TextInput(c.Key("search"), &viewer.query).
					Bind(&viewer.search).
					Grow(1).
					MinWidth(220).
					Label(locales.LogSearch()).
					Placeholder(locales.LogSearch())
				ui.Select(c.Key("level"), &viewer.level, []string{all, "DEBUG", "INFO", "WARN", "ERROR"}).
					Width(128).
					Label(locales.LogMinimumLevel())
				pause, pauseIcon := locales.LogPause(), icons.Lucide.Pause()
				if viewer.paused {
					pause, pauseIcon = locales.LogResume(), icons.Lucide.Play()
				}
				components.ActionButton(c.Key("pause"), pauseIcon, pause).
					OnClick(func() { viewer.paused = !viewer.paused })
				components.ActionButton(c, icons.Lucide.ChevronsDown(), locales.LogLatest()).
					Disabled(len(viewer.rows) == 0).
					OnClick(func() { viewer.list.ScrollToEnd() })
				components.ActionButton(c, icons.Lucide.Copy(), locales.LogCopyFiltered()).
					Disabled(len(viewer.rows) == 0).
					OnClick(func() { viewer.copyFiltered(c) })
				ui.Button(c, locales.LogClearView()).
					Tooltip(locales.LogClearHelp()).
					OnClick(func() {
						if viewer.source != nil {
							viewer.cutoff = viewer.source.Sequence()
						}
						viewer.selected = nil
						viewer.selection = -1
						viewer.list = ui.ListState{FollowEnd: true}
					})
			})
		ui.SplitVerticalPercent(c.Key("log-panes"), &viewer.split, func() { viewer.table(c) }, func() { viewer.details(c) }).
			Grow(1)
		ui.Row(c.Key("status")).
			Wrap().
			Gap(10).
			Padding(6, 12).
			BorderWidth(1, 0, 0, 0).
			BorderColor(c.Theme().Border).
			Children(func() {
				capacity := 0
				if viewer.source != nil {
					capacity = viewer.source.Capacity()
				}
				ui.Text(c, locales.LogVisibleCount(len(viewer.rows), capacity)).
					FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted)
				if viewer.paused {
					ui.Text(c, locales.LogPaused()).
						FontSize(components.FontCaption).
						TextColor(c.Theme().TextMuted)
				}
				if viewer.selected != nil && viewer.selection < 0 {
					ui.Text(c, locales.LogSelectionOutside()).
						FontSize(components.FontCaption).
						TextColor(c.Theme().TextMuted)
				}
			})
	})
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { viewer.search.Focus(c) })
}

func (viewer *LogViewer) table(c *ui.Context) {
	if len(viewer.rows) == 0 {
		ui.Column(c).
			Fill().
			Padding(16).
			Children(func() { ui.Text(c, locales.LogNoEntries()).TextColor(c.Theme().TextMuted) })
		return
	}
	viewer.list.Selected = &viewer.selection
	viewer.list.Key = func(row int) any { return viewer.entries[viewer.rows[row]].entry.Sequence }
	viewer.list.Label = func(row int) string { return viewer.entries[viewer.rows[row]].entry.Message }
	columns := []ui.TableColumn{
		{ID: "time", Title: locales.LogTime(), Width: 112, Fixed: true},
		{ID: "level", Title: locales.LogLevel(), Width: 72, Fixed: true},
		{ID: "operation", Title: locales.LogOperation(), Width: 180},
		{ID: "message", Title: locales.LogMessage()},
	}
	ui.Table(c.Key("entries"), &viewer.list, columns, len(viewer.rows), func(row, column int) {
		entry := viewer.entries[viewer.rows[row]].entry
		switch column {
		case 0:
			ui.Text(c, entry.Time.Format("15:04:05.000")).
				FontFeatures("tnum").
				SingleLine()
		case 1:
			text := ui.Text(c, entry.Level.String()).SingleLine()
			if row != viewer.selection {
				if entry.Level >= slog.LevelError {
					text.TextColor(c.Theme().Danger)
				} else if entry.Level >= slog.LevelWarn {
					text.TextColor(c.Theme().Warning)
				}
			}
		case 2:
			ui.Text(c, entry.Operation).SingleLine()
		case 3:
			ui.Text(c, entry.Message).SingleLine()
		}
	}).
		Grow(1).
		Label(locales.LogEntries()).
		OnChange(func() {
			if viewer.selection >= 0 && viewer.selection < len(viewer.rows) {
				selected := viewer.entries[viewer.rows[viewer.selection]]
				viewer.selected = &selected
			}
		})
}

func (viewer *LogViewer) details(c *ui.Context) {
	ui.Column(c.Key("details")).Fill().Padding(12).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.LogDetails()).Bold().Grow(1)
			components.ActionButton(c, icons.Lucide.Copy(), locales.LogCopySelected()).
				Disabled(viewer.selected == nil).
				OnClick(func() {
					if viewer.selected != nil {
						c.Services().WriteClipboard(viewer.selected.json)
					}
				})
		})
		ui.Scroll(c).Grow(1).Children(func() {
			if viewer.selected == nil {
				ui.Text(c, locales.LogSelectEntry()).
					TextColor(c.Theme().TextMuted)
				return
			}
			ui.Text(c, viewer.selected.details).
				Font("monospace").
				Selectable().
				Label(locales.LogDetailsBody())
		})
	})
}
