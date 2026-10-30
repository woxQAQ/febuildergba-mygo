package records

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type textDraft struct {
	text, original string
	textID         uint16
	epoch          uint64
	preview        *text.Edit
	err            error
}

func (a *form) textDraftFor(
	key fieldKey,
	id uint16,
	original string,
) *textDraft {
	if a.texts == nil {
		a.texts = make(map[fieldKey]*textDraft)
	}
	draft := a.texts[key]
	if draft == nil || draft.original != original || draft.textID != id ||
		draft.epoch != a.state.HistoryEpoch {
		draft = &textDraft{
			text:     original,
			original: original,
			textID:   id,
			epoch:    a.state.HistoryEpoch,
		}
		a.texts[key] = draft
	}
	if draft.preview != nil &&
		draft.preview.Revision() != a.state.Session.Document.State().Revision {
		draft.preview = nil
	}
	return draft
}

func (a *form) textPreview(
	c *ui.Context,
	key fieldKey,
	draft *textDraft,
	label string,
) {
	edit := draft.preview
	if edit == nil {
		return
	}
	uses := edit.Uses()
	ui.Text(c, locales.TextEditSummary(fmt.Sprintf("0x%04X", edit.TextID()), len(uses))).
		FontSize(12)
	for _, kind := range records.Kinds {
		var fields []string
		for _, use := range uses {
			if use.Kind == string(kind) {
				fields = append(
					fields,
					fmt.Sprintf("%02X (%s)", use.ID, locales.Label(use.Field)),
				)
			}
		}
		if len(fields) > 0 {
			ui.Text(c, locales.Label(string(kind))+": "+strings.Join(fields, ", ")).
				FontSize(12)
		}
	}
	ui.Button(c, label).Disabled(a.state.Busy).OnClick(func() {
		if err := a.state.Session.Texts.Apply(edit); err != nil {
			draft.err, draft.preview = err, nil
			return
		}
		delete(a.texts, key)
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
		if field.Text != text.Description {
			continue
		}
		value, err := a.state.Session.Records.Text(kind, row.ID, field.ID)
		key := fieldKey{Kind: kind, Record: row.ID, Field: field.ID}
		draft := a.textDraftFor(key, value.ID, value.Content)
		label := locales.Label(strings.TrimSuffix(field.ID, "_id"))
		ui.Column(c.Key(field.ID + "-editor")).Gap(6).Children(func() {
			if err == nil && value.ID == 0 {
				ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).Bold()
					ui.Text(c, locales.TextNeedsId()).
						TextColor(c.Theme().TextMuted).
						Grow(1)
					ui.Button(c, locales.ChooseTextId()).
						Disabled(a.state.Busy).
						OnClick(func() {
							if kind == records.Units ||
								kind == records.Classes {
								a.profileFor(kind, row.ID).section = 1
							}
							a.focusField(key)
						})
				})
				return
			}
			if height > 64 {
				ui.Text(c, label).Bold()
			}
			unavailable := err != nil || !value.Editable
			components.TextDraft(
				a.state,
				ui.TextArea(c, &draft.text).
					Height(height).
					Label(label).
					Tooltip(locales.DescriptionEditScope()).
					ReadOnly(unavailable).
					Disabled(a.state.Busy).
					OnChange(func() {
						draft.preview, draft.err = nil, nil
					}),
			)
			if draft.text != value.Content {
				ui.Row(c).Gap(8).Children(func() {
					ui.Button(c, locales.PreviewText()).
						Disabled(a.state.Busy || unavailable).
						OnClick(func() {
							draft.preview, draft.err = a.state.Session.Records.PlanTextEdit(
								kind,
								row.ID,
								field.ID,
								draft.text,
							)
						})
					ui.Button(c, locales.Cancel()).
						Disabled(a.state.Busy).
						OnClick(func() {
							draft.text, draft.preview, draft.err = value.Content, nil, nil
						})
				})
			}
			switch {
			case err != nil:
				ui.Text(c, components.ErrorText(err)).
					FontSize(12).
					TextColor(c.Theme().Danger)
			case !value.Editable:
				ui.Text(c, components.ErrorText(value.ReadOnlyReason)).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
			}
			if draft.err != nil {
				ui.Text(c, components.ErrorText(draft.err)).
					TextColor(c.Theme().Danger)
			}
			a.textPreview(c, key, draft, locales.ApplyText())
		})
	}
}

func (a *form) nameEditor(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
) ui.Element {
	key := fieldKey{Kind: kind, Record: row.ID, Field: "name_id"}
	draft := a.textDraftFor(key, row.NameID, row.Name)
	return ui.Column(c.Key("name-editor")).Gap(6).Children(func() {
		unavailable := row.NameID == 0 || row.TextError != nil
		ui.Row(c).Gap(8).Children(func() {
			components.TextDraft(
				a.state,
				ui.TextInput(c, &draft.text).
					Label(locales.Label(string(kind))+" / "+locales.Name()).
					Tooltip(locales.NameEditScope()).
					FontSize(20).
					Height(34).
					Grow(1).
					Disabled(a.state.Busy || unavailable).
					OnChange(func() {
						draft.preview, draft.err = nil, nil
					}),
			)
			if draft.text != row.Name {
				ui.Button(c, locales.PreviewName()).
					Disabled(a.state.Busy || unavailable).
					OnClick(func() {
						draft.preview, draft.err = a.state.Session.Records.PlanTextEdit(
							kind,
							row.ID,
							key.Field,
							draft.text,
						)
					})
				ui.Button(c, locales.Cancel()).
					Disabled(a.state.Busy).
					OnClick(func() {
						draft.text, draft.preview, draft.err = row.Name, nil, nil
					})
			}
		})
		if row.NameID == 0 {
			ui.Text(c, locales.NameNeedsId()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
		if draft.err != nil {
			ui.Text(c, components.ErrorText(draft.err)).
				TextColor(c.Theme().Danger)
		}
		a.textPreview(c, key, draft, locales.ApplyName())
	})
}
