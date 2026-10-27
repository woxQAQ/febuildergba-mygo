package records

import (
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/locales"
)

type batchDraft struct {
	field, choice, value string
	language             string
	err                  error
}

func (p *pane) selectedIDs(rows []game.Row) []int {
	var ids []int
	for _, row := range rows {
		if p.chosen.Has(row.ID) {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

func (a *Page) batchEditor(c *ui.Context, table *game.Table, ids []int) {
	p := &a.panes[a.tab]
	b := &p.batch
	labels := make([]string, len(table.Fields))
	var selected *game.Field
	for i := range table.Fields {
		f := &table.Fields[i]
		labels[i] = locales.Label(f.Group) + " / " + locales.Label(f.ID)
		if f.ID == b.field {
			selected = f
		}
	}
	if b.language != i18n.Locale() {
		for i, f := range table.Fields {
			if f.ID == b.field {
				b.choice = labels[i]
			}
		}
		b.language = i18n.Locale()
	}
	value, parseErr := components.ParseInteger(b.value)
	valid := selected != nil && parseErr == nil
	if selected != nil {
		low, high := selected.Bounds()
		valid = valid && value >= low && value <= high
	}
	ui.Column(c.Key("batch")).Fill().Padding(20).Gap(12).Children(func() {
		ui.Text(c, locales.BatchTitle(len(ids))).FontSize(20).Bold()
		ui.Text(c, locales.BatchScope()).TextColor(c.Theme().TextMuted)
		ui.Field(c, locales.BatchField(), func() {
			ui.Select(c, &b.choice, labels).
				Disabled(a.state.Busy).
				OnChange(func() {
					for i, label := range labels {
						if label == b.choice {
							b.field = table.Fields[i].ID
							b.value = ""
							b.err = nil
							break
						}
					}
				})
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			components.TextDraft(
				a.state,
				ui.TextInput(c, &b.value).
					Label(locales.BatchValue()).
					Placeholder(locales.BatchValue()).
					Width(160).
					Disabled(a.state.Busy || selected == nil),
			)
			ui.Button(c, locales.ApplyBatch()).
				Disabled(a.state.Busy || !valid).
				OnClick(func() { a.applyBatch() })
			if selected != nil {
				low, high := selected.Bounds()
				ui.Textf(c, "%s: %d…%d", locales.AllowedRange(), low, high).
					TextColor(c.Theme().TextMuted)
			}
		})
		if b.err != nil {
			ui.Text(c, components.ErrorText(b.err))
		}
		cols := []ui.TableColumn{
			{ID: "id", Title: "ID", Width: 48},
			{ID: "name", Title: locales.Name()},
			{ID: "before", Title: locales.Before(), Width: 100},
			{ID: "after", Title: locales.After(), Width: 100},
		}
		ui.Table(c, nil, cols, len(ids), func(row, col int) {
			id := ids[row]
			switch col {
			case 0:
				ui.Textf(c, "%02X", id)
			case 1:
				for _, record := range table.Rows {
					if record.ID == id {
						ui.Text(c, record.Name).SingleLine()
						break
					}
				}
			case 2:
				if selected != nil {
					before, err := a.state.Session.Value(
						table.Kind,
						id,
						selected.ID,
					)
					if err == nil {
						ui.Textf(c, "%d", before)
					}
				}
			case 3:
				if valid {
					ui.Textf(c, "%d", value)
				}
			}
		}).Grow(1).Label(locales.BatchPreview())
	})
}

func (a *Page) applyBatch() {
	if a.state.Busy || a.state.Session == nil || a.tab >= len(a.panes) {
		return
	}
	p := &a.panes[a.tab]
	ids := p.selectedIDs(a.rows())
	if len(ids) < 2 {
		p.batch.err = game.ErrField
		return
	}
	value, err := components.ParseInteger(p.batch.value)
	if err != nil {
		p.batch.err = game.ErrValue
		return
	}
	p.batch.err = a.state.Session.Set(
		game.Kinds[a.tab],
		ids,
		p.batch.field,
		value,
	)
	if p.batch.err == nil {
		a.state.HistoryEpoch++
		a.state.Err = nil
	}
}
