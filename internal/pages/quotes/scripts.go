package quotes

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// Script navigation uses the committed pointer site. A pending table edit must
// be applied first because copied, reordered or changed rows may have new owners.
func (p *Page) scriptLink(
	c *ui.Context,
	entry *tableState,
	index int,
	disabled bool,
) {
	value, err := entry.draft.Value(index, quote.Script)
	site := 0
	for _, field := range entry.draft.Fields() {
		if field.ID == quote.Script {
			site = entry.rows[index].Address + field.Offset
			break
		}
	}
	help := locales.QuoteOpenScript()
	if entry.draft.Modified() {
		help = locales.QuoteScriptApplyFirst()
	}
	ui.Button(c, locales.QuoteOpenScript()).Tooltip(help).
		Disabled(disabled || p.openScript == nil || err != nil || site == 0 ||
			value < 0x080000C0 || value >= 0x0A000000 || value%4 != 0 ||
			entry.draft.Modified() || entry.invalid() || !entry.draft.Current(p.state.Session.Quotes)).
		OnClick(func() { p.openScript(entry.draft.Kind(), index, site) })
}
