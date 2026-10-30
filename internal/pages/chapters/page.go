// Package chapters shares a chapter's map, deployment selection and drafts.
package chapters

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

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

// Deactivate cancels pending placement while keeping the chapter drafts.
func (v *Page) Deactivate() {
	for _, entry := range v.events {
		entry.location = locationInteraction{}
	}
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

func (v *Page) Render(
	c *ui.Context,
	state *editor.State,
	row records.Row,
	width float32,
	advanced bool,
	jump func(records.Kind, int),
	openHex func(int, int),
	openText func(uint16),
	openScript func(int, int),
) {
	v.load(state, row)
	v.openScript = openScript
	ui.Tabs(c.Key("chapter-mode"), &v.mode, locales.DeployConfiguration(), locales.Deployments(), locales.TurnEvents(), locales.AreaEvents(), locales.TalkEvents(), locales.Objectives(), locales.MapEditor(), locales.MapChangesEditor(), locales.LocationEvents()).
		Label(locales.Chapters()).
		Wrap()
	for key, entry := range v.events {
		if v.mode != 8 || key.chapter != row.ID {
			entry.location = locationInteraction{}
		}
	}
	if v.mode == 6 {
		v.mapEditor(c, state, row, jump, openHex)
		return
	}
	if v.mode == 7 {
		v.mapChangesEditor(c, state, row, jump, openHex)
		return
	}
	if v.mode == 8 {
		v.locationEditor(c, state, row, width, advanced, openHex, openText)
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
