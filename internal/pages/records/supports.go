package records

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type (
	supportInput struct {
		text    string
		invalid bool
	}
	supportState struct {
		draft       *support.Draft
		original    []support.Relationship
		plan        *support.Plan
		err         error
		inputs      map[string]*supportInput
		synchronize bool
		generation  int
	}
)

func (s *supportState) invalid() bool {
	for _, v := range s.inputs {
		if v.invalid {
			return true
		}
	}
	return false
}

func (a *form) profileSupports(
	c *ui.Context,
	row records.Row,
	entry *profileState,
) {
	v := &entry.support
	editor := a.state.Session.Supports
	reload := func() { *v = supportState{generation: v.generation + 1} }
	if v.draft != nil && !v.draft.Current(editor) && !v.draft.Modified() &&
		!v.invalid() {
		reload()
	}
	if v.inputs == nil {
		v.inputs = make(map[string]*supportInput)
		v.draft, v.err = editor.NewDraft(row.ID)
		if v.draft != nil {
			v.original = v.draft.Relationships()
		}
	}
	ui.Column(c.Key(fmt.Sprintf("support/%d", v.generation))).
		MaxWidth(800).
		Gap(10).
		Children(func() {
			if v.err != nil {
				ui.Text(c, components.ErrorText(v.err)).
					TextColor(c.Theme().Danger)
			}
			if v.draft == nil {
				ui.Button(c, locales.QuoteReload()).
					Disabled(a.state.Busy).
					OnClick(reload)
				return
			}
			stale := !v.draft.Current(editor)
			disabled := stale || a.state.Busy
			if stale {
				ui.Text(c, locales.SupportStale()).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "%s: %d / %d", locales.SupportRelationships(), len(v.draft.Relationships()), v.draft.Capacity()).
					FontSize(13)
				if a.openSupportTalk != nil {
					ui.Button(c, locales.SupportViewDialogues()).
						Disabled(a.state.Busy).
						OnClick(func() { a.openSupportTalk(row.ID, 0) })
				}
			})
			ui.Text(c, locales.SupportRelationshipHelp()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
			if v.draft.Address() == 0 {
				ui.Text(c, locales.SupportInitialize()).FontSize(12)
			}
			a.supportRelations(c, v, disabled)
			ui.Checkbox(c, &v.synchronize, locales.SupportSynchronize()).
				Disabled(disabled).
				OnChange(func() { v.plan = nil }).
				Tooltip(locales.SupportSynchronizeHelp())
			if a.showRaw {
				ui.Textf(c, "0x%08X · % X", v.draft.Address(), v.draft.Raw()).
					Font("monospace").
					FontSize(12).
					Selectable()
				ui.Button(c, locales.EditBytes()).
					Disabled(disabled || v.draft.Address() == 0 || a.openHex == nil).
					OnClick(func() { a.openHex(v.draft.Address(), len(v.draft.Raw())) })
			}
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Button(c, locales.SupportPreview()).
					Disabled(disabled || v.invalid() || !v.draft.Modified()).
					OnClick(func() { v.plan, v.err = editor.Plan(v.draft, v.synchronize) })
				label := locales.QuoteReload()
				if v.draft.Modified() || v.invalid() {
					label = locales.DeployReload()
				}
				ui.Button(c, label).Disabled(a.state.Busy).OnClick(reload)
			})
			if v.plan != nil {
				a.supportPreview(c, v, disabled, reload)
			}
		})
}

func (a *form) supportName(id int) string {
	if table := a.state.Session.Records.Table(
		records.Units,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID == id {
				return row.Name
			}
		}
	}
	return ""
}

func (a *form) supportRelations(c *ui.Context, v *supportState, disabled bool) {
	var choices []string
	values := map[string]uint32{}
	if table := a.state.Session.Records.Table(
		records.Units,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID < 1 || row.ID > 255 || row.ID == v.draft.Unit().ID {
				continue
			}
			label := fmt.Sprintf("%02X · %s", row.ID, row.Name)
			choices = append(choices, label)
			values[label] = uint32(row.ID)
		}
	}
	width, _ := c.Size()
	available := min(float32(800), max(float32(300), width-a.sidebar-40))
	ui.ScrollHorizontal(c.Key(v.generation)).
		Width(available).
		MaxWidthPercent(100).
		Children(func() {
			ui.Grid(c).
				Width(max(available, 510)).
				ColumnTracks(ui.Fr(1), ui.Fixed(72), ui.Fixed(72), ui.Fixed(136)).
				GapX(8).
				GapY(6).
				AlignItems(ui.Center).
				Children(func() {
					for _, label := range []string{locales.SupportPartner(), locales.SupportInitial(), locales.SupportGrowth(), ""} {
						ui.Text(c, label).FontSize(12)
					}
					for i, relation := range v.draft.Relationships() {
						a.supportField(
							c,
							v,
							i,
							"unit",
							locales.SupportPartner(),
							uint32(relation.Unit),
							choices,
							values,
							disabled,
							func(value uint32) error {
								r := v.draft.Relationships()[i]
								r.Unit = uint8(value)
								return v.draft.Set(i, r)
							},
						)
						a.supportField(
							c,
							v,
							i,
							"initial",
							locales.SupportInitial(),
							uint32(relation.Initial),
							nil,
							nil,
							disabled,
							func(value uint32) error {
								r := v.draft.Relationships()[i]
								r.Initial = uint8(value)
								return v.draft.Set(i, r)
							},
						)
						a.supportField(
							c,
							v,
							i,
							"growth",
							locales.SupportGrowth(),
							uint32(relation.Growth),
							nil,
							nil,
							disabled,
							func(value uint32) error {
								r := v.draft.Relationships()[i]
								r.Growth = uint8(value)
								return v.draft.Set(i, r)
							},
						)
						ui.Row(c.Key(fmt.Sprintf("actions/%d", i))).
							Gap(4).
							Children(func() {
								ui.Button(c, locales.SupportTalkLink()).
									Height(26).
									Disabled(a.state.Busy || a.openSupportTalk == nil || v.invalid()).
									OnClick(func() { a.openSupportTalk(v.draft.Unit().ID, int(relation.Unit)) })
								ui.Button(c, locales.SupportRemove()).
									Label(fmt.Sprintf("%s / %02X", locales.SupportRemove(), relation.Unit)).
									Height(26).
									Disabled(disabled || v.invalid()).
									OnClick(func() {
										v.err = v.draft.Remove(i)
										v.plan = nil
										v.generation++
										v.inputs = make(
											map[string]*supportInput,
										)
									})
							})
					}
				})
		})
	used := map[uint8]bool{}
	for _, r := range v.draft.Relationships() {
		used[r.Unit] = true
	}
	next := uint8(0)
	for _, label := range choices {
		candidate := uint8(values[label])
		if !used[candidate] {
			next = candidate
			break
		}
	}
	ui.Button(c, locales.SupportAdd()).
		AlignSelf(ui.Start).
		Disabled(disabled || v.invalid() || len(v.draft.Relationships()) >= v.draft.Capacity() || next == 0).
		OnClick(func() {
			v.err = v.draft.Add(support.Relationship{Unit: next})
			v.plan = nil
		})
}

func (a *form) supportField(
	c *ui.Context,
	v *supportState,
	index int,
	field, label string,
	value uint32,
	choices []string,
	values map[string]uint32,
	disabled bool,
	set func(uint32) error,
) {
	key := fmt.Sprintf("%d/%s", index, field)
	current := v.inputs[key]
	if current == nil {
		current = &supportInput{text: fmt.Sprint(value)}
		for _, choice := range choices {
			if values[choice] == value {
				current.text = choice
				break
			}
		}
		v.inputs[key] = current
	}
	var control ui.Element
	if choices != nil {
		control = ui.Autocomplete(c.Key(key), &current.text, choices)
	} else {
		control = ui.TextInput(c.Key(key), &current.text).
			TextAlign(ui.End).
			FontFeatures("tnum")
	}
	components.TextDraft(
		a.state,
		control.Height(26).
			FontSize(13).
			Label(fmt.Sprintf("%s / %d / %s", locales.SupportEditor(), index+1, label)).
			Disabled(disabled).
			OnChange(func() {
				v.plan = nil
				number, chosen := values[current.text]
				if !chosen {
					parsed, err := input.ParseInteger(current.text)
					if err != nil || parsed < 0 || parsed > 255 {
						current.invalid = true
						return
					}
					number = uint32(parsed)
				}
				v.err = set(number)
				current.invalid = v.err != nil
			}),
	)
	if current.invalid {
		control.Error(locales.GameInvalidValue())
	}
}

func (a *form) supportPreview(
	c *ui.Context,
	v *supportState,
	disabled bool,
	reload func(),
) {
	ui.Column(c.Key("preview")).
		Gap(8).
		Padding(8, 0).
		BorderWidth(1, 0).
		BorderColor(c.Theme().Border).
		Children(func() {
			current := v.draft.Relationships()
			for _, r := range current {
				if slices.Contains(v.original, r) {
					continue
				}
				ui.Textf(c, "%02X %s · %s %d · %s %d", r.Unit, a.supportName(int(r.Unit)), locales.SupportInitial(), r.Initial, locales.SupportGrowth(), r.Growth).
					FontSize(13)
			}
			for _, r := range v.original {
				if !slices.ContainsFunc(
					current,
					func(next support.Relationship) bool { return next.Unit == r.Unit },
				) {
					ui.Textf(c, "%s: %02X %s", locales.SupportRemove(), r.Unit, a.supportName(int(r.Unit))).
						FontSize(13)
				}
			}
			ui.Text(c, locales.SupportPreviewHelp()).FontSize(12)
			for _, move := range v.plan.Moves() {
				if move.Unit != v.draft.Unit().ID {
					ui.Textf(c, "%s: %02X %s", locales.SupportSynchronize(), move.Unit, a.supportName(move.Unit)).
						FontSize(12)
				}
				for _, other := range move.Shared {
					ui.Textf(c, "%s: %02X %s", locales.SupportShared(), other.ID, other.Name).
						FontSize(12)
				}
				if a.showRaw {
					ui.Textf(c, "0x%08X → 0x%08X", move.Before, move.After).
						Font("monospace").
						FontSize(12)
				}
			}
			for _, unit := range v.plan.Missing() {
				ui.Textf(c, "%s: %02X %s", locales.SupportMissingReverse(), unit.ID, unit.Name).
					FontSize(12)
			}
			ui.Button(c, locales.SupportApply()).
				AlignSelf(ui.Start).
				Disabled(disabled || v.invalid() || !v.plan.Current(a.state.Session.Supports)).
				OnClick(func() {
					v.err = a.state.Session.Supports.Apply(v.plan)
					if v.err == nil {
						a.state.Session.Refresh()
						a.state.HistoryEpoch++
						reload()
					}
				})
		})
}
