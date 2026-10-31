// Package shopview renders inventory drafts shared by chapter and standalone shops.
package shopview

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type inventoryInput struct {
	item  string
	extra struct {
		text    string
		invalid bool
	}
}

type View struct {
	selected  int
	list      ui.ListState
	rows      []*inventoryInput
	inventory *shop.Draft
}

func (view *View) Invalid() bool {
	for _, row := range view.rows {
		if row.extra.invalid {
			return true
		}
	}
	return false
}

func itemName(state *editor.State, id int) string {
	if table := state.Session.Records.Table(
		records.Items,
	); table != nil &&
		table.Err == nil {
		for _, row := range table.Rows {
			if row.ID == id {
				return row.Name
			}
		}
	}
	return fmt.Sprintf("0x%02X", id)
}

func newInput(state *editor.State, item shop.Item) *inventoryInput {
	row := &inventoryInput{
		item: fmt.Sprintf("%02X · %s", item.ID, itemName(state, int(item.ID))),
	}
	row.extra.text = fmt.Sprintf("0x%02X", item.Extra)
	return row
}

func (view *View) Render(
	c *ui.Context,
	state *editor.State,
	inventory *shop.Draft,
	disabled, advanced bool,
	changed func(),
) {
	if view.inventory != inventory {
		*view = View{inventory: inventory}
		for _, item := range inventory.Items() {
			view.rows = append(view.rows, newInput(state, item))
		}
	}
	view.selected = max(0, min(view.selected, inventory.Count()-1))
	items := inventory.Items()
	ui.Textf(c, "%d / %d", len(items), shop.MaxItems).
		FontSize(12).
		Tooltip(locales.ShopCapacityHelp())
	if len(items) == 0 {
		ui.Text(c, locales.ShopEmpty()).
			FontSize(13).
			TextColor(c.Theme().TextMuted)
	} else {
		view.list.Selected = &view.selected
		view.list.Key = func(index int) any { return view.rows[index] }
		ui.Table(c.Key(inventory), &view.list, []ui.TableColumn{
			{ID: "index", Title: "#", Width: 36, Fixed: true},
			{ID: "item", Title: locales.ShopItem()},
		}, len(items), func(index, column int) {
			if column == 0 {
				ui.Textf(c, "%d", index+1).FontFeatures("tnum")
			} else {
				ui.Textf(c, "%02X · %s", items[index].ID, itemName(state, int(items[index].ID))).
					SingleLine()
			}
		}).Height(min(152, c.Theme().Space(9)*float32(len(items)+1))).Label(locales.ShopInventory())
	}
	// Removing the selected key can leave the list without a selection during
	// this build. Keep the inspector on the nearest remaining inventory row.
	view.selected = max(0, min(view.selected, len(items)-1))
	ui.Row(c).Wrap().Gap(6).Children(func() {
		ui.Button(c, locales.ShopAdd()).
			Disabled(disabled || len(items) >= shop.MaxItems).
			OnClick(func() {
				if state.Err = inventory.Add(1); state.Err == nil {
					view.rows = append(
						view.rows,
						newInput(state, shop.Item{ID: 1}),
					)
					view.selected = inventory.Count() - 1
					changed()
					view.list.ScrollTo(view.selected, ui.Center)
				}
			})
		ui.Button(c, locales.ShopRemove()).
			Disabled(disabled || len(items) == 0).
			OnClick(func() {
				if state.Err = inventory.Remove(
					view.selected,
				); state.Err == nil {
					view.rows = slices.Delete(
						view.rows,
						view.selected,
						view.selected+1,
					)
					view.selected = min(
						view.selected,
						max(0, inventory.Count()-1),
					)
					changed()
				}
			})
		for _, direction := range []int{-1, 1} {
			label, caption := "↑", locales.MoveUp()
			if direction > 0 {
				label, caption = "↓", locales.MoveDown()
			}
			ui.Button(c.Key(direction), label).Label(caption).Tooltip(caption).
				Disabled(disabled || view.selected+direction < 0 || view.selected+direction >= len(items)).
				OnClick(func() {
					from, to := view.selected, view.selected+direction
					if state.Err = inventory.Move(from, to); state.Err == nil {
						row := view.rows[from]
						view.rows = slices.Delete(view.rows, from, from+1)
						view.rows = slices.Insert(view.rows, to, row)
						view.selected = to
						changed()
					}
				})
		}
	})
	if len(items) == 0 {
		return
	}
	index := view.selected
	row := view.rows[index]
	ui.Column(c.Key(row)).Gap(8).Children(func() {
		choices := make([]string, 255)
		for id := 1; id <= 255; id++ {
			choices[id-1] = fmt.Sprintf(
				"%02X · %s",
				id,
				itemName(state, id),
			)
		}
		components.TextDraft(state,
			ui.Combobox(c.Key("item"), &row.item, choices).
				WidthPercent(100).
				Label(locales.ShopInventory()+" / "+locales.ShopItem()).
				Tooltip(locales.ShopItemHelp()).Disabled(disabled).
				OnChange(func() {
					if choice := slices.Index(choices, row.item); choice >= 0 {
						state.Err = inventory.SetItem(index, choice+1)
						changed()
					}
				}))
		if advanced || row.extra.invalid {
			ui.Text(c, locales.ShopExtra()).
				FontSize(12).
				Tooltip(locales.ShopExtraHelp())
			extra := components.TextDraft(
				state,
				ui.TextInput(c.Key("extra"), &row.extra.text).
					Width(92).
					Label(locales.ShopInventory()+" / "+locales.ShopExtra()).
					Disabled(disabled).
					OnChange(func() {
						value, parseErr := input.ParseInteger(row.extra.text)
						changed()
						if parseErr != nil || value < 0 || value > 255 {
							row.extra.invalid = true
							return
						}
						state.Err = inventory.SetExtra(index, int(value))
						row.extra.invalid = state.Err != nil
					}),
			)
			if row.extra.invalid {
				extra.Error(fmt.Sprintf("%s: 0…255", locales.AllowedRange()))
			}
		}
	})
}
