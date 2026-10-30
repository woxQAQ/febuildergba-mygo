// Package chapters shares a chapter's map, deployment selection and drafts.
package chapters

import (
	"fmt"
	"image"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type entryDraft struct {
	data                   *deployment.Draft
	preview                *deployment.Edit
	selected, lastSelected int
	list                   ui.ListState
	inputs                 map[inputKey]*numberDraft
	pathMode               int
	pathUnitChoice         string
	paths                  map[int]*pathState
}

type chapterSelection struct {
	choice  string
	pointer int
}

type Page struct {
	baseMap                maps.Preview
	mapErr                 error
	mapChapter, mapAddress int
	session                *game.Session
	loaded                 bool
	revision               uint64
	chapter, address       int
	resource               event.Deployments
	err                    error
	mode, positionMode     int
	mapView                mapview.View
	entries                map[int]*entryDraft
	selections             map[int]*chapterSelection
	events                 map[eventKey]*eventState
	openScript             func(int, int)
	mapDrafts              map[int]*mapState
	changeDrafts           map[int]*mapChangesState
}

func (v *Page) OpenDeployment(state *editor.State, chapter, site int) {
	table := state.Session.Records.Table(records.Chapters)
	if table == nil || table.Err != nil {
		state.Err = edit.ErrLayout
		return
	}
	for _, row := range table.Rows {
		if row.ID == chapter {
			v.load(state, row)
			v.mode = 1
			v.selections[chapter] = &chapterSelection{pointer: site}
			return
		}
	}
	state.Err = edit.ErrField
}

func (v *Page) Configuration() bool { return v.mode == 0 }

func (v *Page) load(state *editor.State, row records.Row) {
	s := state.Session
	if v.session != s {
		*v = Page{
			session:      s,
			entries:      make(map[int]*entryDraft),
			selections:   make(map[int]*chapterSelection),
			events:       make(map[eventKey]*eventState),
			mapDrafts:    make(map[int]*mapState),
			changeDrafts: make(map[int]*mapChangesState),
		}
	}
	s.Refresh()
	revision := s.Document.State().Revision
	if !v.loaded || !v.resource.Current(s.Events) || row.ID != v.chapter ||
		row.Address != v.address {
		v.resource, v.err = s.Events.Deployments(row.ID)
		v.loaded, v.revision, v.chapter, v.address = true, revision, row.ID, row.Address
	}
}

func (v *Page) draft(
	state *editor.State,
	group deployment.Group,
) *entryDraft {
	entry := v.entries[group.Pointer]
	if entry != nil && !entry.data.Modified() && !entry.invalid() &&
		!entry.data.Current(state.Session.Deployments) {
		delete(v.entries, group.Pointer)
		entry = nil
	}
	if entry == nil {
		data, err := state.Session.Deployments.NewDraft(
			v.chapter,
			group.Pointer,
		)
		if err != nil {
			state.Err = err
			return nil
		}
		entry = &entryDraft{data: data, lastSelected: -1}
		v.entries[group.Pointer] = entry
	}
	return entry
}

func (v *Page) caption(group deployment.Group) string {
	count, suffix := len(group.Units), ""
	if entry := v.entries[group.Pointer]; entry != nil {
		count = entry.data.Count()
		if entry.data.Modified() {
			suffix = " *"
		}
	}
	return fmt.Sprintf(
		"%s · %d · 0x%08X%s",
		locales.Label(group.Label),
		count,
		group.Pointer,
		suffix,
	)
}

func (v *Page) group(
	c *ui.Context,
	state *editor.State,
) (deployment.Group, bool) {
	if v.err != nil {
		ui.Text(c, components.ErrorText(v.err)).
			Tooltip(v.err.Error()).
			TextColor(c.Theme().Danger)
		return deployment.Group{}, false
	}
	if len(v.resource.Groups) == 0 {
		return deployment.Group{}, false
	}
	selection := v.selections[v.chapter]
	if selection == nil {
		selection = &chapterSelection{pointer: v.resource.Groups[0].Pointer}
		for _, group := range v.resource.Groups {
			if group.Err == nil && len(group.Units) > 0 {
				selection.pointer = group.Pointer
				break
			}
		}
		v.selections[v.chapter] = selection
	}
	options := make([]string, len(v.resource.Groups))
	selected := 0
	for i, group := range v.resource.Groups {
		options[i] = v.caption(group)
		if group.Pointer == selection.pointer {
			selected = i
		}
	}
	if !slices.Contains(options, selection.choice) {
		selection.choice = options[selected]
	}
	ui.Select(c.Key("deployment-entry"), &selection.choice, options).
		Label(locales.Deployments() + " / " + locales.DeployEntry()).
		MaxWidthPercent(100).Disabled(state.Busy).
		OnChange(func() {
			if index := slices.Index(options, selection.choice); index >= 0 {
				selection.pointer = v.resource.Groups[index].Pointer
			}
		})
	for _, group := range v.resource.Groups {
		if group.Pointer == selection.pointer {
			return group, true
		}
	}
	selection.pointer = v.resource.Groups[selected].Pointer
	return v.resource.Groups[selected], true
}

func (v *Page) Render(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	advanced bool,
	jump func(records.Kind, int),
	openHex func(int, int),
	openText func(uint16),
	openScript func(int, int),
) {
	v.load(state, row)
	v.openScript = openScript
	ui.Tabs(c.Key("chapter-mode"), &v.mode, locales.DeployConfiguration(), locales.Deployments(), locales.TurnEvents(), locales.AreaEvents(), locales.TalkEvents(), locales.Objectives(), locales.MapEditor(), locales.MapChangesEditor()).
		Label(locales.Chapters()).
		Wrap()
	if v.mode == 6 {
		v.mapEditor(c, state, row, jump, openHex)
		return
	}
	if v.mode == 7 {
		v.mapChangesEditor(c, state, row, jump, openHex)
		return
	}
	if v.Configuration() {
		v.renderMap(c, state, row, mapview.Overlay{})
		return
	}
	if v.mode >= 2 && v.mode <= 5 {
		table := [...]event.Table{event.TurnEvents, event.AlwaysEvents, event.TalkEvents, event.AlwaysEvents}[v.mode-2]
		v.eventEditor(c, state, row, table, advanced, jump, openHex, openText)
		return
	}
	ui.Column(c.Key("deployments")).Gap(12).Children(func() {
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.DeployEntry()).
				Bold().
				Tooltip(locales.DeployScope())
			ui.Tabs(c.Key("position-mode"), &v.positionMode, locales.DeployStart(), locales.DeployEnd()).
				Label(locales.DeployPosition()).
				OnChange(func() {
					if selected := v.selections[v.chapter]; selected != nil {
						if entry := v.entries[selected.pointer]; entry != nil {
							entry.lastSelected = -1
						}
					}
				})
		})
		group, exists := v.group(c, state)
		if !exists {
			v.renderMap(c, state, row, mapview.Overlay{})
			return
		}
		if group.Err != nil {
			v.renderMap(c, state, row, mapview.Overlay{})
			ui.Text(c, components.ErrorText(group.Err)).
				Tooltip(group.Err.Error()).
				TextColor(c.Theme().Danger)
			return
		}
		entry := v.draft(state, group)
		if entry == nil {
			return
		}
		v.unitScene(c, state, row, entry, nil)
		if len(v.resource.Warnings) > 0 {
			ui.Text(c, locales.GameIncompleteDeploymentScript()).
				FontSize(12).
				TextColor(c.Theme().TextMuted).
				Tooltip(v.resource.Warnings[0].Error())
		}
		v.actions(c, state, group, entry)
		v.preview(c, state, group, entry, jump)
		v.fields(c, state, entry, advanced, jump)
		if advanced {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				ui.Button(c, locales.DeployReference()).
					Disabled(state.Busy).
					OnClick(func() { openHex(group.Pointer, 4) })
				if group.Address != 0 {
					ui.Button(c, locales.DeployRaw()).
						Disabled(state.Busy).
						OnClick(func() { openHex(group.Address, len(group.Units)*group.Stride+4) })
				}
			})
		}
	})
}

func (v *Page) overlay(
	entry *entryDraft,
	units []deployment.Unit,
) mapview.Overlay {
	var markers []mapview.Marker
	for i, unit := range units {
		point := unit.Start
		if v.positionMode == 1 {
			point = unit.End
		}
		markers = append(
			markers,
			mapview.Marker{
				Index:    i,
				Position: point,
				Faction:  unit.Faction,
				Selected: i == entry.selected,
			},
		)
	}
	slices.SortStableFunc(markers, func(a, b mapview.Marker) int {
		if a.Selected == b.Selected {
			return 0
		}
		if a.Selected {
			return 1
		}
		return -1
	})
	return mapview.Overlay{Markers: markers, Choose: func(point image.Point) {
		for offset := 1; offset <= len(units); offset++ {
			index := (entry.selected + offset) % len(units)
			unit := units[index]
			position := unit.Start
			if v.positionMode == 1 {
				position = unit.End
			}
			if position == point {
				entry.selected = index
				return
			}
		}
	}}
}

func nameOf(session *game.Session, kind records.Kind, id int) string {
	if table := session.Records.Table(kind); table != nil && table.Err == nil {
		for _, row := range table.Rows {
			if row.ID == id {
				return row.Name
			}
		}
	}
	return fmt.Sprintf("0x%02X", id)
}

func (v *Page) unitList(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	units []deployment.Unit,
) {
	if len(units) == 0 {
		ui.Text(c, locales.DeployEmpty()).TextColor(c.Theme().TextMuted)
		return
	}
	entry.list.Selected = &entry.selected
	entry.list.Key = func(index int) any { return index }
	columns := []ui.TableColumn{
		{ID: "index", Title: "#", Width: 36, Fixed: true},
		{ID: "name", Title: locales.Name()},
		{ID: "level", Title: locales.DeployLevel(), Width: 44, Fixed: true},
		{
			ID:    "position",
			Title: locales.DeployPosition(),
			Width: 76,
			Fixed: true,
		},
	}
	ui.Table(c.Key("deployment-units"), &entry.list, columns, len(units), func(row, col int) {
		unit := units[row]
		switch col {
		case 0:
			ui.Textf(c, "%02X", row).FontFeatures("tnum")
		case 1:
			ui.Text(c, nameOf(state.Session, records.Units, unit.Character)).
				SingleLine()
		case 2:
			ui.Textf(c, "%d", unit.Level).FontFeatures("tnum")
		case 3:
			point := unit.Start
			if v.positionMode == 1 {
				point = unit.End
			}
			ui.Textf(c, "%d, %d", point.X, point.Y).FontFeatures("tnum")
		}
	}).
		Height(230).
		Label(locales.Deployments() + " · " + locales.Name()).
		Tooltip(locales.DeployPickHelp())
	ui.Text(c, locales.RecordSummary(len(units))).
		FontSize(12).
		TextColor(c.Theme().TextMuted)
}

func (e *entryDraft) invalid() bool {
	for _, input := range e.inputs {
		if input.invalid {
			return true
		}
	}
	return false
}

func (v *Page) actions(
	c *ui.Context,
	state *editor.State,
	group deployment.Group,
	entry *entryDraft,
) {
	stale := !entry.data.Current(state.Session.Deployments)
	disabled := state.Busy || stale
	v.unitActions(c, state, entry)
	if stale {
		ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
	}
	if entry.data.Modified() || stale || entry.invalid() {
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Button(c, locales.DeployPreview()).
				Disabled(disabled || entry.invalid() || !entry.data.Modified()).
				OnClick(func() {
					entry.preview, state.Err = state.Session.Deployments.Plan(
						entry.data,
					)
				})
			ui.Button(c, locales.DeployReload()).
				Disabled(state.Busy).
				OnClick(func() { delete(v.entries, group.Pointer) })
			if entry.data.Modified() {
				ui.Text(c, locales.DeployDraft()).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
			}
		})
	}
}

func (v *Page) place(
	state *editor.State,
	entry *entryDraft,
	point image.Point,
	end bool,
) {
	x, y := "deploy_start_x", "deploy_start_y"
	if end {
		x, y = "deploy_end_x", "deploy_end_y"
	}
	if err := entry.data.Set(entry.selected, x, int64(point.X)); err != nil {
		state.Err = err
		return
	}
	state.Err = entry.data.Set(entry.selected, y, int64(point.Y))
	entry.preview, entry.inputs = nil, nil
}

func (v *Page) preview(
	c *ui.Context,
	state *editor.State,
	group deployment.Group,
	entry *entryDraft,
	jump func(records.Kind, int),
) {
	if entry.preview == nil {
		return
	}
	preview := entry.preview
	ui.Column(c.Key("deployment-preview")).Gap(8).Children(func() {
		ui.Text(c, locales.DeployPreview()).Bold()
		ui.Textf(c, "%s: %d → %d · 0x%08X · %d B", locales.Deployments(), preview.Before(), preview.After(), preview.Destination(), preview.Length()).
			FontFeatures("tnum").
			Selectable()
		ui.Text(c, locales.DeployPreviewScope()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.DeployUses()).Bold()
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, use := range preview.Uses() {
				ui.Button(c, fmt.Sprintf("%02X %s", use.Chapter, nameOf(state.Session, records.Chapters, use.Chapter))).
					Disabled(state.Busy).
					OnClick(func() { jump(records.Chapters, use.Chapter) })
			}
		})
		ui.Button(c, locales.DeployApply()).
			Disabled(state.Busy || !entry.data.Current(state.Session.Deployments) || entry.invalid()).
			OnClick(func() {
				if state.Err = state.Session.Deployments.Apply(
					preview,
				); state.Err == nil {
					delete(v.entries, group.Pointer)
					state.HistoryEpoch++
				}
			})
	})
}

func (v *Page) renderMap(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	overlay mapview.Overlay,
) {
	if !v.baseMap.Current(state.Session.Maps) || v.mapChapter != row.ID ||
		v.mapAddress != row.Address {
		if v.mapChapter != row.ID || v.mapAddress != row.Address {
			v.mapView.ResetInteraction()
		}
		v.baseMap, v.mapErr = state.Session.Maps.Read(row.ID)
		v.mapChapter, v.mapAddress = row.ID, row.Address
	}
	v.mapView.Render(c, v.baseMap, v.mapErr, overlay)
}
