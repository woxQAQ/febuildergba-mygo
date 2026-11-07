package records

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/itemicon"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

const (
	itemRecords = iota
	itemIconTable
)

type itemIconState struct {
	query       string
	selected    int
	list        ui.ListState
	source      itemicon.Table
	previews    map[int]*itemIconPreview
	uses        map[int][]records.Row
	usesErr     error
	target      *itemIconTarget
	imageEditor components.AssetEditor
	assetSource itemicon.Table
	assetBound  bool
	assetIcon   int
	generation  uint64
	usesList    ui.ListState
	useSelected int
}

type itemIconPreview struct {
	bitmap *ui.Bitmap
	err    error
}

type itemIconTarget struct {
	session  *game.Session
	revision uint64
	layout   records.Layout
	item     int
	original int64
	name     string
}

func (target *itemIconTarget) current(state *editor.State) bool {
	if state.Session != target.session ||
		state.Session.Document.State().Revision != target.revision {
		return false
	}
	table := state.Session.Records.Table(records.Items)
	if table == nil || table.Err != nil || table.Layout != target.layout {
		return false
	}
	value, err := state.Session.Records.Value(
		records.Items,
		target.item,
		"icon_id",
	)
	return err == nil && value == target.original
}

func (a *Page) itemNavigation(c *ui.Context) {
	ui.Column(c.Key("item-navigation")).Fill().Children(func() {
		ui.Row(c).Padding(12, 12, 0, 12).Children(func() {
			ui.Tabs(c, &a.itemMode, locales.Label(string(records.Items)), locales.ItemIconTable()).
				Label(locales.ItemNavigation()).
				OnChange(func() {
					a.form.textChoice = nil
					a.panes[a.tab].layout.move = nil
					if a.itemMode == itemRecords {
						a.icons.target = nil
						a.cancelIconEdit()
					}
				})
		})
		if a.itemMode == itemIconTable {
			a.iconRecords(c)
		} else {
			a.records(c)
		}
	})
}

func (a *Page) iconSource() *itemIconState {
	state := &a.icons
	if !state.source.Current(a.state.Session.ItemIcons) {
		state.source = a.state.Session.ItemIcons.Table()
		state.previews = make(map[int]*itemIconPreview)
		state.uses, state.usesErr = a.state.Session.Records.ItemIconUses()
	}
	return state
}

func (state *itemIconState) rows() []int {
	query := strings.ToLower(strings.TrimSpace(state.query))
	parsed, parseErr := strconv.ParseUint(
		strings.TrimPrefix(query, "0x"),
		16,
		8,
	)
	var rows []int
	for id := range state.source.Count {
		match := query == "" || parseErr == nil && id == int(parsed)
		for _, row := range state.uses[id] {
			match = match || strings.Contains(strings.ToLower(row.Name), query)
		}
		if match {
			rows = append(rows, id)
		}
	}
	return rows
}

func (state *itemIconState) preview(a *Page, id int) *itemIconPreview {
	if preview := state.previews[id]; preview != nil {
		return preview
	}
	resource, err := a.state.Session.ItemIcons.Preview(id)
	preview := &itemIconPreview{err: err}
	if resource.Image != nil {
		preview.bitmap = ui.NewBitmap(resource.Image)
		preview.bitmap.Pixelated()
	}
	state.previews[id] = preview
	return preview
}

func (state *itemIconState) caption(id int) string {
	if state.usesErr != nil {
		return locales.ItemIconUnknownUses()
	}
	if len(state.uses[id]) == 0 {
		return locales.ItemIconUnused()
	}
	var names []string
	for _, row := range state.uses[id] {
		name := row.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("%02X", row.ID)
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

func (a *Page) iconRecords(c *ui.Context) {
	state := a.iconSource()
	ui.Column(c.Key("icon-records")).Fill().Gap(8).Padding(12).Children(func() {
		components.TextDraft(
			a.state,
			ui.TextInput(c.Key("search"), &state.query).
				Label(locales.ItemIconSearch()).
				Placeholder(locales.ItemIconSearch()).
				OnChange(func() { state.selected = 0; state.list = ui.ListState{} }),
		)
		if state.source.Err != nil {
			ui.Text(c, components.ErrorText(state.source.Err)).
				TextColor(c.Theme().Danger)
			return
		}
		rows := state.rows()
		if state.selected < 0 || state.selected >= len(rows) {
			state.selected = 0
		}
		state.list.Selected = &state.selected
		state.list.Key = func(index int) any { return rows[index] }
		columns := []ui.TableColumn{
			{ID: "id", Title: "ID", Width: 44, Fixed: true},
			{ID: "image", Title: "", Width: 28, Fixed: true},
			{ID: "uses", Title: locales.ItemIconUses()},
		}
		ui.Table(c.Key("icons"), &state.list, columns, len(rows), func(row, col int) {
			id := rows[row]
			switch col {
			case 0:
				ui.Textf(c, "%02X", id).FontFeatures("tnum")
			case 1:
				if preview := state.preview(a, id); preview.bitmap != nil {
					ui.Image(c, preview.bitmap).
						Size(24, 24).
						Label(fmt.Sprintf("%s %02X", locales.ItemIconTable(), id))
				}
			case 2:
				ui.Text(c, state.caption(id)).SingleLine()
			}
		}).
			Grow(1).
			Label(locales.ItemIconTable())
		ui.Text(c, locales.RecordSummary(len(rows))).
			TextColor(c.Theme().TextMuted)
	})
}

func (a *Page) iconDetails(c *ui.Context) {
	state := a.iconSource()
	rows := state.rows()
	width, _ := c.Size()
	wide := width-a.panes[a.tab].sidebar.Width()-32 >= 680
	ui.Column(c.Key("icon-details")).
		Grow(1).
		MinHeight(0).
		Padding(16).
		Gap(16).
		Children(func() {
			if state.source.Err != nil {
				ui.Text(c, components.ErrorText(state.source.Err)).
					TextColor(c.Theme().Danger)
				return
			}
			if len(rows) == 0 || state.selected < 0 ||
				state.selected >= len(rows) {
				ui.Text(c, locales.NoResults())
				return
			}
			id := rows[state.selected]
			preview := state.preview(a, id)
			if !state.assetBound || state.assetIcon != id ||
				!state.assetSource.Current(a.state.Session.ItemIcons) {
				a.cancelIconEdit()
				state.assetBound, state.assetIcon, state.assetSource = true, id, state.source
				state.usesList, state.useSelected = ui.ListState{}, -1
			}
			ui.Textf(c, "%s %02X", locales.ItemIconTable(), id).
				FontSize(components.FontPanelTitle).
				Bold()
			var body ui.Element
			if wide {
				body = ui.Row(c.Key("icon-content")).
					AlignItems(ui.Stretch).
					Gap(24)
			} else {
				body = ui.Column(c.Key("icon-content")).Gap(16)
			}
			body.Grow(1).MinHeight(0).Children(func() {
				artwork := ui.Scroll(c.Key("icon-artwork")).
					Gap(16).
					MinHeight(0).
					MinWidth(0)
				if wide {
					artwork.Width(280)
				} else {
					artwork.Basis(160).Grow(1)
				}
				artwork.Children(func() { a.iconArtwork(c, id, preview) })
				uses := ui.Column(c.Key("icon-users")).
					Gap(8).
					MinWidth(0).
					MinHeight(0).
					Grow(1)
				if !wide {
					uses.Basis(200).Grow(2)
				}
				uses.Children(func() { a.iconUses(c, id) })
			})
		})
}

func (a *Page) iconArtwork(c *ui.Context, id int, preview *itemIconPreview) {
	state := &a.icons
	if preview.err != nil {
		ui.Text(c, components.ErrorText(preview.err)).
			TextColor(c.Theme().Danger)
	}
	ui.Row(c).Wrap().Gap(16).AlignItems(ui.Start).Children(func() {
		if preview.bitmap != nil {
			ui.Image(c, preview.bitmap).
				Size(128, 128).
				Label(locales.ImagePreview())
		}
		ui.Column(c).Gap(12).Grow(1).MinWidth(176).Children(func() {
			if preview.bitmap != nil {
				state.imageEditor.RenderButtons(
					c,
					a.state,
					graphics.Ref{Kind: graphics.ItemIcon, ID: id},
					a.iconAssetActions(),
				)
			}
			a.iconTargetActions(c, id, preview)
		})
	})
	state.imageEditor.RenderPreview(c, a.state)
}

func (a *Page) iconTargetActions(
	c *ui.Context,
	id int,
	preview *itemIconPreview,
) {
	target := a.icons.target
	if target == nil {
		return
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		ui.Button(c, locales.ItemIconUseFor(target.name)).
			Disabled(a.state.Busy || preview.err != nil || !target.current(a.state)).
			OnClick(func() {
				if !target.current(a.state) {
					a.state.SetErr(rom.ErrStale)
					return
				}
				if err := a.state.Edit(func() error {
					return a.state.Session.Records.Set(
						records.Items,
						[]int{target.item},
						"icon_id",
						int64(id),
					)
				}); err != nil {
					a.state.SetErr(err)
					return
				}
				a.Open(records.Items, target.item)
			})
		ui.Button(c, locales.ItemIconReturn()).
			OnClick(func() { a.Open(records.Items, target.item) })
	})
	if !target.current(a.state) {
		ui.Text(c, components.ErrorText(rom.ErrStale)).
			TextColor(c.Theme().TextMuted)
	}
}

func (a *Page) iconUses(c *ui.Context, id int) {
	state := &a.icons
	ui.Text(c, locales.ItemIconUses()).Bold()
	if state.usesErr != nil {
		ui.Text(c, components.ErrorText(state.usesErr)).
			TextColor(c.Theme().Danger)
		return
	}
	uses := state.uses[id]
	if len(uses) == 0 {
		ui.Text(c, locales.ItemIconUnused()).TextColor(c.Theme().TextMuted)
		return
	}
	state.usesList.Selected = &state.useSelected
	state.usesList.Key = func(index int) any { return uses[index].ID }
	state.usesList.Label = func(index int) string { return fmt.Sprintf("%02X · %s", uses[index].ID, uses[index].Name) }
	ui.List(c.Key("uses"), &state.usesList, len(uses), func(index int) {
		row := uses[index]
		ui.Row(c).
			WidthPercent(100).
			Gap(10).
			Padding(4, 6).
			Cursor(ui.CursorPointer).
			OnClick(func() { a.Open(records.Items, row.ID) }).
			Children(func() {
				ui.Textf(c, "%02X", row.ID).Width(36).FontFeatures("tnum")
				ui.Text(c, row.Name).SingleLine().Grow(1).MinWidth(0)
			})
	}).Grow(1).MinHeight(0).Label(fmt.Sprintf("%s %02X", locales.ItemIconUses(), id)).OnSubmit(func() {
		if state.useSelected >= 0 && state.useSelected < len(uses) {
			a.Open(records.Items, uses[state.useSelected].ID)
		}
	})
}

func (a *Page) cancelIconEdit() {
	state := &a.icons
	if !state.assetBound {
		return
	}
	state.generation++
	state.assetBound = false
	state.imageEditor = components.AssetEditor{}
}

func (a *Page) iconAssetActions() components.AssetActions {
	state := &a.icons
	session, icon, source := a.state.Session, state.assetIcon, state.assetSource
	actions := components.AssetActions{Export: a.assets.Export}
	if a.assets.Import != nil {
		actions.Import = func(done func([]byte, error)) {
			state.generation++
			generation := state.generation
			a.assets.Import(func(content []byte, err error) {
				rows := state.rows()
				if a.state.Session != session || a.itemMode != itemIconTable ||
					!state.assetBound ||
					state.generation != generation ||
					state.assetIcon != icon ||
					!source.Current(a.state.Session.ItemIcons) ||
					state.selected < 0 ||
					state.selected >= len(rows) ||
					rows[state.selected] != icon {
					return
				}
				done(content, err)
			})
		}
	}
	return actions
}

func (a *Page) openItemIcon(icon, item int) {
	if a.state.Busy {
		return
	}
	table, row, _, err := a.state.Session.Records.Resolve(
		records.Items,
		item,
		"icon_id",
	)
	if err != nil {
		a.state.SetErr(err)
		return
	}
	value, err := a.state.Session.Records.Value(records.Items, item, "icon_id")
	if err != nil {
		a.state.SetErr(err)
		return
	}
	a.cancelIconEdit()
	a.panes[a.tab].layout.move = nil
	a.itemMode = itemIconTable
	a.form.textChoice = nil
	a.icons.query = ""
	a.icons.selected = icon
	a.icons.list.ScrollTo(icon, ui.Center)
	a.icons.target = &itemIconTarget{
		session:  a.state.Session,
		revision: a.state.Session.Document.State().Revision,
		layout:   table.Layout,
		item:     item,
		original: value,
		name:     row.Name,
	}
}
