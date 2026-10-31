package chapters

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/shopview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (v *Page) OpenShop(state *editor.State, chapter, location int) {
	table := state.Session.Records.Table(records.Chapters)
	if table == nil || table.Err != nil {
		state.Err = edit.ErrLayout
		return
	}
	for _, row := range table.Rows {
		if row.ID != chapter {
			continue
		}
		v.load(state, row)
		entry, err := v.eventDraft(
			state,
			eventKey{chapter: chapter, table: event.LocationEvents},
		)
		if err != nil {
			state.Err = err
			return
		}
		if location < 0 || location >= len(entry.data.Events()) {
			state.Err = edit.ErrField
			return
		}
		v.mode = 8
		entry.selected = location
		entry.list.ScrollTo(location, ui.Center)
		return
	}
	state.Err = edit.ErrField
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
		entry.shops = make(map[*shop.Draft]*shopview.View)
	}
	view := entry.shops[inventory]
	if view == nil {
		view = &shopview.View{}
		entry.shops[inventory] = view
	}
	view.Render(
		c,
		state,
		inventory,
		state.Busy || !entry.data.Current(state.Session.Events),
		advanced,
		func() { entry.preview = nil },
	)
}
