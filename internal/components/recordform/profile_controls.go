package recordform

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
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

func (a *Form) profileClass(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	table *game.Table,
	entry *profileState,
	id string,
) {
	field, exists := profileField(table, id)
	if !exists {
		return
	}
	value, err := a.state.Session.Value(kind, row.ID, id)
	if err != nil {
		return
	}
	var labels []string
	var ids []int
	target := -1
	caption := fmt.Sprintf("%02X · %s", value, locales.RecordUnknownReference())
	if classes := a.state.Session.Table(
		game.Classes,
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
	ui.Row(c.Key(id)).Gap(6).AlignItems(ui.Center).Children(func() {
		help := locales.RecordDefaultClassHelp()
		if kind == game.Classes {
			help = locales.RecordRelatedClassHelp()
		}
		ui.Text(c, locales.Label(id)).Width(84).FontSize(12).Tooltip(help)
		components.TextDraft(
			a.state,
			ui.Combobox(c, &draft.text, labels).
				Width(190).
				Height(26).
				FontSize(13).
				Label(a.fieldLabel(kind, field)).
				Tooltip(help).
				Bind(&draft.input).
				Disabled(a.state.Busy).
				OnChange(func() {
					if index := slices.Index(labels, draft.text); index >= 0 {
						a.state.Err = a.state.Session.Set(
							kind,
							[]int{row.ID},
							id,
							int64(ids[index]),
						)
					}
				}),
		)
		ui.Button(c, locales.Jump()).
			Height(26).
			FontSize(12).
			Disabled(a.state.Busy || target < 0).
			OnClick(func() { a.jump(game.Classes, target) })
	})
	if draft.input.FocusWithin(c) {
		a.activeField = fieldKey{kind, row.ID, id}
	}
}

func (a *Form) profileEnum(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	table *game.Table,
	entry *profileState,
	id string,
	keys []string,
) {
	field, exists := profileField(table, id)
	if !exists {
		return
	}
	value, err := a.state.Session.Value(kind, row.ID, id)
	if err != nil {
		return
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
	ui.Row(c.Key(id)).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.Label(id)).FontSize(12)
		ui.Select(c, &draft.text, labels).
			MinWidth(72).
			Height(26).
			FontSize(13).
			Bind(&draft.input).
			Label(a.fieldLabel(kind, field)).
			Disabled(a.state.Busy).
			OnChange(func() {
				if index := slices.Index(
					labels,
					draft.text,
				); index >= 0 &&
					index < len(keys) {
					a.state.Err = a.state.Session.Set(
						kind,
						[]int{row.ID},
						id,
						int64(index),
					)
				}
			})
	})
	if draft.input.FocusWithin(c) {
		a.activeField = fieldKey{kind, row.ID, id}
	}
}

func (a *Form) profileWeapons(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	table *game.Table,
) {
	weapons, err := a.state.Session.WeaponSkills(kind, row.ID)
	if err != nil {
		ui.Text(c, components.ErrorText(err))
		return
	}
	width, _ := c.Size()
	available := max(float32(300), width-a.sidebar-40)
	ui.Column(c.Key("profile-weapons")).Gap(6).Children(func() {
		ui.Text(c, locales.Label(game.GroupWeapon)).
			Bold().
			FontSize(13).
			Tooltip(locales.RecordWeaponsHelp())
		ui.ScrollHorizontal(c).
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
							rank := rankCaption(weapon.EffectiveRank)
							if weapon.Inherited &&
								weapon.ReferenceError != nil {
								rank = "?"
							}
							help := locales.RecordWeaponsHelp()
							if weapon.Inherited {
								help += "\n" + locales.RecordInherit() + " " + rank
							}
							key := fieldKey{kind, row.ID, weapon.Field}
							ui.Textf(c, "%s %s", locales.Label(weapon.Weapon), rank).
								FontSize(12).
								TextAlign(ui.End).
								PaddingX(6).
								Label(locales.RecordWeaponField() + " / " + locales.Label(weapon.Weapon)).
								Tooltip(help).
								OnClick(func() { a.activeField = key; a.focusField(key) })
						}
						label := locales.RecordPersonalRanks()
						if kind == game.Classes {
							label = locales.RecordClassRanks()
						}
						ui.Text(c, label).FontSize(13)
						for _, weapon := range weapons {
							field, _ := profileField(table, weapon.Field)
							a.numberField(c, kind, row, field, weapon.Value)
						}
					})
			})
	})
}

func (a *Form) profileRank(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	table *game.Table,
	entry *profileState,
	weapon game.WeaponSkill,
) {
	keys := []string{"none", "E", "D", "C", "B", "A", "S"}
	var labels []string
	for _, rank := range keys {
		labels = append(labels, rankCaption(rank))
	}
	caption := rankCaption(weapon.Rank)
	if kind == game.Units {
		labels[0] = locales.RecordInherit() + " " + rankCaption(
			weapon.ClassRank,
		)
		if weapon.ReferenceError != nil {
			labels[0] = locales.RecordInherit() + " ?"
		}
		if weapon.Inherited {
			caption = labels[0]
		}
	}
	draft := entry.choice("rank/"+weapon.Field, weapon.Value, caption)
	field, _ := profileField(table, weapon.Field)
	help := locales.RecordWeaponsHelp()
	if weapon.ReferenceError != nil {
		help += "\n" + components.ErrorText(weapon.ReferenceError)
	}
	ui.Select(c.Key("rank/"+weapon.Field), &draft.text, labels).
		Width(110).Shrink(0).
		Height(26).
		FontSize(13).
		Label(locales.RecordWeaponRank() + " / " + locales.Label(weapon.Weapon)).
		Tooltip(help).
		Bind(&draft.input).
		Disabled(a.state.Busy).
		OnChange(func() {
			if index := slices.Index(labels, draft.text); index >= 0 {
				a.state.Err = a.state.Session.SetWeaponRank(
					kind,
					row.ID,
					weapon.Field,
					keys[index],
				)
			}
		})
	if draft.input.FocusWithin(c) {
		a.activeField = fieldKey{kind, row.ID, field.ID}
	}
}

func rankCaption(rank string) string {
	if rank == "none" {
		return "—"
	}
	return rank
}

func (a *Form) profileTraits(c *ui.Context, kind game.Kind, row game.Row) {
	flags, err := a.state.Session.Value(kind, row.ID, "flags")
	if err != nil {
		return
	}
	traits := a.state.Session.RecordTraits(kind)
	width, _ := c.Size()
	columns := 4
	if width-a.sidebar < 800 {
		columns = 2
	}
	ui.Column(c.Key("profile-traits")).Gap(6).Children(func() {
		ui.Text(c, locales.RecordTraits()).
			Bold().
			FontSize(13).
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
								FontSize(12).
								Grow(1)
							ui.Textf(c, "0x%02X", (flags>>(bank*8))&0xFF).
								Font("monospace").
								FontSize(12).
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
							FontSize(12).
							Label(locales.RecordTraits() + " / " + label).
							Tooltip(locales.Label(trait.Help)).
							Disabled(a.state.Busy || !trait.Known).
							OnClick(func() { a.state.Err = a.state.Session.SetRecordTrait(kind, row.ID, trait.Bit, on) })
					}
				})
			}
		})
	})
}
