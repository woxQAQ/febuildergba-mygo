package recordform

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type nameDraft struct {
	text, original string
	textID         uint16
	epoch          uint64
	preview        *game.NameEdit
	err            error
}

func (a *Form) nameEditor(c *ui.Context, kind game.Kind, row game.Row) {
	if a.names == nil {
		a.names = make(map[fieldKey]*nameDraft)
	}
	key := fieldKey{Kind: kind, Record: row.ID, Field: "name"}
	draft := a.names[key]
	if draft == nil || draft.original != row.Name || draft.textID != row.NameID || draft.epoch != a.state.HistoryEpoch {
		draft = &nameDraft{text: row.Name, original: row.Name, textID: row.NameID, epoch: a.state.HistoryEpoch}
		a.names[key] = draft
	}
	if draft.preview != nil && draft.preview.Revision() != a.state.Session.Document.State().Revision {
		draft.preview = nil
	}
	ui.Column(c.Key("name-editor")).Gap(6).Children(func() {
		ui.Text(c, locales.NameEditScope()).FontSize(12).TextColor(c.Theme().TextMuted)
		unavailable := row.NameID == 0 || row.TextError != nil
		ui.Row(c).Gap(8).Children(func() {
			components.TextDraft(a.state, ui.TextInput(c, &draft.text).Label(locales.EditName()).Grow(1).
				Disabled(a.state.Busy || unavailable).OnChange(func() {
				draft.preview, draft.err = nil, nil
			}))
			ui.Button(c, locales.PreviewName()).Disabled(a.state.Busy || unavailable || draft.text == row.Name).OnClick(func() {
				draft.preview, draft.err = a.state.Session.PlanNameEdit(kind, row.ID, draft.text)
			})
			ui.Button(c, locales.Cancel()).Disabled(a.state.Busy || draft.text == row.Name).OnClick(func() {
				draft.text, draft.preview, draft.err = row.Name, nil, nil
			})
		})
		if row.NameID == 0 {
			ui.Text(c, locales.NameNeedsId()).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if draft.err != nil {
			ui.Text(c, locales.ErrorText(draft.err)).TextColor(c.Theme().Danger)
		}
		if edit := draft.preview; edit != nil {
			uses := edit.Uses()
			ui.Text(c, locales.NameEditSummary(fmt.Sprintf("0x%04X", edit.TextID()), len(uses))).FontSize(12)
			for _, useKind := range game.Kinds {
				var ids []string
				for _, use := range uses {
					if use.Kind == useKind {
						ids = append(ids, fmt.Sprintf("%02X", use.ID))
					}
				}
				if len(ids) > 0 {
					ui.Text(c, locales.Label(string(useKind))+": "+strings.Join(ids, ", ")).FontSize(12)
				}
			}
			ui.Button(c, locales.ApplyName()).Disabled(a.state.Busy).OnClick(func() {
				if err := a.state.Session.ApplyNameEdit(edit); err != nil {
					draft.err = err
					draft.preview = nil
					return
				}
				delete(a.names, key)
			})
		}
	})
}
