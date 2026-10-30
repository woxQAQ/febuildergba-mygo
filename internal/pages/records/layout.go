package records

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (a *form) recordFields(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
) {
	sections := make(map[string][]records.Field)
	var extra []string
	for _, field := range table.Fields {
		group := field.Group
		if group == records.GroupBase {
			group = records.GroupGeneral
		}
		if group != records.GroupGeneral && group != records.GroupCombat &&
			group != records.GroupWeapon &&
			group != records.GroupAdvanced &&
			sections[group] == nil {
			extra = append(extra, group)
		}
		sections[group] = append(sections[group], field)
	}
	groups := append(
		[]string{
			records.GroupCombat,
			records.GroupWeapon,
			records.GroupGeneral,
		},
		extra...)
	groups = append(groups, records.GroupAdvanced)
	width, _ := c.Size()
	ui.Column(c.Key("field-groups")).Gap(24).Children(func() {
		for _, group := range groups {
			if len(sections[group]) == 0 ||
				group == records.GroupAdvanced && !a.showRaw {
				continue
			}
			columns := 2
			if group == records.GroupWeapon {
				columns = 4
			}
			if width-a.sidebar-40 < 440 {
				columns = max(1, columns/2)
			}
			a.inlineSection(c, kind, row, group, sections[group], columns).
				MaxWidth(600)
		}
	})
}

func (a *form) inlineSection(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	group string,
	fields []records.Field,
	columns int,
) ui.Element {
	// Share input and action widths so each row keeps its values aligned.
	inputWidth, actionWidth := float32(56), float32(0)
	for _, f := range fields {
		if f.Width > 2 {
			inputWidth = 104
		} else if f.Width == 2 {
			inputWidth = max(inputWidth, 72)
		}
		if f.Reference != "" || f.Text != text.NoRole {
			actionWidth = 48
		}
	}
	return ui.Column(c.Key(group)).Gap(8).Children(func() {
		ui.Text(c, locales.Label(group)).Bold().FontSize(13)
		ui.Grid(c).Columns(columns).GapX(16).GapY(4).Children(func() {
			for _, f := range fields {
				a.inlineField(c, kind, row, f, inputWidth, actionWidth)
			}
		})
	})
}

func (a *form) inlineField(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	f records.Field,
	inputWidth, actionWidth float32,
) {
	ui.Row(c.Key(f.ID)).Gap(6).MinHeight(28).Children(func() {
		key := fieldKey{kind, row.ID, f.ID}
		label := locales.Label(f.ID)
		help := label
		if kind == records.Chapters &&
			(f.ID == "objective_text_id" || f.ID == "objective_detail_text_id") {
			help += "\n" + locales.ChapterObjectiveHelp()
		}
		ui.Text(c, label).
			FontSize(13).
			SingleLine().
			MaxWidth(140).
			MinWidth(0).
			Grow(1).
			Tooltip(help).
			OnClick(func() { a.focusField(key) })
		value, err := a.state.Session.Records.Value(kind, row.ID, f.ID)
		if err != nil {
			ui.Text(c, components.ErrorText(err))
			return
		}
		a.numberField(c, kind, row, f, value).Width(inputWidth).Shrink(0)
		switch {
		case f.Reference != "":
			ui.Button(c, locales.Jump()).
				Size(actionWidth, 26).
				Padding(3, 6).
				Shrink(0).
				Disabled(a.state.Busy).
				OnClick(func() { a.jump(f.Reference, int(value)) })
		case f.Text != text.NoRole:
			ui.Button(c, locales.Jump()).
				Size(actionWidth, 26).
				Padding(3, 6).
				Shrink(0).
				Disabled(a.state.Busy || value <= 0 || value >= 0x7FFF).
				OnClick(func() {
					if value > 0 && value < 0x7FFF {
						a.openText(uint16(value))
					}
				})
		case f.Asset == graphics.Portrait && a.openPortrait != nil:
			ui.Button(c, locales.Jump()).
				Height(26).
				Disabled(a.state.Busy).
				OnClick(func() { a.openPortrait(int(value)) })
		case actionWidth > 0:
			ui.Box(c).Width(actionWidth).Shrink(0)
		}
	})
}
