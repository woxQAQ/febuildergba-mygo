package quotes

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// ForUnit gives a character its own drafts, including the separate FE7 tables.
// Applying another character's draft makes pending inputs stale, never merged.
func (p *Page) ForUnit(id int) { p.unit = id }

func (p *Page) refreshTables() {
	p.tables = nil
	for _, table := range p.state.Session.Quotes.Tables() {
		if defeatTable(table.Kind) == (p.unit >= 0) ||
			table.Kind == p.extraKind {
			p.tables = append(p.tables, table)
		}
	}
}

func (p *Page) selectIndex(entry *tableState, index int) {
	entry.filtered = nil
	p.filter(entry)
	for i, physical := range entry.filtered {
		if physical == index {
			entry.selected = i
			break
		}
	}
	entry.list = ui.ListState{}
	entry.list.ScrollTo(entry.selected, ui.Center)
}

func (p *Page) conditions(entry *tableState, index int) string {
	var parts []string
	if chapter, err := entry.draft.Value(index, quote.Chapter); err == nil {
		name := p.name(records.Chapters, chapter)
		if chapter == p.state.Session.Quotes.AnyChapter() {
			name = locales.QuoteAnyChapter()
		} else if triangle, ok := p.state.Session.Quotes.TriangleChapter(entry.draft.Kind()); ok && chapter == triangle {
			name = locales.QuoteTriangle()
		}
		if name == "" {
			name = fmt.Sprintf("%s %02X", locales.Label(quote.Chapter), chapter)
		}
		parts = append(parts, name)
	}
	if route, err := entry.draft.Value(index, quote.Route); err == nil {
		labels := map[uint32]string{
			0:   locales.QuoteRouteCommon(),
			1:   locales.QuoteRouteEirika(),
			2:   locales.QuoteRouteEphraim(),
			255: locales.QuoteAnyRoute(),
		}
		if label := labels[route]; label != "" {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, " · ")
}

func (p *Page) listRow(c *ui.Context, entry *tableState, index int) {
	if p.unit >= 0 {
		ui.Text(c, p.conditions(entry, index)).SingleLine().FontSize(13)
		return
	}
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			unit, _ := entry.draft.Value(index, quote.Unit)
			if condition, err := entry.draft.BattleCondition(
				index,
			); err == nil {
				switch condition.Participants {
				case quote.Attacker:
					p.person(c.Key("unit"), unit)
					ui.Text(c, locales.QuoteAttackerOnly()).FontSize(12)
					return
				case quote.Defender:
					other, _ := entry.draft.Value(index, quote.Opponent)
					p.person(c.Key("opponent"), other)
					ui.Text(c, locales.QuoteDefenderOnly()).FontSize(12)
					return
				case quote.Neither:
					ui.Text(c, locales.QuoteInactive()).FontSize(13)
					return
				}
			}
			p.person(c.Key("unit"), unit)
			if other, err := entry.draft.Value(
				index,
				quote.Opponent,
			); err == nil {
				ui.Text(c, "↔")
				p.person(c.Key("opponent"), other)
			}
		})
		ui.Text(c, p.conditions(entry, index)).
			FontSize(11).
			SingleLine()
	})
}

func (p *Page) person(c *ui.Context, id uint32) {
	name := p.unitName(id)
	ui.Row(c).
		Gap(4).
		Grow(1).
		Basis(0).
		MinWidth(0).
		AlignItems(ui.Center).
		Children(func() {
			ui.Text(c, name).
				FontSize(13).
				SingleLine().
				Grow(1).
				MinWidth(0).
				Tooltip(name)
			if id != 0 && id <= 255 {
				portrait := p.thumbnails.Portrait(p.state.Session, int(id))
				if portrait.Available() {
					portrait.View(c, 32, name)
				}
			}
		})
}
