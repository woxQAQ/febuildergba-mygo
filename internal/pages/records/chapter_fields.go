package records

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (a *form) chapterFields(
	c *ui.Context,
	row records.Row,
	table *records.Table,
) {
	sections := make(map[string][]records.Field)
	groups := []string{records.GroupGeneral}
	for _, field := range table.Fields {
		group := field.Group
		if group == records.GroupBase {
			group = records.GroupGeneral
		}
		if group != records.GroupGeneral && group != records.GroupAdvanced &&
			sections[group] == nil {
			groups = append(groups, group)
		}
		sections[group] = append(sections[group], field)
	}
	groups = append(groups, records.GroupAdvanced)
	ui.Row(c.Key("chapter-fields")).
		Wrap().
		Gap(16).
		AlignItems(ui.Start).
		Children(func() {
			for _, group := range groups {
				ui.Column(c.Key(group)).
					Width(360).
					MaxWidthPercent(100).
					Gap(6).
					Children(func() {
						if group != records.GroupAdvanced {
							ui.Text(c, locales.Label(group)).Bold()
						}
						ui.Row(c).Wrap().Gap(6).Children(func() {
							for _, field := range sections[group] {
								inputWidth := float32(56)
								fieldWidth := float32(172)
								if field.Width > 2 {
									inputWidth, fieldWidth = 104, 220
								} else if field.Width == 2 {
									inputWidth = 72
								}
								if field.Text != text.NoRole {
									fieldWidth = 360
								}
								ui.Column(c.Key(field.ID)).
									Width(fieldWidth).
									MaxWidthPercent(100).
									Children(func() {
										a.inlineField(
											c,
											records.Chapters,
											row,
											field,
											inputWidth,
											0,
											group == records.GroupAdvanced,
										)
									})
							}
						})
					})
			}
		})
}
