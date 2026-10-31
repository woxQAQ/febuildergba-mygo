// Package shops provides one inventory workflow for chapter and native shop entries.
package shops

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/shopview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type entryState struct {
	entry          shop.Entry
	native         *shop.EntryDraft
	chapter        *event.ConditionDraft
	inventory      *shop.Draft
	view           shopview.View
	preview        *shop.EntryEdit
	chapterPreview *event.ConditionEdit
	uses, affected []shop.Reference
	err            error
}

func (entry *entryState) current(s *game.Session) bool {
	if entry.chapter != nil {
		return entry.chapter.Current(s.Events)
	}
	return entry.native != nil && entry.native.Current(s.Shops)
}

func (entry *entryState) modified() bool {
	if entry.chapter != nil {
		return entry.chapter.Modified()
	}
	return entry.native != nil && entry.native.Modified()
}

func (entry *entryState) changed() {
	entry.preview, entry.chapterPreview, entry.uses, entry.affected = nil, nil, nil, nil
}

type Page struct {
	state              *editor.State
	openChapter        func(int, int)
	openHex            func(int, int)
	sidebar            components.Sidebar
	query              string
	category, selected int
	advanced           bool
	showInactive       bool
	list               ui.ListState
	rows               []shop.Entry
	all                []shop.Entry
	warnings           []error
	drafts             map[shop.Key]*entryState
}

func New(state *editor.State, openChapter, openHex func(int, int)) *Page {
	return &Page{
		state:       state,
		openChapter: openChapter,
		openHex:     openHex,
		selected:    -1,
		drafts:      make(map[shop.Key]*entryState),
	}
}

func (page *Page) Reset() {
	state, chapter, hex, sidebar := page.state, page.openChapter, page.openHex, page.sidebar
	*page = *New(state, chapter, hex)
	page.sidebar = sidebar
}

func (page *Page) refresh() {
	var selected *shop.Key
	if page.selected >= 0 && page.selected < len(page.rows) {
		key := page.rows[page.selected].Key
		selected = &key
	}
	page.all, page.warnings = page.state.Session.Shops.Entries()
	page.rows = nil
	query := strings.ToLower(strings.TrimSpace(page.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 16)
	for _, entry := range page.all {
		if page.category > 0 && int(entry.Key.Origin) != page.category-1 {
			continue
		}
		if inactiveWorldMap(entry) && !page.showInactive &&
			!page.pending(entry.Key) {
			continue
		}
		if query == "" ||
			strings.Contains(
				strings.ToLower(shopview.Caption(entry.Reference)),
				query,
			) ||
			idErr == nil && int(id) == entry.Key.ID {
			page.rows = append(page.rows, entry)
		}
	}
	page.selected = -1
	for i, entry := range page.rows {
		if selected != nil && entry.Key == *selected {
			page.selected = i
			break
		}
	}
	if page.selected < 0 && len(page.rows) > 0 {
		page.selected = 0
	}
}

func inactiveWorldMap(entry shop.Entry) bool {
	return entry.Key.Origin == shop.WorldMap && (entry.Null || entry.Empty)
}

func (page *Page) pending(key shop.Key) bool {
	entry := page.drafts[key]
	return entry != nil && (entry.modified() || entry.view.Invalid())
}

func (page *Page) caption(entry shop.Entry) string {
	caption := shopview.Caption(entry.Reference)
	status := ""
	if inactiveWorldMap(entry) {
		status = locales.ShopInactive()
		if entry.Null {
			status = locales.ShopUninitialized()
		}
	}
	if page.pending(entry.Key) {
		status = locales.ShopUnapplied()
	}
	if status != "" {
		return status + " · " + caption
	}
	return caption
}

func (page *Page) View(c *ui.Context) {
	page.refresh()
	page.sidebar.Render(
		c,
		300,
		func() { page.navigation(c) },
		func() { page.detail(c) },
	)
}

func (page *Page) navigation(c *ui.Context) {
	ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
		components.TextDraft(page.state, ui.TextInput(c, &page.query).
			Label(locales.ShopSearch()).
			Placeholder(locales.ShopSearch()).Disabled(page.state.Busy).
			OnChange(func() { page.selected = -1; page.list = ui.ListState{} }))
		ui.Tabs(c, &page.category, locales.ShopAll(), locales.ShopChapter(), locales.ShopPreparation(), locales.ShopDefault(), locales.ShopWorldmap()).
			Wrap().
			Label(locales.ShopSource()).
			Tooltip(locales.ShopVersionHelp()).
			Disabled(page.state.Busy).
			OnChange(func() { page.selected = -1; page.list = ui.ListState{} })
		if (page.category == 0 || page.category == int(shop.WorldMap)+1) &&
			slices.ContainsFunc(
				page.all,
				func(entry shop.Entry) bool { return entry.Key.Origin == shop.WorldMap },
			) {
			ui.Checkbox(c.Key("show-inactive"), &page.showInactive, locales.ShopShowInactive()).
				Disabled(page.state.Busy).
				OnChange(func() { page.list = ui.ListState{} })
		}
		page.list.Selected = &page.selected
		page.list.Key = func(index int) any { return page.rows[index].Key }
		ui.Table(c, &page.list, []ui.TableColumn{{ID: "shop", Title: locales.ShopEntries()}}, len(page.rows), func(row, _ int) {
			entry := page.rows[row]
			caption := page.caption(entry)
			ui.Text(c, caption).SingleLine().FontSize(13).Tooltip(caption)
		}).
			Grow(1).
			Label(locales.ShopEntries()).
			Disabled(page.state.Busy)
		if len(page.rows) == 0 {
			ui.Text(c, locales.ShopNoEntries()).TextColor(c.Theme().TextMuted)
		}
		if len(page.warnings) > 0 {
			ui.Textf(c, "%s: %d", locales.ShopIncomplete(), len(page.warnings)).
				FontSize(12).
				TextColor(c.Theme().TextMuted).
				Tooltip(page.warnings[0].Error())
		}
	})
}

func (page *Page) draft(source shop.Entry) *entryState {
	s := page.state.Session
	entry := page.drafts[source.Key]
	if entry != nil && !entry.current(s) && !entry.modified() &&
		!entry.view.Invalid() {
		entry = nil
	}
	if entry == nil {
		entry = &entryState{entry: source}
		if source.Key.Origin == shop.Chapter {
			entry.chapter, entry.err = s.Events.NewConditions(
				source.Key.ID,
				event.LocationEvents,
			)
			if entry.err == nil {
				entry.inventory, entry.err = entry.chapter.Shop(source.Key.Slot)
			}
		} else {
			entry.native, entry.err = s.Shops.NewDraft(source.Key)
			if entry.err == nil {
				entry.inventory = entry.native.Inventory()
			}
		}
		page.drafts[source.Key] = entry
	}
	return entry
}

func (page *Page) detail(c *ui.Context) {
	if page.selected < 0 || page.selected >= len(page.rows) {
		return
	}
	entry := page.draft(page.rows[page.selected])
	ui.Scroll(c.Key(entry)).Fill().Children(func() {
		ui.Column(c).Padding(16).Gap(12).Children(func() {
			ui.Text(c, shopview.Caption(entry.entry.Reference)).
				FontSize(18).
				Bold()
			if entry.entry.UsesDefault {
				ui.Text(c, locales.ShopUsesDefault()).FontSize(13)
			}
			if entry.entry.Key.Origin == shop.Default {
				ui.Text(c, locales.ShopDefaultHelp()).FontSize(13)
			}
			if entry.entry.Key.Origin == shop.WorldMap {
				ui.Text(c, locales.ShopWorldmapHelp()).FontSize(13)
				if entry.entry.Null && entry.inventory == nil {
					ui.Text(c, locales.ShopNullHelp()).
						TextColor(c.Theme().Danger)
				}
				if entry.inventory != nil && entry.inventory.WorldMapHidden() {
					message := locales.ShopHidden()
					if entry.modified() {
						message = locales.ShopClosePending()
					}
					ui.Text(c, message).TextColor(c.Theme().TextMuted)
				} else if entry.inventory != nil && entry.inventory.Count() == 0 {
					ui.Text(c, locales.ShopEmptyVisible()).FontSize(13)
				}
			}
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					Tooltip(entry.err.Error()).
					TextColor(c.Theme().Danger)
			}
			stale := !entry.current(page.state.Session)
			if stale && entry.modified() {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Checkbox(c, &page.advanced, locales.Advanced())
				if entry.entry.Key.Origin == shop.Chapter {
					ui.Button(c, locales.ShopOpenChapter()).
						Disabled(page.state.Busy).
						OnClick(func() { page.openChapter(entry.entry.Key.ID, entry.entry.Key.Slot) })
				} else if entry.native != nil {
					label := locales.ShopClear()
					if entry.inventory == nil {
						label = locales.ShopInitialize()
					} else if entry.entry.Key.Origin == shop.WorldMap {
						label = locales.ShopClose()
					}
					closed := entry.entry.Key.Origin == shop.WorldMap &&
						entry.inventory != nil && entry.inventory.WorldMapHidden()
					ui.Button(c, label).
						Disabled(page.state.Busy || stale || closed).
						OnClick(func() {
							entry.native.Clear()
							entry.inventory = entry.native.Inventory()
							entry.changed()
						})
				}
			})
			if entry.inventory != nil {
				entry.view.Render(
					c,
					page.state,
					entry.inventory,
					page.state.Busy || stale,
					page.advanced,
					entry.changed,
				)
			}
			page.actions(c, entry)
			if page.advanced {
				ui.Row(c).Gap(8).Wrap().Children(func() {
					ui.Button(c, locales.DeployReference()).
						Disabled(page.state.Busy).
						OnClick(func() { page.openHex(entry.entry.Pointer, 4) })
					ui.Button(c, locales.ShopRaw()).
						Disabled(page.state.Busy || entry.entry.Address == 0).
						OnClick(func() { page.openHex(entry.entry.Address, shop.PayloadSize) })
				})
			}
		})
	})
}

func (page *Page) actions(c *ui.Context, entry *entryState) {
	state, s := page.state, page.state.Session
	stale := !entry.current(s)
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.ShopPreview()).
			Disabled(state.Busy || stale || !entry.modified() || entry.view.Invalid()).
			OnClick(func() {
				entry.changed()
				if entry.chapter != nil {
					entry.chapterPreview, state.Err = s.Events.PlanConditions(
						entry.chapter,
					)
					if state.Err == nil {
						for _, known := range page.all {
							if entry.entry.Address != 0 &&
								known.Address == entry.entry.Address {
								entry.uses = append(entry.uses, known.Reference)
							}
							if known.Key.Origin == shop.Chapter &&
								known.Key.Slot == entry.entry.Key.Slot &&
								slices.Contains(
									entry.chapterPreview.Uses(),
									known.Key.ID,
								) {
								entry.affected = append(
									entry.affected,
									known.Reference,
								)
							}
						}
					}
				} else {
					entry.preview, state.Err = s.Shops.Plan(entry.native)
				}
			})
		ui.Button(c, locales.DeployReload()).
			Disabled(state.Busy).
			OnClick(func() { delete(page.drafts, entry.entry.Key) })
	})
	if entry.preview == nil && entry.chapterPreview == nil {
		return
	}
	ui.Column(c.Key("preview")).Gap(6).Children(func() {
		var before, after, count int
		uses := entry.uses
		var affected []shop.Reference
		if plan := entry.preview; plan != nil {
			before, after, count = plan.Before(), plan.After(), plan.Count()
			uses, affected = plan.Uses(), plan.Affected()
			ui.Text(c, locales.ShopEntryScope()).FontSize(12)
		} else {
			for _, change := range entry.chapterPreview.Inventories() {
				before, after, count = change.Before, change.After, change.Count
			}
			ui.Text(c, locales.ShopPreviewScope()).FontSize(12)
			affected = entry.affected
		}
		ui.Textf(c, "%s · %d/%d · 0x%08X → 0x%08X", locales.ShopInventory(), count, shop.MaxItems, before, after).
			Selectable()
		ui.Text(c, locales.ShopAffected()).FontSize(12).Bold()
		for _, ref := range affected {
			ui.Text(c, shopview.Caption(ref)).FontSize(12)
		}
		if len(uses) > 0 {
			ui.Text(c, locales.ShopKnownEntries()).FontSize(12).Bold()
			for _, ref := range uses {
				ui.Text(c, shopview.Caption(ref)).FontSize(12)
			}
		}
		ui.Text(c, locales.ShopReferenceLimit()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		ui.PrimaryButton(c, locales.ShopApply()).
			Disabled(state.Busy || stale || entry.view.Invalid()).
			OnClick(func() {
				if entry.chapterPreview != nil {
					state.Err = s.Events.ApplyConditions(entry.chapterPreview)
				} else {
					state.Err = s.Shops.Apply(entry.preview)
				}
				if state.Err == nil {
					delete(page.drafts, entry.entry.Key)
					state.HistoryEpoch++
				}
			})
	})
}
