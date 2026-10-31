package quotes

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// ViewDefeat edits the selected character in place. Native table navigation
// appears only for FE7; record navigation appears only when there is a choice.
func (p *Page) ViewDefeat(c *ui.Context, available float32, raw bool) {
	p.advanced = raw
	p.refreshTables()
	if len(p.tables) == 0 {
		return
	}
	p.category = max(0, min(p.category, len(p.tables)-1))
	table := p.tables[p.category]
	entry := p.draft(table)
	p.query = ""
	p.filter(entry)
	stale := entry.draft == nil || !entry.draft.Current(p.state.Session.Quotes)
	width := min(available, 880)
	ui.Column(c.Key(p)).
		Width(width).
		MaxWidthPercent(100).
		Gap(12).
		Children(func() {
			if len(p.tables) > 1 {
				labels := make([]string, len(p.tables))
				for i, table := range p.tables {
					switch table.Kind {
					case quote.Death:
						labels[i] = locales.QuoteOrdinary()
					case quote.DeathLyn:
						labels[i] = locales.QuoteLynScripts()
					case quote.DeathOther:
						labels[i] = locales.QuoteOtherScripts()
					}
				}
				ui.Tabs(c.Key("defeat-table"), &p.category, labels...).
					Label(locales.QuoteTable()).
					Disabled(p.state.Busy)
			}
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			if stale && entry.draft != nil {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			if entry.draft == nil {
				p.actions(c, entry, stale)
				return
			}
			if len(entry.filtered) > 1 {
				labels := make([]string, len(entry.filtered))
				for i, index := range entry.filtered {
					labels[i] = fmt.Sprintf(
						"%d · %s",
						i+1,
						p.conditions(entry, index),
					)
				}
				selected := entry.index()
				if selected >= 0 &&
					(entry.choiceID != entry.rows[selected].ID || !slices.Contains(labels, entry.recordChoice)) {
					entry.choiceID = entry.rows[selected].ID
					entry.recordChoice = labels[entry.selected]
				}
				ui.Select(c.Key("defeat-record"), &entry.recordChoice, labels).
					Label(locales.QuoteRecords()).
					Disabled(p.state.Busy).
					OnChange(func() {
						for i, label := range labels {
							if label == entry.recordChoice {
								entry.selected = i
								entry.choiceID = entry.rows[entry.filtered[i]].ID
								break
							}
						}
					})
			}
			p.structure(c, entry, stale || p.state.Busy)
			if index := entry.index(); index >= 0 {
				ui.Column(c.Key(struct {
					table *tableState
					row   uint64
				}{entry, entry.rows[index].ID})).Gap(14).Children(func() {
					fields := entry.draft.Fields()
					render := func(id string, space float32) {
						for _, field := range fields {
							if field.ID == id {
								p.field(
									c.Key(id),
									entry,
									index,
									field,
									stale || p.state.Busy,
									space,
								)
								return
							}
						}
					}
					columns := 1
					if width >= 680 {
						columns = 2
					}
					cell := (width - float32((columns-1)*16)) / float32(columns)
					ui.Grid(c.Key("conditions")).
						Columns(columns).
						Gap(16).
						AlignItems(ui.Start).
						Children(func() {
							render(quote.Chapter, cell-80)
							render(quote.Route, cell-80)
						})
					for _, id := range []string{quote.Text, quote.FinalText} {
						if _, err := entry.draft.Value(index, id); err != nil {
							continue
						}
						ui.Column(c.Key("text-" + id)).
							Gap(8).
							Children(func() { render(id, width-100); p.textPreview(c, entry, index, id) })
					}
					ui.Grid(c.Key("flags-script")).
						Columns(columns).
						Gap(16).
						AlignItems(ui.Start).
						Children(func() {
							render(quote.Flag, cell-80)
							render(quote.Script, cell-100)
						})
					if p.advanced {
						p.rawRecord(c, entry.rows[index])
					}
				})
			} else {
				ui.Text(c, locales.QuoteNoDefeat()).
					TextColor(c.Theme().TextMuted)
			}
			ui.Collapsible(
				c.Key("matching-rules"),
				locales.QuoteMatchingRules(),
				&p.showHelp,
				func() { ui.Text(c, p.tableHelp(table.Kind)).FontSize(12) },
			)
			if p.advanced {
				ui.Row(c).Wrap().Gap(8).Children(func() {
					for _, site := range table.Pointers {
						ui.Button(c.Key(site), fmt.Sprintf("0x%08X", site)).
							Disabled(p.state.Busy).
							OnClick(func() { p.openHex(site, 4) })
					}
				})
			}
			p.actions(c, entry, stale)
		})
}
