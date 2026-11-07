package records

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (a *form) item(c *ui.Context, row records.Row, table *records.Table) {
	width, _ := c.Size()
	available := max(float32(0), width-a.sidebar-32)
	preview := a.previewFor(records.Items, row.ID)
	options := []string{"weapon_type", "weapon_rank", "combat.range"}
	combat := []string{
		"combat.uses",
		"combat.might",
		"combat.hit",
		"combat.weight",
		"combat.critical",
		"price",
		"weapon_experience",
	}
	rendered := map[string]bool{
		"flags":         true,
		"use_effect":    true,
		"weapon_effect": true,
	}
	for _, id := range append(append([]string{}, options...), combat...) {
		rendered[id] = true
	}
	ui.Column(c.Key(fmt.Sprintf("item/%d", row.ID))).Fill().Children(func() {
		ui.Scroll(c.Key("item")).Grow(1).Padding(16).Children(func() {
			ui.Column(c).
				Width(min(980, available)).
				MaxWidthPercent(100).
				Gap(20).
				Children(func() {
					ui.Row(c.Key("identity")).
						Wrap().
						Gap(12).
						AlignItems(ui.Start).
						Children(func() {
							if preview != nil {
								ui.ButtonBase(c.Key("icon-link")).
									Size(36, 36).
									Padding(2).
									FocusRing(true).
									Label(locales.OpenReference(locales.ItemIconTable())).
									Tooltip(locales.ItemIconOpenHelp()).
									Disabled(a.state.Busy || a.openItemIcon == nil).
									OnClick(func() { a.openItemIcon(preview.ref.ID, row.ID) }).
									Children(func() {
										if preview.bitmap != nil {
											ui.Image(c, preview.bitmap).
												Size(32, 32).
												Label(row.Name + " " + locales.ImagePreview())
										} else {
											ui.Text(c, "—").
												TextColor(c.Theme().TextMuted)
										}
									})
							}
							ui.Textf(c, "%02X", row.ID).
								Margin(9, 0, 0, 0).
								FontSize(components.FontCaption).
								FontFeatures("tnum").
								TextColor(c.Theme().TextMuted)
							a.nameEditor(c, records.Items, row).
								Basis(320).
								Grow(1).
								MinWidth(min(220, available)).
								MaxWidthPercent(100)
						})
					if preview != nil && preview.err != nil {
						ui.Text(c, components.ErrorText(preview.err)).
							TextColor(c.Theme().Danger)
					}
					if row.TextError != nil {
						ui.Text(c, locales.TextError()).
							TextColor(c.Theme().Danger)
					}
					ui.Row(c.Key("descriptions")).
						Wrap().
						Gap(16).
						AlignItems(ui.Start).
						Children(func() {
							for _, field := range table.Fields {
								if field.Text == text.Description {
									a.descriptionEditor(c, records.Items, row, field, 64).
										Basis(300).
										Grow(1).
										MinWidth(min(300, available)).
										MaxWidthPercent(100)
								}
							}
						})
					ui.Column(c.Key("combat")).Gap(10).Children(func() {
						ui.Text(c, locales.Label(records.GroupCombat)).Bold()
						ui.Row(c.Key("requirements")).
							Wrap().
							Gap(12).
							AlignItems(ui.Start).
							Children(func() {
								for _, id := range options {
									if field, exists := profileField(
										table,
										id,
									); exists {
										a.itemOptionField(c, row, field)
									}
								}
							})
						a.itemNumberGrid(
							c,
							row,
							table,
							combat,
							max(2, min(7, int((min(980, available)+12)/106))),
						)
					})
					ui.Row(c.Key("effects")).
						Wrap().
						Gap(16).
						AlignItems(ui.Start).
						Children(func() {
							for _, id := range []string{"use_effect", "weapon_effect"} {
								if field, exists := profileField(
									table,
									id,
								); exists {
									a.itemOptionField(c, row, field).
										Basis(300).
										Grow(1).
										MinWidth(min(300, available)).
										MaxWidthPercent(100)
								}
							}
						})
					a.itemTraits(c, row, min(980, available))
					var remaining []string
					for _, field := range table.Fields {
						if field.Text != text.NoRole || rendered[field.ID] {
							continue
						}
						remaining = append(remaining, field.ID)
					}
					ui.Column(c.Key("identifiers")).Gap(10).Children(func() {
						ui.Text(c, locales.ItemOtherSettings()).Bold()
						a.itemNumberGrid(
							c,
							row,
							table,
							remaining,
							max(2, min(5, int((min(980, available)+12)/132))),
						)
					})
				})
		})
	})
	a.chooseItemText(c)
}
