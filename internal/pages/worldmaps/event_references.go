package worldmaps

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (p *Page) SetScriptOpener(open func(int, int)) { p.openScript = open }

func (p *Page) eventChapters(entry event.WorldMapEvent) string {
	var names []string
	table := p.state.Session.Records.Table(records.Chapters)
	for _, id := range entry.Chapters {
		var caption strings.Builder
		fmt.Fprintf(&caption, "%02X", id)
		if table != nil && table.Err == nil {
			for _, row := range table.Rows {
				if row.ID == id {
					if row.Name != "" {
						caption.WriteString(" · " + row.Name)
					}
					break
				}
			}
		}
		names = append(names, caption.String())
	}
	return strings.Join(names, " / ")
}

func worldMapEventCaption(entry event.ScriptEntry) string {
	caption := locales.Label(entry.Source)
	if entry.WorldMap && entry.EventID >= 0 {
		caption += fmt.Sprintf(" %02X", entry.EventID)
	}
	return caption
}

func (p *Page) relatedEvents(
	c *ui.Context,
	target event.ScriptTarget,
	id int,
	open *bool,
) {
	if !p.state.Session.Events.WorldMapEventsAvailable() {
		return
	}
	ui.Collapsible(c.Key("related-events"), locales.Label("worldmap_related_events"), open, func() {
		rows, complete, err := p.state.Session.Events.WorldMapUses(target, id)
		if err != nil {
			ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
			return
		}
		if !complete {
			ui.Text(c, locales.Label("worldmap_event_partial")).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
		if len(rows) == 0 {
			ui.Text(c, locales.Label("worldmap_no_event_references"))
			return
		}
		ui.Scroll(c.Key("event-references")).
			MaxHeight(240).
			Gap(8).
			Children(func() {
				for _, item := range rows {
					entry := item.Entry
					key := struct {
						pointer int
						source  string
					}{entry.Pointer, entry.Source}
					ui.Row(c.Key(key)).
						Gap(8).
						AlignItems(ui.Center).
						Children(func() {
							ui.Column(c).
								Grow(1).
								MinWidth(0).
								Gap(2).
								Children(func() {
									if chapters := p.eventChapters(
										item,
									); chapters != "" {
										ui.Text(c, chapters).
											SingleLine().
											Tooltip(chapters)
									}
									ui.Text(c, worldMapEventCaption(entry)).
										SingleLine().
										Tooltip(worldMapEventCaption(entry)).
										FontSize(components.FontCaption).
										TextColor(c.Theme().TextMuted)
								})
							ui.Button(c, locales.Label("worldmap_open_event")).
								Shrink(0).
								Disabled(p.state.Busy || p.openScript == nil).
								OnClick(func() { p.openScript(event.WorldMapScripts, entry.Pointer) })
						})
				}
			})
	}).
		Disabled(p.state.Busy)
}
