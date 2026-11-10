package eventeditor

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// OpenQuote selects the committed table role as well as its physical pointer.
// Existing chapter, text and quote drafts remain owned by their original views.
func (page *Page) OpenQuote(source string, record, site int) {
	page.Open(event.QuoteScripts, site)
	for index, entry := range page.rows {
		if entry.Pointer == site && entry.Source == source &&
			entry.Record == record {
			page.selected = index
			page.list.ScrollTo(index, ui.Center)
			return
		}
	}
	page.state.SetErr(event.ErrScript)
}

func (page *Page) readOnlyOperands(
	c *ui.Context,
	draft *draftState,
	row event.ScriptInstruction,
) {
	stale := !draft.data.Current(page.state.Session.Events)
	if row.Help != "" {
		ui.Text(c, locales.Label(row.Help)).
			FontSize(components.FontCaption).TextColor(c.Theme().TextMuted)
	}
	for _, field := range draft.data.Fields(draft.selected) {
		if field.Target == event.ScriptTextTarget {
			continue
		}
		value, _ := draft.data.Value(draft.selected, field.ID)
		format := "%d"
		if field.Hex && value >= 0 {
			format = "0x%X"
		}
		ui.Textf(c.Key(field.ID), "%s · "+format, locales.Label(field.ID), value).
			Selectable().
			Tooltip(locales.Label(field.Help))
		if field.Target == event.ScriptEventTarget {
			reference, err := draft.data.Reference(draft.selected, field.ID)
			ui.Button(c.Key("call-"+field.ID), locales.ScriptOpenCall()).
				Disabled(page.state.Busy || stale || err != nil).
				OnClick(func() { page.Open(reference.Chapter, reference.Pointer) })
		}
	}
	for _, operand := range draft.data.TextOperands(draft.selected) {
		ui.Column(c.Key(operand.Field)).Gap(4).Children(func() {
			ui.Text(c, locales.Label(operand.Field))
			components.ReferenceLink(
				c,
				fmt.Sprintf("%s %04X", locales.QuoteOpenText(), operand.ID),
				page.state.Busy || stale || page.openText == nil,
				func() { page.openText(operand.ID) },
			)
			components.TextPreview(
				c,
				page.state.Session.Texts,
				uint32(operand.ID),
			)
		})
	}
	if row.Op == event.ScriptUnknown || row.Op == event.ScriptASM {
		ui.Textf(c, "% X", row.Raw).Font("monospace").Selectable()
	}
}
