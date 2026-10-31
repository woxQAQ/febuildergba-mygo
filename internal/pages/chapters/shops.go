package chapters

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type inventoryInput struct {
	item  string
	extra numberDraft
}

type inventoryState struct {
	selected int
	list     ui.ListState
	rows     []*inventoryInput
}

func (s *inventoryState) invalid() bool {
	for _, row := range s.rows {
		if row.extra.invalid {
			return true
		}
	}
	return false
}

func newInventoryInput(state *editor.State, item shop.Item) *inventoryInput {
	return &inventoryInput{
		item: fmt.Sprintf(
			"%02X · %s",
			item.ID,
			nameOf(state.Session, records.Items, int(item.ID)),
		),
		extra: numberDraft{text: fmt.Sprintf("0x%02X", item.Extra)},
	}
}

func (v *Page) shopKind(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	kind event.ShopKind,
) {
	if entry.inputs == nil {
		entry.inputs = make(map[inputKey]*numberDraft)
	}
	key := inputKey{entry.selected, "shop_kind"}
	value := entry.inputs[key]
	if value == nil {
		value = &numberDraft{}
		entry.inputs[key] = value
	}
	kinds := []event.ShopKind{event.Armory, event.Vendor, event.SecretShop}
	labels := []string{
		locales.ShopArmory(),
		locales.ShopVendor(),
		locales.ShopSecret(),
	}
	if value.value != int64(kind) || !slices.Contains(labels, value.text) {
		value.text, value.value = labels[slices.Index(kinds, kind)], int64(kind)
	}
	ui.Select(c.Key("shop-kind"), &value.text, labels).
		Label(locales.ShopKind()).Width(180).MaxWidthPercent(100).
		Disabled(state.Busy || !entry.data.Current(state.Session.Events)).
		OnChange(func() {
			if index := slices.Index(labels, value.text); index >= 0 {
				state.Err = entry.data.SetShopKind(entry.selected, kinds[index])
				entry.preview = nil
			}
		})
}

func (v *Page) shopInventory(
	c *ui.Context,
	state *editor.State,
	entry *eventState,
	condition event.Condition,
	advanced bool,
) {
	v.shopKind(c, state, entry, condition.Location.ShopKind)
	inventory, err := entry.data.Shop(entry.selected)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).
			TextColor(c.Theme().Danger).
			Tooltip(err.Error())
		return
	}
	if entry.shops == nil {
		entry.shops = make(map[*shop.Draft]*inventoryState)
	}
	view := entry.shops[inventory]
	if view == nil {
		view = &inventoryState{}
		for _, item := range inventory.Items() {
			view.rows = append(view.rows, newInventoryInput(state, item))
		}
		entry.shops[inventory] = view
	}
	disabled := state.Busy || !entry.data.Current(state.Session.Events)
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
				ui.Textf(c, "%02X · %s", items[index].ID, nameOf(state.Session, records.Items, int(items[index].ID))).
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
						newInventoryInput(state, shop.Item{ID: 1}),
					)
					view.selected, entry.preview = inventory.Count()-1, nil
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
					view.selected, entry.preview = min(
						view.selected,
						max(0, inventory.Count()-1),
					), nil
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
						view.selected, entry.preview = to, nil
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
				nameOf(state.Session, records.Items, id),
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
						entry.preview = nil
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
						entry.preview = nil
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
