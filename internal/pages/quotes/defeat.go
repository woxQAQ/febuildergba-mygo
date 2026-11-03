package quotes

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// ViewDefeat puts the character's dialog first. Conditional records retain
// their scope and navigation; uncommon edits live in an explicit disclosure.
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
	disabled := stale || p.state.Busy
	width := min(available, 880)
	ui.Column(c.Key(p)).
		Width(width).
		MaxWidthPercent(100).
		Gap(12).
		Children(func() {
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			if stale && entry.draft != nil {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			if entry.draft == nil {
				p.defeatTableChoice(c)
				p.actions(c, entry, stale)
				return
			}
			if table.Kind != quote.Death {
				ui.Text(c, p.tableLabel(table.Kind)).
					FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted)
			}
			p.defeatRecordChoice(c, entry)
			index := entry.index()
			scriptPrimary := false
			if index >= 0 {
				ui.Column(c.Key(struct {
					table *tableState
					row   uint64
				}{entry, entry.rows[index].ID})).Gap(8).Children(func() {
					if len(entry.filtered) == 1 {
						if scope := p.defeatScope(entry, index); scope != "" {
							ui.Text(c, scope).
								FontSize(components.FontCaption).
								TextColor(c.Theme().TextMuted)
						}
					}
					p.defeatField(
						c,
						entry,
						index,
						quote.Text,
						width-100,
						disabled,
					)
					textID, textErr := entry.draft.Value(index, quote.Text)
					script, scriptErr := entry.draft.Value(index, quote.Script)
					scriptPrimary = scriptErr == nil &&
						(textErr != nil || textID == 0 && script != 0)
					if scriptPrimary {
						p.defeatField(
							c,
							entry,
							index,
							quote.Script,
							width-100,
							disabled,
						)
					}
					if p.advanced {
						p.rawRecord(c, entry.rows[index])
					}
				})
			} else {
				ui.Text(c, locales.QuoteNoDefeat()).
					TextColor(c.Theme().TextMuted)
				p.addButton(c, entry, disabled).AlignSelf(ui.Start)
			}
			ui.Collapsible(
				c.Key("defeat-settings"),
				locales.QuoteDefeatSettings(),
				&p.showDefeatSettings,
				func() {
					ui.Column(c).Gap(12).Children(func() {
						p.defeatTableChoice(c)
						if index >= 0 {
							p.structure(c, entry, disabled)
							columns := 1
							if width >= 680 {
								columns = 2
							}
							cell := (width - float32((columns-1)*16)) / float32(
								columns,
							)
							ui.Column(c.Key(struct {
								table *tableState
								row   uint64
							}{entry, entry.rows[index].ID})).Gap(12).Children(func() {
								ui.Grid(c.Key("conditions")).
									Columns(columns).
									Gap(16).
									AlignItems(ui.Start).
									Children(func() {
										p.defeatField(
											c,
											entry,
											index,
											quote.Chapter,
											cell-80,
											disabled,
										)
										p.defeatField(
											c,
											entry,
											index,
											quote.Route,
											cell-80,
											disabled,
										)
									})
								ui.Grid(c.Key("flags-script")).
									Columns(columns).
									Gap(16).
									AlignItems(ui.Start).
									Children(func() {
										p.defeatField(
											c,
											entry,
											index,
											quote.Flag,
											cell-80,
											disabled,
										)
										if !scriptPrimary {
											p.defeatField(
												c,
												entry,
												index,
												quote.Script,
												cell-100,
												disabled,
											)
										}
									})
								p.defeatField(
									c,
									entry,
									index,
									quote.FinalText,
									width-100,
									disabled,
								)
							})
						}
						ui.Text(c, p.tableHelp(table.Kind)).
							FontSize(components.FontCaption).
							TextColor(c.Theme().TextMuted)
						if p.advanced {
							ui.Row(c).Wrap().Gap(8).Children(func() {
								for _, site := range table.Pointers {
									ui.Button(c.Key(site), fmt.Sprintf("0x%08X", site)).
										Disabled(p.state.Busy).
										OnClick(func() { p.openHex(site, 4) })
								}
							})
						}
					})
				},
			)
			p.actions(c, entry, stale)
		})
}

func (p *Page) defeatField(
	c *ui.Context,
	entry *tableState,
	index int,
	id string,
	width float32,
	disabled bool,
) {
	for _, field := range entry.draft.Fields() {
		if field.ID == id {
			p.field(c.Key(id), entry, index, field, disabled, width)
			return
		}
	}
}

func (p *Page) defeatScope(entry *tableState, index int) string {
	var parts []string
	for _, field := range entry.draft.Fields() {
		if field.ID != quote.Chapter && field.ID != quote.Route {
			continue
		}
		value, err := entry.draft.Value(index, field.ID)
		if err != nil ||
			field.ID == quote.Chapter &&
				value == p.state.Session.Quotes.AnyChapter() ||
			field.ID == quote.Route && value == 0xFF {
			continue
		}
		caption := fmt.Sprintf("%02X", value)
		labels, values := p.choices(field)
		for _, label := range labels {
			if values[label] == value {
				caption = label
				break
			}
		}
		parts = append(parts, locales.Label(field.ID)+": "+caption)
	}
	return strings.Join(parts, " / ")
}

func (p *Page) defeatTableChoice(c *ui.Context) {
	if len(p.tables) < 2 {
		return
	}
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
	if !slices.Contains(labels, p.categoryChoice) {
		p.categoryChoice = labels[p.category]
	}
	ui.Select(c.Key("defeat-table"), &p.categoryChoice, labels).
		Label(locales.QuoteTable()).
		Disabled(p.state.Busy).
		OnChange(func() { p.category = max(0, slices.Index(labels, p.categoryChoice)) })
}

func (p *Page) defeatRecordChoice(c *ui.Context, entry *tableState) {
	if len(entry.filtered) < 2 {
		return
	}
	labels := make([]string, len(entry.filtered))
	for i, index := range entry.filtered {
		labels[i] = fmt.Sprintf("%d · %s", i+1, p.conditions(entry, index))
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
