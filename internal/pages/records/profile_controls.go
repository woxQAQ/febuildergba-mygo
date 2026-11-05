package records

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (entry *profileState) choice(
	id string,
	value int64,
	label string,
) *choiceDraft {
	draft := entry.choices[id]
	if draft == nil {
		draft = &choiceDraft{}
		entry.choices[id] = draft
	}
	if !draft.initialized || draft.value != value || draft.text == draft.label {
		draft.text, draft.label, draft.value, draft.initialized = label, label, value, true
	}
	return draft
}

func (a *form) profileClass(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	entry *profileState,
	id string,
) ui.Element {
	field, exists := profileField(table, id)
	if !exists {
		return ui.Element{}
	}
	value, err := a.state.Session.Records.Value(kind, row.ID, id)
	if err != nil {
		return ui.Element{}
	}
	var labels []string
	var ids []int
	target := -1
	caption := fmt.Sprintf("%02X · %s", value, locales.RecordUnknownReference())
	if classes := a.state.Session.Records.Table(
		records.Classes,
	); classes != nil &&
		classes.Err == nil {
		for _, class := range classes.Rows {
			label := fmt.Sprintf("%02X · %s", class.ID, class.Name)
			labels = append(labels, label)
			ids = append(ids, class.ID)
			if int64(class.ID) == value {
				caption = label
				target = class.ID
			}
		}
	}
	if target < 0 {
		labels = append(labels, caption)
		ids = append(ids, int(value))
	}
	draft := entry.choice(id, value, caption)
	return ui.Column(c.Key(id)).Gap(4).Children(func() {
		help := locales.RecordDefaultClassHelp()
		if kind == records.Classes {
			help = locales.RecordRelatedClassHelp()
		}
		ui.Text(c, locales.Label(id)).
			FontSize(components.FontCaption).
			Tooltip(help)
		ui.Row(c).Gap(0).AlignItems(ui.Center).Children(func() {
			components.TextDraft(
				a.state,
				ui.Combobox(c, &draft.text, labels).
					Grow(1).
					MinWidth(0).
					Height(26).
					Label(a.fieldLabel(kind, field)).
					Tooltip(help).
					Disabled(a.state.Busy).
					OnChange(func() {
						if index := slices.Index(
							labels,
							draft.text,
						); index >= 0 {
							a.state.SetErr(a.state.Edit(func() error {
								return a.state.Session.Records.Set(
									kind,
									[]int{row.ID},
									id,
									int64(ids[index]),
								)
							}))
						}
					}),
			)
			components.ReferenceLink(c, locales.OpenReference(caption),
				a.state.Busy || target < 0 || a.jump == nil,
				func() { a.jump(records.Classes, target) },
			)
		})
	})
}

func (a *form) profileEnum(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	entry *profileState,
	id string,
	keys []string,
) ui.Element {
	field, exists := profileField(table, id)
	if !exists {
		return ui.Element{}
	}
	value, err := a.state.Session.Records.Value(kind, row.ID, id)
	if err != nil {
		return ui.Element{}
	}
	labels := make([]string, len(keys))
	for index, key := range keys {
		labels[index] = locales.Label(key)
	}
	caption := fmt.Sprintf("%d · %s", value, locales.RecordUnknownReference())
	if value >= 0 && value < int64(len(labels)) {
		caption = labels[value]
	} else {
		labels = append(labels, caption)
	}
	draft := entry.choice(id, value, caption)
	return ui.Column(c.Key(id)).Gap(4).Children(func() {
		ui.Text(c, locales.Label(id)).FontSize(components.FontCaption)
		ui.Select(c, &draft.text, labels).
			WidthPercent(100).
			MinWidth(0).
			Height(26).
			Label(a.fieldLabel(kind, field)).
			Disabled(a.state.Busy).
			OnChange(func() {
				if index := slices.Index(
					labels,
					draft.text,
				); index >= 0 &&
					index < len(keys) {
					a.state.SetErr(a.state.Edit(func() error {
						return a.state.Session.Records.Set(
							kind,
							[]int{row.ID},
							id,
							int64(index),
						)
					}))
				}
			})
	})
}

func (a *form) profileWeapons(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	available float32,
) {
	weapons, err := a.state.Session.Records.WeaponSkills(kind, row.ID)
	if err != nil {
		ui.Text(c, components.ErrorText(err))
		return
	}
	ui.ScrollHorizontal(c.Key("weapons")).
		Width(available).
		MaxWidthPercent(100).
		Children(func() {
			tracks := []ui.Track{ui.Fixed(92)}
			for range weapons {
				tracks = append(tracks, ui.Fr(1))
			}
			ui.Grid(c).
				Width(max(available, 620)).
				ColumnTracks(tracks...).
				GapX(6).
				GapY(3).
				AlignItems(ui.Center).
				Children(func() {
					ui.Text(c, "")
					for _, weapon := range weapons {
						a.profileRank(c, kind, row, weapon)
					}
					label := locales.Label(records.GroupWeapon)
					if kind == records.Classes {
						label = locales.RecordClassRanks()
					}
					ui.Text(c, label)
					for _, weapon := range weapons {
						field, _ := profileField(table, weapon.Field)
						a.numberField(c, kind, row, field, weapon.Value)
					}
				})
		})
}

func (a *form) profileRank(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	weapon records.WeaponSkill,
) {
	rank := rankCaption(weapon.EffectiveRank)
	if weapon.Inherited && weapon.ReferenceError != nil {
		rank = "?"
	}
	help := locales.Label(weapon.Weapon) + "\n" + locales.RecordWeaponsHelp()
	if weapon.Inherited {
		help += "\n" + locales.RecordInherit() + " " + rank
	}
	if weapon.ReferenceError != nil {
		help += "\n" + components.ErrorText(weapon.ReferenceError)
	}
	control := ui.ButtonBase(c.Key("rank/"+weapon.Field)).
		Height(24).Padding(2, 4).Radius(2).
		Cursor(ui.CursorPointer).
		Label(locales.RecordWeaponRank() + " / " + locales.Label(weapon.Weapon)).
		Value(rank).
		Tooltip(help).
		Disabled(a.state.Busy)
	if control.Hovered() {
		control.Background(c.Theme().SurfaceHover)
	}
	control.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			icon := a.thumbnails.Weapon(
				a.state.Session,
				slices.Index(records.WeaponNames, weapon.Weapon),
			)
			if icon.Available() {
				icon.View(c, 16, locales.Label(weapon.Weapon))
			} else {
				ui.Text(c, locales.Label(weapon.Weapon)).
					FontSize(components.FontCaption)
			}
			ui.Textf(c, "%s ▾", rank).
				FontSize(components.FontCaption).
				TextAlign(ui.End).
				Grow(1)
		})
	})
	control.PopoverMenuButton(func(menu *ui.Menu) {
		for _, option := range []string{"none", "E", "D", "C", "B", "A", "S"} {
			label := rankCaption(option)
			if option == "none" && kind == records.Units {
				inherited := rankCaption(weapon.ClassRank)
				if weapon.ReferenceError != nil {
					inherited = "?"
				}
				label = locales.RecordInherit() + " " + inherited
			}
			if menu.Item(label).Radio(option == weapon.Rank).Chosen() {
				a.state.SetErr(a.state.Edit(func() error {
					return a.state.Session.Records.SetWeaponRank(
						kind, row.ID, weapon.Field, option,
					)
				}))
			}
		}
	})
}

func rankCaption(rank string) string {
	if rank == "none" {
		return "—"
	}
	return rank
}

func (a *form) profileTraits(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
) {
	flags, err := a.state.Session.Records.Value(kind, row.ID, "flags")
	if err != nil {
		return
	}
	traits := a.state.Session.Records.RecordTraits(kind)
	width, _ := c.Size()
	columns := 4
	if width-a.sidebar < 800 {
		columns = 2
	}
	ui.Column(c.Key("profile-traits")).Gap(6).Children(func() {
		ui.Text(c, locales.RecordTraits()).
			Bold().
			Tooltip(locales.RecordTraitsHelp())
		ui.Grid(c).Columns(columns).GapX(16).GapY(12).Children(func() {
			for bank := range 4 {
				ui.Column(c.Key(bank)).Gap(1).Children(func() {
					ui.Row(c).
						Gap(8).
						AlignItems(ui.Center).
						Padding(0, 0, 4, 0).
						BorderWidth(0, 0, 1, 0).
						BorderColor(c.Theme().Border).
						Children(func() {
							ui.Textf(c, "%s %d", locales.RecordTraits(), bank+1).
								FontSize(components.FontCaption).
								Grow(1)
							ui.Textf(c, "0x%02X", (flags>>(bank*8))&0xFF).
								Font("monospace").
								FontSize(components.FontCaption).
								TextColor(c.Theme().TextMuted)
						})
					for bit := range 8 {
						trait := traits[bank*8+bit]
						on := flags&(int64(1)<<trait.Bit) != 0
						label := locales.Label(trait.Label)
						if !trait.Known {
							label = fmt.Sprintf(
								"%s %d",
								locales.RecordUnknownBit(),
								trait.Bit,
							)
						}
						ui.Checkbox(c.Key(trait.Bit), &on, label).
							Height(23).
							FontSize(components.FontCaption).
							Label(locales.RecordTraits() + " / " + label).
							Tooltip(locales.Label(trait.Help)).
							Disabled(a.state.Busy || !trait.Known).
							OnClick(func() {
								a.state.SetErr(
									a.state.Edit(
										func() error { return a.state.Session.Records.SetRecordTrait(kind, row.ID, trait.Bit, on) },
									),
								)
							})
					}
				})
			}
		})
	})
}
