package records

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type textDraft struct {
	text, original string
	textID         uint16
	epoch          uint64
	preview        *text.Plan
	err            error
}

func (draft *textDraft) modified() bool { return draft.text != draft.original }

func (draft *textDraft) matches(value text.Value) bool {
	return draft.textID == value.ID && draft.original == value.Content
}

// Names retain the record reader's display semantics; descriptions retain
// the complete editable control language. Read errors never become new text.
func (a *form) textSource(key fieldKey) (text.Value, error) {
	if key.Field != "name_id" {
		return a.state.Session.Records.Text(key.Kind, key.Record, key.Field)
	}
	_, row, _, err := a.state.Session.Records.Resolve(
		key.Kind,
		key.Record,
		key.Field,
	)
	if err != nil {
		return text.Value{}, err
	}
	return text.Value{
		ID: row.NameID, Content: row.Name,
		Editable: row.NameID != 0 && row.TextError == nil,
	}, row.TextError
}

func (a *form) textDraftFor(
	key fieldKey,
	value text.Value,
	readErr error,
) *textDraft {
	if a.texts == nil {
		a.texts = make(map[fieldKey]*textDraft)
	}
	draft := a.texts[key]
	if draft == nil ||
		!draft.modified() && readErr == nil && !draft.matches(value) {
		draft = &textDraft{
			text:     value.Content,
			original: value.Content,
			textID:   value.ID,
		}
		a.texts[key] = draft
	}
	if readErr != nil || !value.Editable || !draft.matches(value) ||
		draft.epoch != a.state.HistoryEpoch ||
		draft.preview != nil &&
			draft.preview.Revision() != a.state.Session.Document.State().Revision {
		draft.preview = nil
	}
	draft.epoch = a.state.HistoryEpoch
	return draft
}

func (a *form) checkTextDraft(key fieldKey, draft *textDraft) error {
	if a.state.Busy || a.texts[key] != draft {
		return rom.ErrStale
	}
	value, err := a.textSource(key)
	if err != nil {
		return err
	}
	if !value.Editable || !draft.matches(value) {
		return rom.ErrStale
	}
	return nil
}

func (a *form) previewText(key fieldKey, draft *textDraft) {
	draft.preview = nil
	draft.err = a.checkTextDraft(key, draft)
	if draft.err == nil {
		draft.preview, draft.err = a.state.Session.Records.PlanTextEdit(
			key.Kind, key.Record, key.Field, draft.text,
		)
	}
	a.state.Report("editor.operation", draft.err)
}

func (a *form) textPreview(
	c *ui.Context,
	key fieldKey,
	draft *textDraft,
	label string,
	unavailable bool,
) {
	edit := draft.preview
	if edit == nil {
		return
	}
	uses := edit.Uses()
	ui.Text(
		c,
		locales.TextEditSummary(
			fmt.Sprintf("0x%04X", edit.TextID()),
			len(uses),
		),
	)
	components.TextUses(
		c.Key("text-uses"),
		a.state,
		uses,
		edit.UsesComplete(),
		unavailable,
		nil,
	)
	ui.Button(c, label).Disabled(a.state.Busy || unavailable).OnClick(func() {
		if err := a.state.Edit(func() error {
			if err := a.checkTextDraft(key, draft); err != nil {
				return err
			}
			if draft.preview != edit {
				return rom.ErrStale
			}
			return a.state.Session.Texts.Apply(edit)
		}); err != nil {
			draft.err, draft.preview = err, nil
			return
		}
		delete(a.texts, key)
	})
}

func (a *form) textReferenceDraft(key fieldKey) *fieldDraft {
	if a.drafts == nil {
		a.drafts = make(map[fieldKey]*fieldDraft)
	}
	if a.drafts[key] == nil {
		a.drafts[key] = &fieldDraft{}
	}
	return a.drafts[key]
}

func (a *form) openTextReference(key fieldKey) {
	switch key.Kind {
	case records.Units, records.Classes:
		a.selectProfileTab(key.Kind, key.Record, profileRawTab)
	}
	a.textReferenceDraft(key).input.Focus()
}

func (a *form) textReferenceField(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	field records.Field,
) {
	key := fieldKey{kind, row.ID, field.ID}
	draft := a.textReferenceDraft(key)
	value, err := a.state.Session.Records.Value(kind, row.ID, field.ID)
	ui.Column(c.Key(field.ID)).Gap(4).Children(func() {
		label := locales.Label(field.ID)
		help := label
		if kind == records.Chapters &&
			(field.ID == "objective_text_id" || field.ID == "objective_detail_text_id") {
			help += "\n" + locales.ChapterObjectiveHelp()
		}
		ui.Text(c, label).Tooltip(help).
			OnClick(func() { a.focusField(key) })
		if err != nil {
			ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
			return
		}
		if value < 0 || value > 0xFFFFFFFF {
			ui.Text(c, locales.GameInvalidValue()).TextColor(c.Theme().Danger)
			return
		}
		components.TextReference(
			c,
			a.state,
			&draft.reference,
			uint32(value),
			a.fieldLabel(kind, field),
			false,
			func(id uint16) {
				a.state.SetErr(a.state.Edit(func() error {
					return a.state.Session.Records.Set(
						kind,
						[]int{row.ID},
						field.ID,
						int64(id),
					)
				}))
			},
			a.openText,
		).
			Bind(&draft.input)
	})
}

func (a *form) descriptionEditors(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
) {
	a.descriptions(c, kind, row, table, 112)
}

func (a *form) descriptions(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	height float32,
) {
	for _, field := range table.Fields {
		if field.Text == text.Description {
			a.descriptionEditor(c, kind, row, field, height)
		}
	}
}

func (a *form) descriptionEditor(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	field records.Field,
	height float32,
) ui.Element {
	key := fieldKey{Kind: kind, Record: row.ID, Field: field.ID}
	value, err := a.textSource(key)
	draft := a.textDraftFor(key, value, err)
	stale := err == nil && !draft.matches(value)
	pending := draft.modified() || stale
	unavailable := err != nil || !value.Editable || stale
	label := locales.Label(strings.TrimSuffix(field.ID, "_id"))
	return ui.Column(c.Key(field.ID + "-editor")).Gap(6).Children(func() {
		if err == nil && value.ID == 0 && !pending {
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).Bold().Grow(1)
				if kind == records.Items {
					a.itemTextAction(c, key, label, 0)
				} else {
					ui.Button(c, locales.ChooseTextId()).
						Disabled(a.state.Busy).
						OnClick(func() { a.openTextReference(key) })
				}
			})
			return
		}
		if kind == records.Items {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).Bold().Grow(1)
				a.itemTextAction(c, key, label, value.ID)
			})
		} else if height > 64 {
			ui.Text(c, label).Bold()
		}
		components.TextDraft(
			a.state,
			ui.TextArea(c, &draft.text).
				Height(height).
				Label(label).
				Tooltip(locales.DescriptionEditScope()).
				ReadOnly(err != nil || !value.Editable).
				Disabled(a.state.Busy).
				OnChange(func() { draft.preview, draft.err = nil, nil }),
		)
		if pending {
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, locales.PreviewText()).
					Disabled(a.state.Busy || unavailable).
					OnClick(func() { a.previewText(key, draft) })
				cancel := locales.Cancel()
				if stale || err != nil {
					cancel = locales.ReloadText()
				}
				ui.Button(c, cancel).
					Disabled(a.state.Busy).
					OnClick(func() { delete(a.texts, key) })
			})
		}
		if stale {
			ui.Text(c, locales.TextDraftStale()).TextColor(c.Theme().Danger)
		}
		switch {
		case err != nil:
			ui.Text(c, components.ErrorText(err)).
				FontSize(components.FontCaption).
				TextColor(c.Theme().Danger)
		case !value.Editable:
			ui.Text(c, components.ErrorText(value.ReadOnlyReason)).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
		if draft.err != nil {
			ui.Text(c, components.ErrorText(draft.err)).
				TextColor(c.Theme().Danger)
		}
		a.textPreview(c, key, draft, locales.ApplyText(), unavailable)
	})
}

func (a *form) nameEditor(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
) ui.Element {
	key := fieldKey{Kind: kind, Record: row.ID, Field: "name_id"}
	value, err := a.textSource(key)
	draft := a.textDraftFor(key, value, err)
	stale := err == nil && !draft.matches(value)
	pending := draft.modified() || stale
	unavailable := err != nil || !value.Editable || stale
	return ui.Column(c.Key("name-editor")).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if kind == records.Items && value.ID == 0 && !pending {
				a.itemTextAction(c, key, locales.Name(), 0)
				return
			}
			components.TextDraft(
				a.state,
				ui.TextInput(c, &draft.text).
					Placeholder(locales.Name()).
					Label(locales.Label(string(kind))+" / "+locales.Name()).
					Tooltip(locales.NameEditScope()).
					FontSize(components.FontTitle).
					Height(34).
					Grow(1).
					ReadOnly(err != nil || !value.Editable).
					Disabled(a.state.Busy).
					OnChange(func() { draft.preview, draft.err = nil, nil }),
			)
			if kind == records.Items {
				a.itemTextAction(c, key, locales.Name(), value.ID)
			} else if value.ID == 0 {
				ui.Button(c, locales.ChooseTextId()).
					Disabled(a.state.Busy).
					OnClick(func() { a.openTextReference(key) })
			}
			if pending {
				ui.Button(c, locales.PreviewName()).
					Disabled(a.state.Busy || unavailable).
					OnClick(func() { a.previewText(key, draft) })
				cancel := locales.Cancel()
				if stale || err != nil {
					cancel = locales.ReloadText()
				}
				ui.Button(c, cancel).
					Disabled(a.state.Busy).
					OnClick(func() { delete(a.texts, key) })
			}
		})
		if stale {
			ui.Text(c, locales.TextDraftStale()).TextColor(c.Theme().Danger)
		}
		if err != nil {
			ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
		}
		if draft.err != nil {
			ui.Text(c, components.ErrorText(draft.err)).
				TextColor(c.Theme().Danger)
		}
		a.textPreview(c, key, draft, locales.ApplyName(), unavailable)
	})
}
