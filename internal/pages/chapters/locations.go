package chapters

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type locationInteraction struct {
	adding, moving bool
	target         int
	kind           int
	template       string
}

func locationCaption(condition event.Condition) string {
	if condition.Location.Kind != "" {
		return locales.Label(string(condition.Location.Kind))
	}
	return fmt.Sprintf(
		"0x%02X / 0x%04X",
		condition.Kind,
		condition.Location.Command,
	)
}

func (v *Page) locationEditor(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	width float32,
	advanced bool,
	openHex func(int, int),
	openText func(uint16),
) {
	key := eventKey{row.ID, event.LocationEvents}
	entry, err := v.eventDraft(state, key)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).
			TextColor(c.Theme().Danger).
			Tooltip(err.Error())
		return
	}
	entry.selected = min(entry.selected, max(0, len(entry.data.Events())-1))
	stale := !entry.data.Current(state.Session.Events)
	if stale || state.Busy ||
		(entry.location.moving && entry.location.target != entry.selected) {
		entry.location = locationInteraction{}
	}
	wide := width >= 760
	mapWidth, inspectorWidth := width, width
	if wide {
		inspectorWidth = 320
		mapWidth = width - inspectorWidth - 20
	}
	_, height := c.Size()
	mapHeight := min(max(240, height-490), 560)
	if !wide {
		mapHeight = min(mapHeight, 240)
	}
	ui.Column(c.Key(key)).Gap(12).Children(func() {
		if stale {
			ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
		}
		if entry.location.adding || entry.location.moving {
			c.OnShortcut(
				0,
				ui.KeyEscape,
				func() { entry.location = locationInteraction{} },
			)
		}
		ui.Row(c.Key("workspace")).
			Wrap().
			Gap(20).
			AlignItems(ui.Start).
			Children(func() {
				ui.Column(c.Key("location-map")).
					Width(mapWidth).
					MaxWidthPercent(100).
					Shrink(0).
					Gap(10).
					Children(func() {
						overlay := v.locationOverlay(state, entry)
						overlay.Tools = func(c *ui.Context) { v.locationTools(c, state, entry) }
						if entry.location.adding {
							overlay.Prompt = locales.LocationChoosePosition()
						} else if entry.location.moving {
							overlay.Prompt = locales.LocationMovePosition(
								fmt.Sprintf("%02X", entry.location.target),
							)
						}
						overlay.Viewport = image.Pt(
							int(mapWidth),
							int(mapHeight),
						)
						v.renderMap(c, state, row, overlay)
						ui.Row(c).
							Wrap().
							Gap(8).
							AlignItems(ui.Center).
							Children(func() {
								ui.Text(c, locales.LocationEvents()).
									Bold().
									Grow(1)
								ui.Button(c, locales.LocationCopy()).
									Disabled(state.Busy || stale || len(entry.data.Events()) == 0).
									OnClick(func() {
										state.Err = entry.data.CopyLocation(
											entry.selected,
										)
										if state.Err == nil {
											entry.selected = len(
												entry.data.Events(),
											) - 1
											entry.preview, entry.lastSelected = nil, -1
											entry.location = locationInteraction{}
										}
									})
								ui.Button(c, locales.LocationRemove()).
									Disabled(state.Busy || stale || len(entry.data.Events()) == 0).
									OnClick(func() { state.Err = entry.removeSelected() })
							})
						v.eventTable(
							c,
							entry,
							entry.data.Events(),
							state.Session,
						)
					})
				inspector := ui.Column(c.Key("location-inspector")).
					Width(inspectorWidth).
					MaxWidthPercent(100).
					Shrink(0).
					Gap(16).
					BorderColor(c.Theme().Border)
				if wide {
					inspector.Padding(0, 0, 0, 16).BorderWidth(0, 0, 0, 1)
				} else {
					inspector.Padding(16, 0, 0, 0).BorderWidth(1, 0, 0, 0)
				}
				inspector.Children(func() {
					if len(entry.data.Events()) == 0 {
						ui.Text(c, locales.LocationEmpty()).
							TextColor(c.Theme().TextMuted)
					} else {
						v.locationInspector(
							c,
							state,
							entry,
							advanced,
							openHex,
							openText,
						)
					}
					v.eventActions(c, state, row, entry)
				})
			})
	})
}

func (v *Page) locationTools(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
) {
	disabled := state.Busy || !entry.data.Current(state.Session.Events)
	ui.Row(c.Key("location-tools")).
		Wrap().
		Gap(8).
		MaxWidthPercent(100).
		AlignItems(ui.Center).
		Children(func() {
			ui.Button(c, locales.LocationAdd()).
				Tooltip(locales.LocationScope()).
				Disabled(disabled || entry.location.adding).
				OnClick(func() { entry.location = locationInteraction{adding: true} })
			if entry.location.adding {
				ui.Text(c, locales.LocationType())
				kinds := []event.LocationKind{
					event.House,
					event.Village,
					event.Chest,
					event.Door,
					event.Seize,
					event.Shop,
				}
				labels := make([]string, len(kinds))
				for i, kind := range kinds {
					labels[i] = locales.Label(string(kind))
				}
				if !slices.Contains(labels, entry.location.template) {
					entry.location.template = labels[entry.location.kind]
				}
				ui.Select(c.Key("new-location-type"), &entry.location.template, labels).
					Width(140).
					Label(locales.LocationAdd() + " / " + locales.LocationType()).
					OnChange(func() { entry.location.kind = max(0, slices.Index(labels, entry.location.template)) })
				if v.mapErr != nil {
					ui.Button(c, locales.LocationAddOrigin()).OnClick(func() {
						v.placeLocation(state, entry, image.Point{})
					})
				}
			}
			if entry.location.adding || entry.location.moving {
				ui.Button(c, locales.Cancel()).
					OnClick(func() { entry.location = locationInteraction{} })
			}
		})
}

func (v *Page) placeLocation(
	state *editor.State,
	entry *eventState,
	point image.Point,
) {
	if state.Busy || !entry.data.Current(state.Session.Events) {
		return
	}
	index := entry.location.target
	if entry.location.adding {
		kinds := [...]event.LocationKind{
			event.House,
			event.Village,
			event.Chest,
			event.Door,
			event.Seize,
			event.Shop,
		}
		index = len(entry.data.Events())
		if state.Err = entry.data.AddLocation(
			state.Session.Events,
			kinds[entry.location.kind],
		); state.Err != nil {
			return
		}
	}
	if state.Err = entry.data.SetLocationPosition(
		index,
		point,
	); state.Err != nil {
		if entry.location.adding {
			_ = entry.data.Remove(index)
		}
		return
	}
	delete(entry.inputs, inputKey{index, event.LocationX})
	delete(entry.inputs, inputKey{index, event.LocationY})
	entry.selected, entry.lastSelected, entry.preview = index, -1, nil
	entry.location = locationInteraction{}
}

func (v *Page) locationOverlay(
	state *editor.State,
	entry *eventState,
) mapview.Overlay {
	overlay := mapview.Overlay{}
	events := entry.data.Events()
	for i, condition := range events {
		if condition.Location.Kind == "" {
			continue
		}
		point := condition.Location.Position
		overlay.Regions = append(overlay.Regions, mapview.Region{
			Bounds: image.Rect(point.X, point.Y, point.X+1, point.Y+1),
			Index:  i, Selected: i == entry.selected,
		})
	}
	if len(events) > 0 && entry.lastSelected != entry.selected {
		entry.lastSelected = entry.selected
		if events[entry.selected].Location.Kind != "" {
			overlay.Focus = new(events[entry.selected].Location.Position)
		}
	}
	overlay.Choose = func(point image.Point) {
		if entry.location.adding || entry.location.moving {
			v.placeLocation(state, entry, point)
			return
		}
		var matches []int
		for _, region := range overlay.Regions {
			if point.In(region.Bounds) {
				matches = append(matches, region.Index)
			}
		}
		if len(matches) > 0 {
			next := slices.Index(matches, entry.selected) + 1
			entry.selected = matches[next%len(matches)]
			entry.list.ScrollTo(entry.selected, ui.Center)
		}
	}
	return overlay
}

func (v *Page) locationInspector(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	advanced bool,
	openHex func(int, int),
	openText func(uint16),
) {
	condition := entry.data.Events()[entry.selected]
	disabled := state.Busy || !entry.data.Current(state.Session.Events)
	ui.Textf(c, "%02X  %s", entry.selected, locationCaption(condition)).
		FontSize(18).
		Bold()
	fields := entry.data.Fields(entry.selected)
	ui.Column(c.Key(fmt.Sprintf("location-%d", entry.selected))).
		Gap(12).
		Children(func() {
			if len(fields) == 0 {
				ui.Text(c, locales.TurnOpaque()).
					FontSize(13).
					TextColor(c.Theme().TextMuted)
			} else {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, locales.DeployPosition()).Bold().Grow(1)
					ui.Button(c, locales.LocationPlace()).
						Disabled(disabled || entry.location.moving).
						OnClick(func() { entry.location = locationInteraction{moving: true, target: entry.selected} })
				})
				ui.Row(c).Wrap().Gap(16).Children(func() {
					for _, field := range fields[:2] {
						ui.Row(c.Key(field.ID)).
							Gap(6).
							AlignItems(ui.Center).
							Children(func() {
								label := "X"
								if field.ID == event.LocationY {
									label = "Y"
								}
								ui.Text(c, label)
								v.eventNumber(c, state, entry, field)
							})
					}
				})
				ui.Row(c.Key(event.EventFlag)).
					Gap(8).
					AlignItems(ui.Center).
					Children(func() {
						ui.Text(c, locales.Label(event.EventFlag)).Grow(1)
						v.eventNumber(c, state, entry, fields[2])
					})
				if condition.Flag == 0 {
					ui.Text(c, locales.LocationRepeat()).
						Tooltip(locales.LocationRepeatHelp()).
						FontSize(12).
						TextColor(c.Theme().TextMuted)
				}
			}
			ui.Column(c.Key("trigger")).Gap(10).Padding(12, 0, 0, 0).
				BorderWidth(1, 0, 0, 0).
				BorderColor(c.Theme().Border).Children(func() {
				title := locales.LocationContent()
				switch condition.OperandKind {
				case event.ConditionScript:
					title = locales.ScriptEditor()
					if _, created := entry.data.Dialog(
						entry.selected,
					); created ||
						len(condition.Texts) > 0 {
						title = locales.LocationDialogue()
					}
				case event.ShopTarget:
					title = locales.ShopInventory()
				case event.RewardTarget, event.RandomRewardTarget:
					title = locales.LocationReward()
				}
				ui.Text(c, title).Bold()
				switch condition.OperandKind {
				case event.ConditionScript:
					if id, created := entry.data.Dialog(
						entry.selected,
					); created {
						v.dialoguePicker(c, state, entry, id)
						if id != 0 {
							v.locationTexts(
								c,
								state,
								entry,
								condition,
								openText,
							)
						}
					} else {
						if condition.Script == 1 {
							ui.Text(c, locales.LocationNativeAction()).
								FontSize(13)
						}
						v.locationTexts(c, state, entry, condition, openText)
					}
					v.eventLinks(c, state, condition)
					ui.Row(c).Wrap().Gap(8).Children(func() {
						ui.Button(c, locales.LocationOpenScript()).
							Tooltip(locales.LocationScriptHelp()).
							Disabled(disabled || entry.data.Modified() || condition.Script < 0xC0 || condition.Address == 0 || v.openScript == nil).
							OnClick(func() { v.openScript(v.chapter, condition.Address+4) })
					})
					ui.Collapsible(
						c.Key("change-script"),
						locales.LocationChangeScript(),
						&entry.locationScript,
						func() {
							v.conditionScript(c, state, entry, condition)
						},
					)
				case event.RewardTarget:
					v.locationReward(c, state, entry, condition, fields)
				case event.ShopTarget:
					v.shopInventory(c, state, entry, condition, advanced)
				case event.RandomRewardTarget:
					ui.Text(c, locales.LocationResourceReadonly()).FontSize(13)
				case event.NoTarget:
					ui.Text(c, locales.LocationNativeAction()).FontSize(13)
				}
			})
			if advanced && condition.Address > 0 {
				ui.Row(c).Wrap().Gap(8).Children(func() {
					ui.Textf(c, "0x%08X → 0x%08X", condition.Address, condition.Operand).
						FontSize(12).
						Selectable()
					ui.Button(c, locales.EditBytes()).Disabled(state.Busy).
						OnClick(func() { openHex(condition.Address, len(condition.Raw)) })
				})
			}
		})
}

func (v *Page) locationReward(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	condition event.Condition,
	fields []records.Field,
) {
	location := condition.Location
	if location.Item == location.GoldItem {
		ui.Textf(c, "%s: %d", locales.ChestGold(), location.Money)
	} else {
		ui.Textf(
			c,
			"%04X · %s",
			location.Item,
			nameOf(state.Session, records.Items, location.Item),
		)
	}
	for _, field := range fields[3:] {
		ui.Row(c.Key(field.ID)).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.Label(field.ID)).Grow(1)
			v.eventNumber(c, state, entry, field)
		})
	}
	ui.Row(c).Children(func() {
		ui.Button(c, locales.ChestUseGold()).
			Disabled(state.Busy || !entry.data.Current(state.Session.Events) || location.Item == location.GoldItem).
			OnClick(func() {
				state.Err = entry.data.SetChestReward(
					entry.selected,
					location.GoldItem,
					max(location.GoldMinimum, location.Money),
				)
				if state.Err == nil {
					entry.preview = nil
					delete(
						entry.inputs,
						inputKey{entry.selected, event.ChestItem},
					)
					delete(
						entry.inputs,
						inputKey{entry.selected, event.ChestMoney},
					)
				}
			})
	})
	ui.Textf(c, "%s: %d–65535 · %s: 1–%d", locales.ChestGold(), location.GoldMinimum, locales.Label(event.ChestItem), location.ItemMaximum).
		FontSize(12).
		TextColor(c.Theme().TextMuted).
		Tooltip(locales.ChestRewardHelp())
}

func (v *Page) locationTexts(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	condition event.Condition,
	openText func(uint16),
) {
	if len(condition.Texts) == 0 {
		return
	}
	choices := make([]string, len(condition.Texts))
	for i, id := range condition.Texts {
		choices[i] = fmt.Sprintf("%04X", id)
	}
	if !slices.Contains(choices, entry.textReference) {
		entry.textReference = choices[0]
	}
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		if len(choices) > 1 {
			ui.Select(c.Key("reference-id"), &entry.textReference, choices).
				Width(90).Label(locales.EventTextRefs()).Disabled(state.Busy)
		}
		id := condition.Texts[max(0, slices.Index(choices, entry.textReference))]
		ui.Button(c, fmt.Sprintf("%s · %04X", locales.TextEditor(), id)).
			Disabled(state.Busy).OnClick(func() { openText(id) })
	})
	id := condition.Texts[max(0, slices.Index(choices, entry.textReference))]
	value, err := state.Session.Texts.Read(id)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().TextMuted)
		return
	}
	ui.Text(c, locationTextExcerpt(value.Content)).
		FontSize(13).
		Selectable().
		Label(locales.LocationTextExcerpt())
}

// Excerpts omit decoded text markup; escaped brackets remain literal. The full
// command-bearing text remains available through the text editor.
func locationTextExcerpt(content string) string {
	var out strings.Builder
	for len(content) > 0 {
		switch content[0] {
		case '\\':
			if len(content) > 1 {
				content = content[1:]
			}
		case '[':
			if end := strings.IndexByte(content, ']'); end >= 0 {
				marker := content[1:end]
				if marker == "A" || marker == "Clear" || marker == "NL" {
					out.WriteByte(' ')
				}
				content = content[end+1:]
				continue
			}
		}
		out.WriteByte(content[0])
		content = content[1:]
	}
	runes := []rune(strings.Join(strings.Fields(out.String()), " "))
	if len(runes) > 180 {
		runes = append(runes[:180], '…')
	}
	return string(runes)
}
