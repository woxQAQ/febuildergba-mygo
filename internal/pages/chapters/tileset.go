package chapters

import (
	"fmt"
	"image"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type tileDefinitionInputs struct {
	terrain numberDraft
	parts   [4][2]numberDraft
}

type tilesetState struct {
	mapPainter

	data                    *maps.TilesetDraft
	preview                 *maps.TilesetPlan
	inputs                  map[int]*tileDefinitionInputs
	selected                int
	choice                  string
	choices                 []string
	revision                uint64
	address                 int
	err, mapErr, tilesetErr error
	editFocus, reviewFocus  ui.Handle
	historyFocus            ui.Handle
	changes                 ui.ListState
}

func (s *tileDefinitionInputs) invalid() bool {
	if s.terrain.invalid {
		return true
	}
	for _, part := range s.parts {
		if part[0].invalid || part[1].invalid {
			return true
		}
	}
	return false
}

func (s *tilesetState) invalid() bool {
	for _, definition := range s.inputs {
		if definition.invalid() {
			return true
		}
	}
	return false
}

func (v *Page) tilesetDraft(
	state *editor.State,
	row records.Row,
) *tilesetState {
	entry := v.tilesetDrafts[row.ID]
	if entry != nil {
		stale := entry.revision != state.Session.Document.State().Revision ||
			entry.address != row.Address
		if entry.data != nil {
			stale = !entry.data.Current(state.Session.Maps)
		}
		if stale &&
			(entry.data == nil || (!entry.data.Modified() && !entry.invalid())) {
			entry = nil
		}
	}
	if entry == nil {
		entry = &tilesetState{
			inputs:   make(map[int]*tileDefinitionInputs),
			revision: state.Session.Document.State().Revision,
			address:  row.Address,
		}
		entry.data, entry.err = state.Session.Maps.NewTilesetDraft(row.ID)
		if entry.err == nil {
			for index := range entry.data.Count() {
				entry.choices = append(
					entry.choices,
					fmt.Sprintf("0x%03X", index),
				)
			}
			entry.refresh()
		}
		v.tilesetDrafts[row.ID] = entry
	}
	return entry
}

func (s *tilesetState) refresh() {
	s.preview = nil
	s.tileset, s.tilesetErr = s.data.Tileset()
	s.image, s.mapErr = s.data.Preview()
	s.bitmap = nil
	if s.tilesetErr == nil {
		s.bitmap = ui.NewBitmap(s.tileset.Image).Pixelated()
	}
	s.selectDefinition(s.selected)
}

func (s *tilesetState) selectDefinition(index int) {
	if index < 0 || index >= s.data.Count() {
		return
	}
	s.selected, s.choice = index, fmt.Sprintf("0x%03X", index)
	definition, _ := s.data.Definition(index)
	s.tile = maps.Cell{
		Tile:    uint16(index * 4),
		Terrain: byte(definition.Terrain),
	}
	s.brush = nil
	for i, cell := range s.tileset.Cells {
		if cell.Tile == s.tile.Tile {
			s.choose(s.tileset, image.Pt(i%s.tileset.Width, i/s.tileset.Width))
			break
		}
	}
}

func (v *Page) tilesetEditor(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	jump func(records.Kind, int),
) {
	entry := v.tilesetDraft(state, row)
	reload := func() {
		selected := entry.selected
		delete(v.tilesetDrafts, row.ID)
		next := v.tilesetDraft(state, row)
		if next.data != nil {
			next.selectDefinition(min(selected, next.data.Count()-1))
		}
		next.editFocus.Focus(c)
	}
	ui.Column(c.Key(entry)).Bind(&entry.historyFocus).Gap(12).Children(func() {
		if entry.data == nil {
			ui.Text(c, components.ErrorText(entry.err)).
				TextColor(c.Theme().Danger)
			ui.Button(c, locales.MapReload()).
				Disabled(state.Busy).
				OnClick(reload)
			return
		}
		stale := !entry.data.Current(state.Session.Maps)
		disabled := state.Busy || stale
		// Configuration fields use native text history. The map/grid has no
		// local undo, so it must not undo an unrelated ROM edit while focused.
		entry.historyActive = entry.historyFocus.FocusWithin(c) ||
			(entry.historyActive && !c.Root().FocusWithin())
		if entry.historyActive {
			state.DraftHistory = &editor.History{}
		}
		if stale {
			entry.preview = nil
			ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
		}
		if entry.invalid() {
			ui.Text(c, locales.SupportInvalidInput()).
				TextColor(c.Theme().Danger)
			ui.Row(c.Key("invalid-definitions")).Wrap().Gap(6).Children(func() {
				for index := range entry.data.Count() {
					if fields := entry.inputs[index]; fields != nil &&
						fields.invalid() {
						ui.Button(c.Key(index), fmt.Sprintf("0x%03X", index)).
							Disabled(state.Busy).
							OnClick(func() { entry.selectDefinition(index); entry.editFocus.Focus(c) })
					}
				}
			})
		}
		if entry.err != nil {
			ui.Text(c, components.ErrorText(entry.err)).
				TextColor(c.Theme().Danger)
		}
		if entry.preview != nil {
			v.tilesetReview(c, state, entry, jump, reload)
			return
		}
		ui.Row(c.Key("actions")).
			Wrap().
			Gap(8).
			AlignItems(ui.Center).
			Children(func() {
				ui.Button(c.Key("preview"), locales.MapTilesetPreview()).
					Disabled(disabled || entry.invalid() || !entry.data.Modified()).
					OnClick(func() {
						entry.preview, entry.err = state.Session.Maps.PlanTileset(
							entry.data,
						)
						if entry.err == nil {
							entry.reviewFocus.Focus(c)
						}
					})
				ui.Button(c.Key("reload"), locales.MapReload()).
					Disabled(state.Busy).
					OnClick(reload)
			})
		ui.Text(c, locales.MapTilesetHelp()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		ui.Row(c.Key("selection")).
			Wrap().
			Gap(12).
			AlignItems(ui.Center).
			Children(func() {
				ui.Text(c, locales.MapDefinitionId()).FontSize(13)
				ui.Select(c.Key("definition"), &entry.choice, entry.choices).
					Bind(&entry.editFocus).
					Label(locales.MapDefinitionId()).Width(140).
					Disabled(state.Busy).
					OnChange(func() {
						if index, err := strconv.ParseInt(
							entry.choice,
							0,
							32,
						); err == nil {
							entry.selectDefinition(int(index))
						}
					})
				if entry.brush != nil {
					ui.Image(c.Key("selected-image"), entry.brush).
						Size(64, 64).
						Shrink(0)
				}
			})
		ui.Column(c.Key(entry.selected)).Gap(8).Children(func() {
			entry.definitionFields(c, state, disabled)
		})
		ui.Row(c.Key("previews")).
			Wrap().
			Gap(16).
			AlignItems(ui.Start).
			Children(func() {
				ui.Column(c.Key("tileset")).
					Width(514).
					MaxWidthPercent(100).
					Gap(6).
					Children(func() {
						if entry.tilesetErr != nil {
							ui.Text(c, components.ErrorText(entry.tilesetErr)).
								TextColor(c.Theme().Danger)
						} else {
							entry.palette(c, state.Busy)
							if index := int(
								entry.tile.Tile >> 2,
							); index != entry.selected {
								entry.selectDefinition(index)
							}
						}
					})
				ui.Column(c.Key("map")).
					Width(360).
					Grow(1).
					MaxWidthPercent(100).
					Children(func() {
						entry.view.Render(
							c,
							entry.image,
							entry.mapErr,
							mapview.Overlay{Choose: func(point image.Point) {
								if !state.Busy {
									if cell, ok := entry.image.Cell(
										point.X,
										point.Y,
									); ok {
										entry.selectDefinition(
											int(cell.Tile >> 2),
										)
									}
								}
							}},
						)
					})
			})
		ui.Text(c, locales.MapTilesetDrafts()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	})
}

func (s *tilesetState) definitionFields(
	c *ui.Context,
	state *editor.State,
	disabled bool,
) {
	index := s.selected
	definition, _ := s.data.Definition(index)
	fields := s.inputs[index]
	if fields == nil {
		fields = &tileDefinitionInputs{
			terrain: numberDraft{text: strconv.Itoa(definition.Terrain)},
		}
		for i, part := range definition.Parts {
			fields.parts[i] = [2]numberDraft{
				{text: strconv.Itoa(part.Tile)},
				{text: strconv.Itoa(part.Palette)},
			}
		}
		s.inputs[index] = fields
	}
	ui.Row(c.Key("terrain")).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.MapTerrainId()).FontSize(13)
		s.definitionNumber(
			c,
			state,
			&fields.terrain,
			locales.MapTerrainId(),
			0,
			255,
			disabled,
			func(value int) error { return s.data.SetTerrain(index, value) },
		)
	})
	ui.Row(c.Key("parts")).Wrap().Gap(12).Children(func() {
		for i, part := range definition.Parts {
			ui.Column(c.Key(i)).
				Width(250).
				MaxWidthPercent(100).
				Gap(4).
				Children(func() {
					name := tilePartName(i)
					ui.Text(c, name).FontSize(13).Bold()
					ui.Row(c.Key("tile")).
						Gap(8).
						AlignItems(ui.Center).
						Children(func() {
							ui.Text(c, locales.MapPartTile()).
								FontSize(13).
								Grow(1)
							s.definitionNumber(
								c,
								state,
								&fields.parts[i][0],
								name+" / "+locales.MapPartTile(),
								0,
								min(1023, max(0, s.data.GraphicsCount()-1)),
								disabled,
								func(value int) error {
									current, _ := s.data.Definition(index)
									current.Parts[i].Tile = value
									return s.data.SetPart(
										index,
										i,
										current.Parts[i],
									)
								},
							)
						})
					ui.Row(c.Key("palette")).
						Gap(8).
						AlignItems(ui.Center).
						Children(func() {
							ui.Text(c, locales.MapPartPalette()).
								FontSize(13).
								Grow(1)
							s.definitionNumber(
								c,
								state,
								&fields.parts[i][1],
								name+" / "+locales.MapPartPalette(),
								0,
								s.data.PaletteCount()-1,
								disabled,
								func(value int) error {
									current, _ := s.data.Definition(index)
									current.Parts[i].Palette = value
									return s.data.SetPart(
										index,
										i,
										current.Parts[i],
									)
								},
							)
						})
					ui.Row(c.Key("flips")).Wrap().Gap(8).Children(func() {
						ui.Checkbox(c.Key("x"), &part.FlipX, locales.MapPartFlipX()).
							Label(name + " / " + locales.MapPartFlipX()).
							Disabled(disabled).
							OnClick(func() {
								s.preview = nil
								s.err = s.data.SetPart(index, i, part)
								if s.err == nil {
									s.refresh()
								}
							})
						ui.Checkbox(c.Key("y"), &part.FlipY, locales.MapPartFlipY()).
							Label(name + " / " + locales.MapPartFlipY()).
							Disabled(disabled).
							OnClick(func() {
								s.preview = nil
								s.err = s.data.SetPart(index, i, part)
								if s.err == nil {
									s.refresh()
								}
							})
					})
				})
		}
	})
}

func (s *tilesetState) definitionNumber(
	c *ui.Context,
	state *editor.State,
	field *numberDraft,
	label string,
	minimum, maximum int,
	disabled bool,
	set func(int) error,
) {
	control := components.TextDraft(
		state,
		ui.TextInput(c.Key("value"), &field.text).
			Width(82).
			TextAlign(ui.End).
			FontFeatures("tnum").Label(label).Disabled(disabled).
			OnChange(func() {
				s.preview, s.err = nil, nil
				parsed, err := input.ParseInteger(field.text)
				field.invalid = err != nil || parsed < int64(minimum) ||
					parsed > int64(maximum)
				if !field.invalid {
					field.invalid = set(int(parsed)) != nil
				}
				if !field.invalid {
					s.refresh()
				}
			}),
	)
	if field.invalid {
		control.Error(fmt.Sprintf("%d…%d", minimum, maximum))
	}
}

func tilePartName(part int) string {
	return [...]string{locales.MapPartTopLeft(), locales.MapPartTopRight(), locales.MapPartBottomLeft(), locales.MapPartBottomRight()}[part]
}

func tileFlipValue(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (v *Page) tilesetReview(
	c *ui.Context,
	state *editor.State,
	entry *tilesetState,
	jump func(records.Kind, int),
	reload func(),
) {
	plan := entry.preview
	ui.Column(c.Key("review")).Gap(12).Children(func() {
		ui.Text(c, locales.MapTilesetPreview()).FontSize(16).Bold()
		type changeRow struct {
			index                int
			field, before, after string
		}
		var rows []changeRow
		for _, change := range plan.Changes() {
			if change.Before.Terrain != change.After.Terrain {
				rows = append(
					rows,
					changeRow{
						change.Index,
						locales.MapTerrainId(),
						strconv.Itoa(change.Before.Terrain),
						strconv.Itoa(change.After.Terrain),
					},
				)
			}
			for part, after := range change.After.Parts {
				before := change.Before.Parts[part]
				for _, field := range []struct{ label, before, after string }{
					{locales.MapPartTile(), strconv.Itoa(before.Tile), strconv.Itoa(after.Tile)},
					{locales.MapPartPalette(), strconv.Itoa(before.Palette), strconv.Itoa(after.Palette)},
					{locales.MapPartFlipX(), tileFlipValue(before.FlipX), tileFlipValue(after.FlipX)},
					{locales.MapPartFlipY(), tileFlipValue(before.FlipY), tileFlipValue(after.FlipY)},
				} {
					if field.before != field.after {
						rows = append(
							rows,
							changeRow{
								change.Index,
								tilePartName(part) + " / " + field.label,
								field.before,
								field.after,
							},
						)
					}
				}
			}
		}
		ui.Table(c.Key("changes"), &entry.changes, []ui.TableColumn{
			{
				ID:    "id",
				Title: locales.MapDefinitionId(),
				Width: 100,
			},
			{ID: "part", Title: locales.MapTilesetPart(), MinWidth: 230},
			{ID: "before", Title: locales.Before(), Width: 100},
			{ID: "after", Title: locales.After(), Width: 100},
		}, len(rows), func(index, column int) {
			row := rows[index]
			values := [4]string{
				fmt.Sprintf("0x%03X", row.index),
				row.field,
				row.before,
				row.after,
			}
			ui.Text(c, values[column]).FontSize(13).Tooltip(values[column])
		}).Height(float32(min(270, max(72, (len(rows)+1)*32)))).MaxWidthPercent(100)
		mapUses(c, state, plan.Uses(), locales.MapTilesetScope(), jump)
		ui.Text(c, locales.MapTilesetScope()).FontSize(12)
		ui.Textf(c, "0x%08X → 0x%08X · %d B", plan.Source(), plan.Destination(), plan.Length()).
			FontFeatures("tnum").
			FontSize(12).
			Selectable()
		ui.Row(c.Key("actions")).Wrap().Gap(8).Children(func() {
			ui.PrimaryButton(c.Key("apply"), locales.MapTilesetApply()).
				Disabled(state.Busy || entry.invalid() || !plan.Current(state.Session.Maps)).
				OnClick(func() {
					entry.err = state.Session.Maps.ApplyTileset(plan)
					if entry.err == nil {
						state.HistoryEpoch++
						reload()
					}
				})
			ui.Button(c.Key("back"), locales.MapBackToEdit()).
				Bind(&entry.reviewFocus).
				Disabled(state.Busy).
				OnClick(func() { entry.preview = nil; entry.editFocus.Focus(c) })
		})
	})
}
