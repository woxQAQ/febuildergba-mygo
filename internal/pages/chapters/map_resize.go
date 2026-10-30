package chapters

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (entry *mapState) resizeDimensions() (int, int, bool) {
	width, werr := input.ParseInteger(entry.resizeWidth)
	height, herr := input.ParseInteger(entry.resizeHeight)
	if werr != nil || herr != nil || width < 1 || width > maps.MaxMapWidth ||
		height < 1 ||
		height > maps.MaxMapHeight ||
		width*height > maps.MaxMapCells {
		return 0, 0, false
	}
	return int(width), int(height), true
}

func (entry *mapState) resizePending() bool {
	width, height, valid := entry.resizeDimensions()
	return !valid || width != entry.data.Width() ||
		height != entry.data.Height()
}

func (entry *mapState) resizeControls(
	c *ui.Context,
	state *editor.State,
	disabled bool,
) {
	options := []string{
		locales.MapAnchorTopLeft(),
		locales.MapAnchorTop(),
		locales.MapAnchorTopRight(),
		locales.MapAnchorLeft(),
		locales.MapAnchorCenter(),
		locales.MapAnchorRight(),
		locales.MapAnchorBottomLeft(),
		locales.MapAnchorBottom(),
		locales.MapAnchorBottomRight(),
	}
	if !slices.Contains(options, entry.anchorChoice) {
		entry.anchorChoice = options[entry.anchor]
	}
	width, height, valid := entry.resizeDimensions()
	ui.Column(c.Key("map-resize")).Gap(6).Children(func() {
		ui.Row(c).Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
			for _, field := range []struct {
				label string
				text  *string
			}{
				{locales.MapWidth(), &entry.resizeWidth}, {locales.MapHeight(), &entry.resizeHeight},
			} {
				ui.Row(c.Key(field.label)).
					Gap(6).
					AlignItems(ui.Center).
					Children(func() {
						ui.Text(c, field.label)
						components.TextDraft(
							state,
							ui.TextInput(c, field.text).
								Width(60).
								Label(locales.MapEditor()+" / "+field.label).
								Disabled(disabled).
								OnChange(func() { entry.preview = nil }),
						)
					})
			}
			ui.Select(c.Key("anchor"), &entry.anchorChoice, options).
				Label(locales.MapAnchor()).
				Tooltip(locales.MapAnchorHelp()).
				Disabled(disabled).
				OnChange(func() {
					index := slices.Index(options, entry.anchorChoice)
					if index >= 0 && index <= int(maps.MapBottomRight) {
						entry.anchor = maps.Anchor(index)
						entry.preview = nil
					}
				})
			ui.Button(c, locales.MapResize()).
				Disabled(disabled || !valid || (width == entry.data.Width() && height == entry.data.Height())).
				OnClick(func() {
					w, h, ok := entry.resizeDimensions()
					if !ok {
						state.Err = edit.ErrValue
						return
					}
					if state.Err = entry.data.Resize(
						w,
						h,
						entry.anchor,
						entry.tile.Tile,
					); state.Err == nil {
						entry.refresh()
						entry.resizing = false
						entry.editFocus.Focus(c)
					}
				})
		})
		ui.Text(c, fmt.Sprintf(locales.MapLimits(), maps.MaxMapWidth, maps.MaxMapHeight, maps.MaxMapCells)).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.MapResizeHelp()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		if entry.resizePending() {
			ui.Text(c, locales.MapResizePending()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
	})
}

func (v *Page) mapImpacts(
	c *ui.Context,
	state *editor.State,
	entry *mapState,
	jump func(records.Kind, int),
	openHex func(int, int),
) {
	if !entry.preview.Resized() {
		return
	}
	items := entry.preview.Impacts()
	ui.Text(c, locales.MapImpactTitle()).
		Bold().
		Tooltip(locales.MapImpactScope())
	if len(items) == 0 {
		ui.Text(c, locales.MapImpactNone()).TextColor(c.Theme().TextMuted)
	} else {
		entry.impactList.Selected = &entry.impactSelected
		entry.impactList.Key = func(index int) any { return index }
		columns := []ui.TableColumn{
			{ID: "chapter", Title: locales.Chapters(), Width: 80, Fixed: true},
			{ID: "kind", Title: locales.MapImpactKind()},
			{
				ID:    "record",
				Title: locales.MapImpactIndex(),
				Width: 116,
				Fixed: true,
			},
			{ID: "bounds", Title: locales.MapImpactBounds()},
			{ID: "status", Title: locales.MapImpactStatus()},
		}
		ui.Table(c.Key("map-impacts"), &entry.impactList, columns, len(items), func(index, column int) {
			item := items[index]
			text := ""
			switch column {
			case 0:
				text = fmt.Sprintf("%02X", item.Chapter)
			case 1:
				text = locales.Label(string(item.Kind))
			case 2:
				text = fmt.Sprintf("%02X", item.Index)
				if item.Step >= 0 {
					text += fmt.Sprintf(" / %d", item.Step)
				}
			case 3:
				text = fmt.Sprintf(
					"%d,%d",
					item.Bounds.Min.X,
					item.Bounds.Min.Y,
				)
				if item.Bounds.Dx() != 1 || item.Bounds.Dy() != 1 {
					text += fmt.Sprintf(
						"…%d,%d",
						item.Bounds.Max.X-1,
						item.Bounds.Max.Y-1,
					)
				}
			case 4:
				text = mapImpactStatus(item)
			}
			ui.Text(c, text).SingleLine().Tooltip(text).FontFeatures("tnum")
		}).
			Height(min(220, float32(len(items)+1)*max(28, c.Theme().Space(8)))).
			Label(locales.MapImpactTitle())
		if entry.impactSelected >= 0 && entry.impactSelected < len(items) {
			item := items[entry.impactSelected]
			canJump := jump != nil
			switch item.Kind {
			case maps.ScriptPosition, maps.ScriptPath:
				canJump = v.openScript != nil
			case maps.ChangeArea:
				canJump = openHex != nil
			}
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Textf(c, "0x%08X · %s", item.Address, mapImpactStatus(item)).
					FontFeatures("tnum").
					Selectable()
				ui.Button(c, locales.Jump()).
					Disabled(state.Busy || !canJump).
					OnClick(func() {
						switch item.Kind {
						case maps.UnitStart,
							maps.UnitEnd,
							maps.MovementNode:
							v.OpenDeployment(
								state,
								item.Chapter,
								item.Reference,
							)
							jump(records.Chapters, item.Chapter)
						case maps.TriggerLocation:
							v.mode = 8
							jump(records.Chapters, item.Chapter)
						case maps.TriggerArea:
							v.mode = 3
							jump(records.Chapters, item.Chapter)
						case maps.ScriptPosition, maps.ScriptPath:
							v.openScript(item.Chapter, item.Reference)
						default:
							openHex(item.Address, 12)
						}
					})
			})
		}
	}
	warnings := entry.preview.ImpactWarnings()
	if len(warnings) == 0 {
		return
	}
	ui.Column(c.Key("map-impact-incomplete")).
		Padding(8, 0, 0).
		Gap(6).
		Children(func() {
			ui.Text(c, locales.MapImpactWarnings()).Bold()
			ui.Scroll(c.Key("map-impact-warnings")).
				MaxHeight(160).
				Gap(6).
				Children(func() {
					for _, warning := range warnings {
						ui.Textf(c, "%02X · %s · 0x%08X: %s", warning.Chapter, locales.Label(string(warning.Kind)), warning.Address, components.ErrorText(warning.Err)).
							Tooltip(components.ErrorText(warning.Err))
					}
				})
		})
}

func mapImpactStatus(item maps.Impact) string {
	switch {
	case item.LostCells > 0:
		return fmt.Sprintf(locales.MapImpactNew(), item.LostCells)
	case !item.AfterOutside:
		return locales.MapImpactResolved()
	case item.GainedCells > 0:
		return locales.MapImpactPartial()
	default:
		return locales.MapImpactExisting()
	}
}
