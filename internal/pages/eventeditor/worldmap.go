package eventeditor

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func entryCaption(entry event.ScriptEntry) string {
	caption := locales.Label(entry.Source)
	if entry.WorldMap && entry.EventID >= 0 {
		caption += fmt.Sprintf(" %02X", entry.EventID)
	}
	if entry.Chapter == event.QuoteScripts && entry.Record >= 0 {
		caption += fmt.Sprintf(" · %04X", entry.Record)
	}
	return caption
}

func (page *Page) worldMapOptions(field event.ScriptField) ([]int64, []string) {
	var values []int64
	var captions []string
	nodes, err := page.state.Session.WorldMap.Nodes()
	if err != nil {
		return nil, nil
	}
	name := func(id int) string {
		if id < 0 || id >= len(nodes) {
			return fmt.Sprintf("%02X", id)
		}
		value, _ := page.state.Session.Texts.Value(nodes[id].Name)
		return fmt.Sprintf("%02X · %s", id, value.Plain)
	}
	if field.Target == event.ScriptWorldMapNodeTarget {
		for _, node := range nodes {
			values = append(values, int64(node.ID))
			captions = append(captions, name(node.ID))
		}
	} else {
		routes, _ := page.state.Session.WorldMap.Routes()
		for _, route := range routes {
			values = append(values, int64(route.ID))
			captions = append(
				captions,
				fmt.Sprintf(
					"%02X · %s ↔ %s",
					route.ID,
					name(route.Nodes[0]),
					name(route.Nodes[1]),
				),
			)
		}
	}
	return values, captions
}

func (page *Page) worldMapLink(
	c *ui.Context,
	draft *draftState,
	field event.ScriptField,
	value int64,
) {
	values, _ := page.worldMapOptions(field)
	valid := slices.Contains(values, value)
	label := "script_wm_open_node"
	if field.Target == event.ScriptWorldMapRouteTarget {
		label = "script_wm_open_route_resource"
	}
	ui.Button(c.Key(field.ID), locales.Label(label)+" · "+locales.Label(field.ID)).
		Disabled(page.state.Busy || !valid || !draft.data.Current(page.state.Session.Events) || page.openWorldMap == nil).
		OnClick(func() { page.openWorldMap(field.Target, int(value)) })
}
