// Package worldmaps owns drafts and navigation for native world-map nodes and routes.
package worldmaps

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type valueInput struct {
	text    string
	invalid bool
}

type nodeState struct {
	draft      *worldmap.Draft
	plan       *worldmap.Plan
	err        error
	inputs     map[worldmap.Field]*valueInput
	name       components.TextReferenceState
	eventsOpen bool
}

func (s *nodeState) invalid() bool {
	for _, field := range s.inputs {
		if field.invalid {
			return true
		}
	}
	return false
}

type Page struct {
	state          *editor.State
	openRecord     func(records.Kind, int)
	openText       func(uint16)
	openShop       func(shop.Key)
	openHex        func(int, int)
	openScript     func(int, int)
	sidebar        components.Sidebar
	query          string
	selected       int
	list           ui.ListState
	rows           []worldmap.Node
	drafts         map[int]*nodeState
	advanced, more bool
	tab            int
	routes         routePage
}

func New(
	state *editor.State,
	record func(records.Kind, int),
	text func(uint16),
	stock func(shop.Key),
	hex func(int, int),
) *Page {
	return &Page{
		state:      state,
		openRecord: record,
		openText:   text,
		openShop:   stock,
		openHex:    hex,
		selected:   -1,
		drafts:     make(map[int]*nodeState),
		routes: routePage{
			selected:   -1,
			nodeFilter: -1,
			drafts:     make(map[int]*routeState),
		},
	}
}

func (p *Page) Reset() {
	sidebar, openScript := p.sidebar, p.openScript
	*p = *New(p.state, p.openRecord, p.openText, p.openShop, p.openHex)
	p.sidebar, p.openScript = sidebar, openScript
}

func (p *Page) Open(id int) {
	p.tab = 0
	p.query = ""
	if p.refresh() != nil {
		return
	}
	for index, node := range p.rows {
		if node.ID == id {
			p.selected = index
			p.list.ScrollTo(index, ui.Center)
			return
		}
	}
}

func (p *Page) caption(node worldmap.Node) string {
	name, _ := p.state.Session.Texts.Value(node.Name)
	return fmt.Sprintf(
		"%02X · %s",
		node.ID,
		strings.Join(strings.Fields(name.Plain), " "),
	)
}

func (p *Page) refresh() error {
	selected := -1
	if p.selected >= 0 && p.selected < len(p.rows) {
		selected = p.rows[p.selected].ID
	}
	nodes, err := p.state.Session.WorldMap.Nodes()
	p.rows = nil
	query := strings.ToLower(strings.TrimSpace(p.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 8)
	for _, node := range nodes {
		if query == "" ||
			strings.Contains(strings.ToLower(p.caption(node)), query) ||
			idErr == nil && int(id) == node.ID {
			p.rows = append(p.rows, node)
		}
	}
	p.selected = -1
	for index, node := range p.rows {
		if node.ID == selected {
			p.selected = index
			break
		}
	}
	if p.selected < 0 && len(p.rows) > 0 {
		p.selected = 0
	}
	return err
}

func (p *Page) View(c *ui.Context) {
	ui.Column(c.Key("worldmap")).Grow(1).Children(func() {
		ui.Tabs(c.Key("mode"), &p.tab, locales.WorldmapNodes(), locales.WorldmapRoutes()).
			Shrink(0).
			Disabled(p.state.Busy)
		if p.tab == 1 {
			p.viewRoutes(c)
		} else {
			p.activateRoute(nil)
			p.viewNodes(c)
		}
	})
}

func (p *Page) viewNodes(c *ui.Context) {
	if !p.state.Session.WorldMap.Available() {
		ui.Text(c, locales.WorldmapUnavailable()).Padding(16)
		return
	}
	err := p.refresh()
	p.sidebar.Render(c, 260, func() {
		ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
			components.TextDraft(
				p.state,
				ui.TextInput(c.Key("search"), &p.query).
					Label(locales.WorldmapSearch()).
					Placeholder(locales.WorldmapSearch()).
					Disabled(p.state.Busy).
					OnChange(func() { p.list = ui.ListState{} }),
			)
			p.list.Selected = &p.selected
			p.list.Key = func(index int) any { return p.rows[index].ID }
			ui.Table(c.Key("nodes"), &p.list, []ui.TableColumn{{ID: "node", Title: locales.WorldmapNodes()}}, len(p.rows), func(row, _ int) {
				node := p.rows[row]
				caption := p.caption(node)
				if draft := p.drafts[node.ID]; draft != nil &&
					draft.draft != nil &&
					(draft.draft.Modified() || draft.invalid()) {
					caption += " *"
				}
				ui.Text(c, caption).SingleLine().FontSize(13).Tooltip(caption)
			}).
				Grow(1).
				Label(locales.WorldmapNodes()).
				Disabled(p.state.Busy)
			if err != nil {
				ui.Text(c, components.ErrorText(err)).
					TextColor(c.Theme().Danger)
			}
		})
	}, func() { p.detail(c) })
}

func (p *Page) draft(id int) *nodeState {
	entry := p.drafts[id]
	if entry != nil && entry.draft != nil &&
		!entry.draft.Current(p.state.Session.WorldMap) &&
		!entry.draft.Modified() &&
		!entry.invalid() {
		entry = nil
	}
	if entry == nil {
		entry = &nodeState{inputs: make(map[worldmap.Field]*valueInput)}
		entry.draft, entry.err = p.state.Session.WorldMap.NewDraft(id)
		p.drafts[id] = entry
	}
	return entry
}

func (p *Page) detail(c *ui.Context) {
	if p.selected < 0 || p.selected >= len(p.rows) {
		return
	}
	node := p.rows[p.selected]
	entry := p.draft(node.ID)
	reload := func() { delete(p.drafts, node.ID) }
	ui.Scroll(c.Key(entry)).
		Fill().
		MinWidth(0).
		Padding(16).
		Gap(12).
		Children(func() {
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.caption(node)).
					FontSize(20).
					Bold().
					Tooltip(locales.WorldmapScope())
				ui.Checkbox(c, &p.advanced, locales.Advanced())
			})
			if entry.draft == nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
				ui.Button(c, locales.QuoteReload()).
					Disabled(p.state.Busy).
					OnClick(reload)
				return
			}
			e := p.state.Session.WorldMap
			stale := !entry.draft.Current(e)
			disabled := p.state.Busy || stale
			if stale {
				entry.plan = nil
				ui.Text(c, locales.WorldmapStale()).TextColor(c.Theme().Danger)
			}
			if entry.draft.Modified() || entry.invalid() || stale {
				ui.Row(c).Wrap().Gap(8).Children(func() {
					ui.Button(c.Key("preview"), locales.WorldmapPreview()).
						Disabled(disabled || entry.invalid() || !entry.draft.Modified()).
						OnClick(func() { entry.plan, entry.err = e.Plan(entry.draft) })
					ui.Button(c, locales.DeployReload()).
						Disabled(p.state.Busy).
						OnClick(reload)
				})
			}
			if entry.invalid() {
				ui.Text(c, locales.SupportInvalidInput()).
					FontSize(12).
					TextColor(c.Theme().Danger)
			}
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			if entry.plan != nil {
				p.preview(c, entry, reload)
			}
			value := entry.draft.Node()
			ui.Column(c.Key("name")).Gap(4).Children(func() {
				ui.Text(c, locales.WorldmapName()).FontSize(13).Bold()
				components.TextReference(
					c,
					p.state,
					&entry.name,
					uint32(value.Name),
					locales.WorldmapName(),
					p.advanced,
					disabled,
					func(id uint16) {
						entry.plan = nil
						entry.err = entry.draft.Set(worldmap.Name, int(id))
					},
					p.openText,
				)
			})
			p.fields(c, entry, disabled, worldmap.X, worldmap.Y)
			p.connectedRoutes(c, node.ID)
			p.relatedEvents(
				c,
				event.ScriptWorldMapNodeTarget,
				node.ID,
				&entry.eventsOpen,
			)
			ui.Text(c, locales.WorldmapChapters()).FontSize(13).Bold()
			p.fields(
				c,
				entry,
				disabled,
				worldmap.EirikaChapter,
				worldmap.EphraimChapter,
			)
			ui.Text(c, locales.WorldmapDestinations()).
				FontSize(13).
				Bold().
				Tooltip(locales.WorldmapNextHelp())
			p.fields(
				c,
				entry,
				disabled,
				worldmap.EirikaNext,
				worldmap.EphraimNext,
			)
			p.fields(c, entry, disabled, worldmap.BranchFlag)
			p.fields(
				c,
				entry,
				disabled,
				worldmap.EirikaAlternate,
				worldmap.EphraimAlternate,
			)
			ui.Collapsible(
				c.Key("rules"),
				locales.WorldmapOtherRules(),
				&p.more,
				func() {
					p.fields(
						c,
						entry,
						disabled,
						worldmap.UnclearedIcon,
						worldmap.ClearedIcon,
					)
					p.fields(
						c,
						entry,
						disabled,
						worldmap.Placement,
						worldmap.Encounters,
						worldmap.ShipTravel,
					)
				},
			)
			ui.Row(c.Key("shops")).Wrap().Gap(8).Children(func() {
				for slot, label := range []string{locales.ShopArmory(), locales.ShopVendor(), locales.ShopSecret()} {
					ui.Button(c.Key(slot), label).
						Disabled(p.state.Busy || p.openShop == nil).
						OnClick(func() { p.openShop(shop.Key{Origin: shop.WorldMap, ID: node.ID, Slot: slot}) })
				}
			})
			if p.advanced {
				ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Textf(c, "0x%08X", value.Address).
						Font("monospace").
						FontSize(12)
					ui.Button(c, locales.EditBytes()).
						Disabled(disabled || p.openHex == nil).
						OnClick(func() { p.openHex(value.Address, worldmap.NodeSize) })
				})
			}
		})
}

func fieldLabel(field worldmap.Field) string {
	labels := [...]string{
		locales.WorldmapName(),
		locales.WorldmapX(),
		locales.WorldmapY(),
		locales.WorldmapEirikaChapter(),
		locales.WorldmapEphraimChapter(),
		locales.WorldmapBranchFlag(),
		locales.WorldmapEirikaNext(),
		locales.WorldmapEphraimNext(),
		locales.WorldmapEirikaAlternate(),
		locales.WorldmapEphraimAlternate(),
		locales.WorldmapClearedIcon(),
		locales.WorldmapUnclearedIcon(),
		locales.WorldmapPlacement(),
		locales.WorldmapEncounters(),
		locales.WorldmapShipTravel(),
	}
	return labels[field]
}

func fieldHelp(field worldmap.Field) string {
	switch field {
	case worldmap.EirikaNext,
		worldmap.EphraimNext,
		worldmap.EirikaAlternate,
		worldmap.EphraimAlternate:
		return locales.WorldmapNextHelp()
	case worldmap.BranchFlag:
		return locales.WorldmapFlagHelp()
	case worldmap.ClearedIcon, worldmap.UnclearedIcon:
		return locales.WorldmapIconHelp()
	case worldmap.Placement:
		return locales.WorldmapPlacementHelp()
	case worldmap.Encounters:
		return locales.WorldmapEncountersHelp()
	case worldmap.ShipTravel:
		return locales.WorldmapShipHelp()
	case worldmap.X, worldmap.Y:
		return locales.WorldmapPositionHelp()
	default:
		return locales.WorldmapChapterHelp()
	}
}

func (p *Page) fields(
	c *ui.Context,
	entry *nodeState,
	disabled bool,
	fields ...worldmap.Field,
) {
	ui.Row(c.Key(fields[0])).Wrap().Gap(12).Children(func() {
		for _, field := range fields {
			ui.Column(c.Key(field)).
				Width(210).
				MaxWidthPercent(100).
				Gap(4).
				Children(func() {
					label := fieldLabel(field)
					ui.Text(c, label).FontSize(13)
					value := entry.draft.Node().Value(field)
					inputState := entry.inputs[field]
					if inputState == nil {
						inputState = &valueInput{text: strconv.Itoa(value)}
						entry.inputs[field] = inputState
					}
					options := p.options(field)
					var control ui.Element
					if len(options) > 0 {
						control = ui.Autocomplete(
							c.Key("value"),
							&inputState.text,
							options,
						)
					} else {
						control = ui.TextInput(c.Key("value"), &inputState.text).
							TextAlign(ui.End)
					}
					components.TextDraft(
						p.state,
						control.Label(label).Tooltip(fieldHelp(field)).
							FontFeatures("tnum").
							Disabled(disabled).OnChange(func() {
							entry.plan, entry.err = nil, nil
							text, _, _ := strings.Cut(inputState.text, " · ")
							parsed, err := input.ParseInteger(text)
							minimum, maximum := field.Bounds()
							inputState.invalid = err != nil ||
								parsed < int64(minimum) ||
								parsed > int64(maximum)
							if !inputState.invalid {
								inputState.invalid = entry.draft.Set(
									field,
									int(parsed),
								) != nil
							}
						}),
					)
					if inputState.invalid {
						minimum, maximum := field.Bounds()
						control.Error(fmt.Sprintf("%d…%d", minimum, maximum))
					}
					if field == worldmap.EirikaChapter ||
						field == worldmap.EphraimChapter {
						ui.Button(c, locales.WorldmapOpenChapter()).
							Disabled(p.state.Busy || inputState.invalid || p.openRecord == nil || !p.chapterExists(value)).
							OnClick(func() { p.openRecord(records.Chapters, value) })
					} else if field >= worldmap.EirikaNext &&
						field <= worldmap.EphraimAlternate {
						ui.Button(c, locales.WorldmapOpenNode()).
							Disabled(p.state.Busy || inputState.invalid || value < 0 || value >= worldmap.NodeCount).
							OnClick(func() { p.Open(value) })
					}
				})
		}
	})
}

func (p *Page) chapterExists(id int) bool {
	if table := p.state.Session.Records.Table(
		records.Chapters,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID == id {
				return true
			}
		}
	}
	return false
}

func (p *Page) options(field worldmap.Field) []string {
	var options []string
	if field == worldmap.EirikaChapter || field == worldmap.EphraimChapter {
		if table := p.state.Session.Records.Table(
			records.Chapters,
		); table != nil &&
			table.Err == nil {
			for _, row := range table.Rows {
				options = append(
					options,
					fmt.Sprintf("0x%02X · %s", row.ID, row.Name),
				)
			}
		}
	} else if field >= worldmap.EirikaNext && field <= worldmap.EphraimAlternate {
		options = append(options, "-1 · "+locales.WorldmapNoNext())
		nodes, _ := p.state.Session.WorldMap.Nodes()
		for _, node := range nodes {
			name, _ := p.state.Session.Texts.Value(node.Name)
			options = append(
				options,
				fmt.Sprintf(
					"0x%02X · %s",
					node.ID,
					strings.Join(strings.Fields(name.Plain), " "),
				),
			)
		}
	}
	return options
}

func (p *Page) preview(c *ui.Context, entry *nodeState, reload func()) {
	plan := entry.plan
	ui.Column(c.Key("preview")).
		Gap(6).
		Padding(8, 0).
		BorderWidth(1, 0).
		BorderColor(c.Theme().Border).
		Children(func() {
			ui.Text(c, locales.WorldmapApplyHelp()).FontSize(12)
			for _, change := range plan.Changes() {
				ui.Textf(c, "%s: %d → %d", fieldLabel(change.Field), change.Before, change.After).
					FontSize(13)
			}
			ui.PrimaryButton(c, locales.WorldmapApply()).
				Disabled(p.state.Busy || entry.invalid() || !plan.Current(p.state.Session.WorldMap)).
				OnClick(func() {
					entry.err = p.state.Session.WorldMap.Apply(plan)
					entry.plan = nil
					if entry.err == nil {
						p.state.HistoryEpoch++
						reload()
					}
				})
		})
}
