package records

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type itemTextChoice struct {
	open      bool
	key       fieldKey
	session   *game.Session
	revision  uint64
	layout    records.Layout
	original  uint16
	selected  uint16
	reference components.TextReferenceState
	search    ui.Handle
	returnTo  ui.Handle
	err       error
}

func (a *form) itemTextAction(
	c *ui.Context,
	key fieldKey,
	label string,
	id uint16,
) {
	action := &a.textReferenceDraft(key).input
	if id == 0 {
		ui.Button(c, locales.ItemChooseText(label)+"…").
			Bind(action).
			Disabled(a.state.Busy).
			OnClick(func() { a.beginItemTextChoice(key) })
		return
	}
	ui.Button(c, "…").
		Size(26, 26).
		Padding(0).
		Bind(action).
		Label(locales.ItemTextActions(label)).
		Tooltip(locales.ItemTextActions(label)).
		Disabled(a.state.Busy).
		PopoverMenuButton(func(menu *ui.Menu) {
			if menu.Item(locales.ItemChangeText()).
				Disabled(a.state.Busy).
				Chosen() {
				a.beginItemTextChoice(key)
			}
			if menu.Item(fmt.Sprintf("%s (%04X)", locales.QuoteOpenText(), id)).
				Disabled(a.state.Busy || a.openText == nil).Chosen() {
				a.openText(id)
			}
		})
}

func (a *form) beginItemTextChoice(key fieldKey) {
	if a.state.Busy || key.Kind != records.Items {
		return
	}
	table, _, _, err := a.state.Session.Records.Resolve(
		key.Kind,
		key.Record,
		key.Field,
	)
	if err != nil {
		a.state.SetErr(err)
		return
	}
	value, err := a.state.Session.Records.Value(key.Kind, key.Record, key.Field)
	if err != nil {
		a.state.SetErr(err)
		return
	}
	if value < 0 || value > 65535 {
		a.state.SetErr(edit.ErrValue)
		return
	}
	choice := &itemTextChoice{
		open:     true,
		key:      key,
		session:  a.state.Session,
		revision: a.state.Session.Document.State().Revision,
		layout:   table.Layout,
		original: uint16(value),
		selected: uint16(value),
		returnTo: a.textReferenceDraft(key).input,
	}
	a.textChoice = choice
	choice.search.Focus()
}

func (choice *itemTextChoice) current(state *editor.State) bool {
	if !choice.open || state.Session != choice.session ||
		state.Session.Document.State().Revision != choice.revision {
		return false
	}
	table := state.Session.Records.Table(choice.key.Kind)
	if table == nil || table.Err != nil || table.Layout != choice.layout {
		return false
	}
	value, err := state.Session.Records.Value(
		choice.key.Kind,
		choice.key.Record,
		choice.key.Field,
	)
	return err == nil && value == int64(choice.original)
}

func (a *form) closeItemTextChoice(c *ui.Context) {
	if choice := a.textChoice; choice != nil {
		choice.open = false
		choice.search.CancelFocus(c)
		choice.returnTo.Focus(c)
		a.textChoice = nil
	}
}

func (a *form) chooseItemText(c *ui.Context) {
	choice := a.textChoice
	if choice == nil {
		return
	}
	width, height := c.Size()
	modal := ui.Modal(c.Key(choice), &choice.open, func() {
		ui.Column(c).
			Width(min(float32(520), max(float32(0), width-80))).
			Gap(12).
			Children(func() {
				label := locales.Label(
					strings.TrimSuffix(choice.key.Field, "_id"),
				)
				ui.Text(c, locales.ItemChooseText(label)).
					FontSize(components.FontPanelTitle).
					Bold()
				current := choice.current(a.state)
				ui.Scroll(c).
					MaxHeight(max(float32(80), height-220)).
					Gap(10).
					Children(func() {
						if !current {
							ui.Text(c, components.ErrorText(rom.ErrStale)).
								TextColor(c.Theme().Danger)
						}
						components.TextReference(c, a.state, &choice.reference, uint32(choice.selected),
							locales.ItemTextSearch(),
							!current, func(id uint16) {
								choice.selected, choice.err = id, nil
							}, nil).
							Bind(&choice.search)
						if draft := a.texts[choice.key]; draft != nil &&
							choice.selected != choice.original && draft.text != draft.original {
							ui.Text(c, locales.ItemTextDiscardDraft()).
								FontSize(components.FontCaption)
						}
						if choice.err != nil {
							ui.Text(c, components.ErrorText(choice.err)).
								TextColor(c.Theme().Danger)
						}
					})
				_, readErr := a.state.Session.Texts.Value(choice.selected)
				ui.Row(c).Gap(8).Children(func() {
					label := locales.ItemUseText()
					if choice.selected == 0 {
						label = locales.ItemClearText()
					}
					ui.PrimaryButton(c, label).
						Disabled(a.state.Busy || !current || readErr != nil).
						OnClick(func() { a.commitItemTextChoice(c, choice) })
					ui.Button(c, locales.Cancel()).
						OnClick(func() { a.closeItemTextChoice(c) })
				})
			})
	})
	if modal.Dismissed() {
		a.closeItemTextChoice(c)
	}
}

func (a *form) commitItemTextChoice(c *ui.Context, choice *itemTextChoice) {
	if a.textChoice != choice || a.state.Busy {
		return
	}
	if !choice.current(a.state) {
		choice.err = rom.ErrStale
		return
	}
	if choice.selected != choice.original {
		if _, err := a.state.Session.Texts.Value(choice.selected); err != nil {
			choice.err = err
			return
		}
		if err := a.state.Edit(func() error {
			return a.state.Session.Records.Set(
				choice.key.Kind,
				[]int{choice.key.Record},
				choice.key.Field,
				int64(choice.selected),
			)
		}); err != nil {
			choice.err = err
			return
		}
	}
	a.closeItemTextChoice(c)
}
