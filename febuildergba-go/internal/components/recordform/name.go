package recordform

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (a *Form) nameEditor(c *ui.Context, kind game.Kind, row game.Row) ui.Element {
	key := fieldKey{Kind: kind, Record: row.ID, Field: "name_id"}
	draft := a.textDraftFor(key, row.NameID, row.Name)
	return ui.Column(c.Key("name-editor")).Gap(6).Children(func() {
		unavailable := row.NameID == 0 || row.TextError != nil
		ui.Row(c).Gap(8).Children(func() {
			components.TextDraft(a.state, ui.TextInput(c, &draft.text).Label(locales.Name()).Tooltip(locales.NameEditScope()).FontSize(20).Height(34).Grow(1).
				Disabled(a.state.Busy || unavailable).OnChange(func() {
				draft.preview, draft.err = nil, nil
			}))
			if draft.text != row.Name {
				ui.Button(c, locales.PreviewName()).Disabled(a.state.Busy || unavailable).OnClick(func() {
					draft.preview, draft.err = a.state.Session.PlanTextEdit(kind, row.ID, key.Field, draft.text)
				})
				ui.Button(c, locales.Cancel()).Disabled(a.state.Busy).OnClick(func() {
					draft.text, draft.preview, draft.err = row.Name, nil, nil
				})
			}
		})
		if row.NameID == 0 {
			ui.Text(c, locales.NameNeedsId()).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if draft.err != nil {
			ui.Text(c, locales.ErrorText(draft.err)).TextColor(c.Theme().Danger)
		}
		a.textPreview(c, key, draft, locales.ApplyName())
	})
}
