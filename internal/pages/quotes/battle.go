package quotes

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (p *Page) tableLabel(kind quote.Kind) string {
	if p.state.Session.Quotes.Version() == 8 && kind == quote.Battle {
		return locales.QuoteEditor()
	}
	return locales.Label(string(kind))
}

func (p *Page) matchesBattle(entry *tableState, index int) bool {
	if p.battleScope == 0 || p.unit >= 0 || entry.draft.Kind() != quote.Battle {
		return true
	}
	condition, err := entry.draft.BattleCondition(index)
	if err != nil {
		return true
	}
	switch p.battleScope {
	case 1:
		return condition.Participants == quote.Pair
	case 2:
		return condition.Participants == quote.Attacker ||
			condition.Participants == quote.Defender
	case 3:
		return condition.Triangle
	}
	return true
}

func (p *Page) battleFilter(c *ui.Context, entry *tableState) {
	if p.state.Session.Quotes.Version() != 8 || entry.draft == nil ||
		entry.draft.Kind() != quote.Battle {
		return
	}
	labels := []string{
		locales.QuoteAllBattles(),
		locales.QuotePairs(),
		locales.QuoteSingles(),
		locales.QuoteTriangle(),
	}
	if p.scopeChoice == "" || !slices.Contains(labels, p.scopeChoice) {
		p.scopeChoice = labels[p.battleScope]
	}
	ui.Select(c.Key("battle-filter"), &p.scopeChoice, labels).
		Label(locales.QuoteView()).
		Tooltip(locales.QuoteFe8ViewsHelp()).
		Disabled(p.state.Busy).
		OnChange(func() {
			for i, label := range labels {
				if label == p.scopeChoice {
					p.battleScope = i
					break
				}
			}
		})
}

func (p *Page) addRecord(entry *tableState) error {
	if p.state.Session.Quotes.Version() == 8 &&
		entry.draft.Kind() == quote.Battle {
		condition := quote.BattleCondition{Participants: quote.Pair}
		if p.battleScope == 2 || p.battleScope == 3 {
			condition.Participants = quote.Attacker
		}
		condition.Triangle = p.battleScope == 3
		return entry.draft.AddBattle(condition)
	}
	return entry.draft.Add()
}
