package records

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type terrainInput struct {
	text    string
	invalid bool
}

type terrainState struct {
	draft      *terrain.Draft
	plan       *terrain.Plan
	err        error
	query      string
	inputs     map[string]*terrainInput
	generation int
}

func (v *terrainState) invalid() bool {
	for _, field := range v.inputs {
		if field.invalid {
			return true
		}
	}
	return false
}

func terrainColumn(
	kind terrain.Kind,
) string {
	return locales.Label("terrain_column_" + string(kind))
}

func (a *form) profileTerrain(
	c *ui.Context,
	row records.Row,
	entry *profileState,
) {
	v := &entry.terrain
	editor := a.state.Session.Terrain
	if v.draft != nil && !v.draft.Current(editor) && !v.draft.Modified() &&
		!v.invalid() {
		*v = terrainState{query: v.query, generation: v.generation + 1}
	}
	if v.inputs == nil {
		v.inputs = make(map[string]*terrainInput)
		v.draft, v.err = editor.NewDraft(row.ID)
	}
	reload := func() { *v = terrainState{query: v.query, generation: v.generation + 1} }
	ui.Column(c.Key(fmt.Sprintf("terrain/%d", v.generation))).
		Gap(10).
		Children(func() {
			if v.draft == nil {
				ui.Text(c, components.ErrorText(v.err)).
					TextColor(c.Theme().Danger)
				ui.Button(c, locales.TerrainReload()).
					Disabled(a.state.Busy).
					OnClick(reload)
				return
			}
			current := v.draft.Current(editor)
			if !current {
				v.plan = nil
				ui.Text(c, locales.TerrainStale()).
					TextColor(c.Theme().Danger).
					FontSize(13)
			}
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				components.TextDraft(
					a.state,
					ui.TextInput(c.Key("search"), &v.query).
						Width(190).
						Height(26).
						FontSize(13).
						Label(locales.TerrainSearch()).
						Placeholder(locales.TerrainSearch()).
						Tooltip(locales.TerrainNamesHelp()),
				)
				ui.Text(c, locales.TerrainHelp()).
					FontSize(12).
					TextColor(c.Theme().TextMuted).
					Tooltip(locales.TerrainScope())
			})
			if v.draft.Modified() || v.invalid() || !current {
				ui.Row(c.Key("actions")).
					Wrap().
					Gap(8).
					AlignItems(ui.Center).
					Children(func() {
						ui.Button(c, locales.TerrainPreview()).
							Disabled(a.state.Busy || !current || v.invalid() || !v.draft.Modified()).
							OnClick(func() { v.plan, v.err = editor.Plan(v.draft) })
						ui.Button(c, locales.TerrainReload()).
							Disabled(a.state.Busy).
							OnClick(reload)
						if v.invalid() {
							ui.Text(c, locales.TerrainRange()).
								FontSize(12).
								TextColor(c.Theme().Danger)
						}
					})
			}
			if v.err != nil {
				ui.Text(c, components.ErrorText(v.err)).
					TextColor(c.Theme().Danger)
			}
			if v.plan != nil {
				a.terrainPreview(c, v, reload)
			}
			tables := v.draft.Tables()
			for _, table := range tables {
				if table.Err != nil || table.Empty {
					message := locales.TerrainEmpty()
					if table.Err != nil {
						message = components.ErrorText(table.Err)
					}
					ui.Text(c, terrainColumn(table.Kind)+": "+message).
						FontSize(12).
						TextColor(c.Theme().TextMuted)
				}
			}
			format := editor.Format()
			width, _ := c.Size()
			available := max(float32(300), width-a.sidebar-40)
			ui.ScrollHorizontal(c.Key("tables")).
				Width(available).
				MaxWidthPercent(100).
				Children(func() {
					tracks := []ui.Track{ui.Fixed(154)}
					for range tables {
						tracks = append(tracks, ui.Fr(1))
					}
					ui.Grid(c).
						Width(max(available, 154+float32(len(tables))*80)).
						ColumnTracks(tracks...).
						GapX(8).
						GapY(4).
						AlignItems(ui.Center).
						Children(func() {
							ui.Text(c, locales.TerrainName()).
								FontSize(13).
								Bold()
							for _, table := range tables {
								help := locales.TerrainBonusHelp()
								if table.Kind.Movement() {
									help = locales.TerrainMovementHelp()
								}
								ui.Text(c, terrainColumn(table.Kind)).
									FontSize(12).
									TextAlign(ui.End).
									PaddingX(6).
									Tooltip(help)
							}
							query := strings.ToLower(strings.TrimSpace(v.query))
							for id := range format.Count() {
								name := locales.Label(format.NameKey(id))
								if query != "" &&
									!strings.Contains(
										strings.ToLower(name),
										query,
									) &&
									!strings.EqualFold(
										query,
										fmt.Sprintf("%02X", id),
									) &&
									!strings.EqualFold(
										query,
										fmt.Sprintf("0x%02X", id),
									) {
									continue
								}
								ui.Textf(c, "%02X  %s", id, name).
									FontSize(13).
									SingleLine().
									Tooltip(name)
								for _, table := range tables {
									if table.Empty || table.Err != nil {
										ui.Text(c, "—").
											TextAlign(ui.End).
											PaddingX(6).
											FontSize(13)
										continue
									}
									a.terrainField(
										c,
										v,
										table,
										id,
										name,
										!current,
									)
								}
							}
						})
				})
			if a.showRaw {
				ui.Column(c.Key("raw")).Gap(4).Children(func() {
					for _, table := range tables {
						ui.Row(c.Key(table.Kind)).
							Wrap().
							Gap(8).
							AlignItems(ui.Center).
							Children(func() {
								ui.Textf(c, "%s  0x%08X → 0x%08X", terrainColumn(table.Kind), table.Pointer, table.Address).
									Font("monospace").
									FontSize(12)
								ui.Button(c, locales.EditBytes()).
									Height(26).
									Disabled(a.state.Busy || table.Err != nil || table.Empty || a.openHex == nil).
									OnClick(func() { a.openHex(table.Address, format.Count()) })
							})
					}
				})
			}
		})
}

func (a *form) terrainField(
	c *ui.Context,
	v *terrainState,
	table terrain.Table,
	id int,
	name string,
	stale bool,
) {
	key := fmt.Sprintf("%s/%d", table.Kind, id)
	field := v.inputs[key]
	if field == nil {
		field = &terrainInput{text: strconv.Itoa(table.Values[id])}
		v.inputs[key] = field
	}
	help := locales.TerrainBonusHelp()
	if table.Kind.Movement() {
		help = locales.TerrainMovementHelp()
	}
	if table.Kind.Movement() && table.Values[id] == -1 {
		help = locales.TerrainImpassable() + "\n" + help
	}
	if a.showRaw {
		help += fmt.Sprintf(
			"\n0x%08X  0x%02X",
			table.Address+id,
			table.Values[id]&0xFF,
		)
	}
	control := components.TextDraft(
		a.state,
		ui.TextInput(c.Key(key), &field.text).
			Height(26).
			Padding(3, 6).FontSize(13).FontFeatures("tnum").TextAlign(ui.End).
			Label(fmt.Sprintf("%02X %s / %s", id, name, terrainColumn(table.Kind))).
			Tooltip(help).
			Disabled(a.state.Busy || stale).
			OnChange(func() {
				v.plan = nil
				value, err := input.ParseInteger(field.text)
				field.invalid = err != nil || value < -128 || value > 127
				if !field.invalid {
					v.err = v.draft.Set(table.Kind, id, int(value))
				}
			}),
	)
	if field.invalid {
		control.Error(locales.TerrainRange()).Tooltip(locales.TerrainRange())
	}
}

func (a *form) terrainPreview(c *ui.Context, v *terrainState, reload func()) {
	ui.Column(c.Key("preview")).
		Gap(8).
		Padding(8, 0).
		BorderWidth(1, 0).
		BorderColor(c.Theme().Border).
		Children(func() {
			for _, move := range v.plan.Moves() {
				ui.Text(c, terrainColumn(move.Kind)).FontSize(13).Bold()
				ui.Row(c).Wrap().Gap(12).Children(func() {
					for _, change := range move.Changes {
						ui.Textf(c, "%02X %s: %d → %d", change.Terrain, locales.Label(a.state.Session.Terrain.Format().NameKey(change.Terrain)), change.Before, change.After).
							FontSize(12)
					}
				})
				if len(move.Shared) > 0 {
					ui.Text(c, locales.TerrainShared()).
						FontSize(12).
						Tooltip(locales.TerrainSharedHelp())
					ui.Scroll(c.Key(move.Kind)).
						MaxHeight(100).
						Gap(2).
						Children(func() {
							for _, ref := range move.Shared {
								ui.Textf(c, "%02X %s / %s", ref.Class.ID, ref.Class.Name, terrainColumn(ref.Kind)).
									FontSize(12)
							}
						})
				}
				if a.showRaw {
					ui.Textf(c, "0x%08X → 0x%08X", move.Address, move.Destination).
						Font("monospace").
						FontSize(12)
				}
			}
			ui.Text(c, locales.TerrainApplyHelp()).FontSize(12)
			ui.Button(c, locales.TerrainApply()).
				Disabled(a.state.Busy || v.invalid() || !v.plan.Current(a.state.Session.Terrain)).
				OnClick(func() {
					v.err = a.state.Session.Terrain.Apply(v.plan)
					if v.err == nil {
						a.state.Session.Refresh()
						reload()
					}
				})
		})
}
