package chapterview

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *View) changeReview(
	c *ui.Context,
	state *components.State,
	row game.Row,
	entry *mapChangesState,
	jump func(game.Kind, int),
) {
	plan := entry.preview
	ui.Column(c.Key("changes-review")).Gap(16).Children(func() {
		ui.Column(c.Key("summary")).Gap(6).Children(func() {
			ui.Text(c, locales.MapChangesReview()).FontSize(16).Bold()
			ui.Textf(c, locales.MapChangesCount(), plan.Before(), plan.After()).
				FontFeatures("tnum")
		})
		mapUses(c, state, plan.Uses(), locales.MapChangesScope(), jump)
		ui.Column(c.Key("references")).
			Gap(6).
			Children(func() { v.changeReferences(c, state, entry, plan.References()) })
		if !plan.Applicable() {
			ui.Text(c, components.ErrorText(game.ErrMapChangeReference)).
				TextColor(c.Theme().Danger)
		}
		ui.Column(c.Key("details")).Gap(6).Children(func() {
			ui.Checkbox(c, &entry.details, locales.MapReviewDetails())
			if entry.details {
				ui.Textf(c, "0x%08X → 0x%08X · %d B", plan.Source(), plan.Destination(), plan.Length()).
					FontFeatures("tnum").
					Selectable()
				ui.Textf(c, locales.MapChangesPayloads(), plan.Payloads())
				ui.Text(c, locales.MapChangesScope())
				ui.Text(c, locales.MapChangesReferencesScope())
			}
		})
		ui.Column(c.Key("apply")).Gap(6).Children(func() {
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Button(c, locales.MapChangesApply()).
					Disabled(state.Busy || !entry.data.Current(state.Session) || !plan.Applicable()).
					OnClick(func() {
						state.Err = state.Session.ApplyMapChangesEdit(plan)
						if state.Err == nil {
							delete(v.changeDrafts, row.ID)
							state.HistoryEpoch++
						}
					})
				ui.Button(c, locales.MapBackToEdit()).
					Bind(&entry.reviewFocus).
					Disabled(state.Busy).
					OnClick(func() { entry.preview = nil; entry.editFocus.Focus(c) })
			})
			ui.Text(c, locales.MapEditScope()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		})
	})
}

func (v *View) changeReferenceRows(
	c *ui.Context,
	state *components.State,
	entry *mapChangesState,
	references []game.MapChangeReference,
) {
	entry.referenceList.Selected = &entry.referenceSelected
	entry.referenceList.Key = func(index int) any { return index }
	columns := []ui.TableColumn{
		{ID: "chapter", Title: locales.Chapters(), Width: 90, Fixed: true},
		{ID: "script", Title: locales.ScriptEditor()},
		{ID: "selector", Title: locales.MapChangeSelector()},
		{ID: "result", Title: locales.MapChangeTarget()},
	}
	ui.Table(c.Key("change-references"), &entry.referenceList, columns, len(references), func(index, column int) {
		ref := references[index]
		text := ""
		switch column {
		case 0:
			text = fmt.Sprintf("%02X", ref.ScriptChapter)
		case 1:
			text = fmt.Sprintf("0x%08X", ref.Address)
		case 2:
			if ref.ByPosition {
				text = fmt.Sprintf("%d,%d", ref.Position.X, ref.Position.Y)
			} else {
				text = fmt.Sprintf("ID %02X", ref.ID)
			}
		case 3:
			before, after := locales.MapChangeMissing(), locales.MapChangeMissing()
			if ref.Before >= 0 {
				before = fmt.Sprintf("%02X", ref.ID)
			}
			if ref.After >= 0 {
				after = fmt.Sprintf("%02X", ref.AfterID)
			}
			text = before
			if before != after {
				text += " → " + after
			} else if ref.Retargeted {
				text += " · " + locales.MapChangeRetargeted()
			}
		}
		ui.Text(c, text).SingleLine().FontFeatures("tnum").Tooltip(text)
	}).
		Height(min(180, float32(len(references)+1)*max(28, c.Theme().Space(8)))).
		Label(locales.MapChangesReferences() + " / " + locales.ScriptEditor())
	if entry.referenceSelected >= 0 &&
		entry.referenceSelected < len(references) {
		ref := references[entry.referenceSelected]
		ui.Button(c, locales.MapChangeOpenScript()).
			Disabled(state.Busy || v.openScript == nil).
			OnClick(func() { v.openScript(ref.ScriptChapter, ref.Pointer) })
	}
}

func (v *View) changeReferences(
	c *ui.Context,
	state *components.State,
	entry *mapChangesState,
	report game.MapChangeReferences,
) {
	ui.Text(c, locales.MapChangesReferences()).
		Bold().
		Tooltip(locales.MapChangesReferencesScope())
	if len(report.References) == 0 {
		ui.Text(c, locales.MapChangesNoReferences()).
			TextColor(c.Theme().TextMuted)
	} else {
		v.changeReferenceRows(c, state, entry, report.References)
	}
	if len(report.Warnings) > 0 {
		ui.Column(c.Key("incomplete")).Padding(8, 0, 0).Gap(6).Children(func() {
			ui.Text(c, locales.MapImpactWarnings()).Bold()
			ui.Scroll(c).MaxHeight(130).Gap(6).Children(func() {
				for _, warning := range report.Warnings {
					ui.Textf(c, "%02X · 0x%08X: %s", warning.Chapter, warning.Address, components.ErrorText(warning.Err)).
						Tooltip(components.ErrorText(warning.Err))
				}
			})
		})
	}
}
