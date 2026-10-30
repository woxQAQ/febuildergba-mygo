// Package portraits owns shared portrait resources, part positioning and PNG edits.
package portraits

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

const (
	mouthPart   = "portrait_mouth"
	eyePart     = "portrait_eye"
	mouthXField = mouthPart + "_x"
	mouthYField = mouthPart + "_y"
	eyeXField   = eyePart + "_x"
	eyeYField   = eyePart + "_y"
	blinkField  = "portrait_blink"
)

type numberDraft struct {
	text    string
	value   int64
	epoch   uint64
	invalid bool
}
type viewState struct {
	section, eye, mouth, part, zoom int
	markers, advanced               bool
	drafts                          map[string]*numberDraft
	blink, blinkLabel               string
	blinkValue                      int
}
type Page struct {
	state                    *editor.State
	actions                  components.AssetActions
	openRecord               func(records.Kind, int)
	openHex                  func(int, int)
	sidebar                  components.Sidebar
	query                    string
	selected                 int
	list                     ui.ListState
	rows                     []portrait.Entry
	err                      error
	portrait                 *portrait.Portrait
	views                    map[int]*viewState
	thumbnails               map[int]*ui.Bitmap
	thumbRevision            uint64
	main, mini, sheet, check *ui.Bitmap
	frameKey                 string
	frameErr                 error
	editor                   components.AssetEditor
	drag                     *positionDrag
}

func New(
	state *editor.State,
	actions components.AssetActions,
	openRecord func(records.Kind, int),
	openHex func(int, int),
) *Page {
	return &Page{
		state:      state,
		actions:    actions,
		openRecord: openRecord,
		openHex:    openHex,
		selected:   -1,
		views:      make(map[int]*viewState),
		thumbnails: make(map[int]*ui.Bitmap),
	}
}

func (page *Page) Reset() {
	state, actions, record, hex, sidebar := page.state, page.actions, page.openRecord, page.openHex, page.sidebar
	*page = *New(state, actions, record, hex)
	page.sidebar = sidebar
}

func (page *Page) refresh() {
	selectedID := -1
	if page.selected >= 0 && page.selected < len(page.rows) {
		selectedID = page.rows[page.selected].ID
	}
	entries, err := page.state.Session.Portraits.Entries()
	page.err = err
	query := strings.ToLower(strings.TrimSpace(page.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 16)
	page.rows = nil
	for _, entry := range entries {
		if query == "" ||
			strings.Contains(strings.ToLower(entry.Name), query) ||
			idErr == nil && entry.ID == int(id) {
			page.rows = append(page.rows, entry)
		}
	}
	page.selected = 0
	if selectedID < 0 {
		for index, entry := range page.rows {
			if !entry.Empty {
				page.selected = index
				break
			}
		}
	}
	for index, entry := range page.rows {
		if entry.ID == selectedID {
			page.selected = index
			break
		}
	}
	revision := page.state.Session.Document.State().Revision
	if page.thumbRevision != revision {
		page.thumbnails = make(map[int]*ui.Bitmap)
		page.thumbRevision = revision
	}
}

func (page *Page) Open(id int) {
	page.query = ""
	page.selected = -1
	page.refresh()
	page.list = ui.ListState{}
	for index, entry := range page.rows {
		if entry.ID == id {
			page.selected = index
			page.list.ScrollTo(index, ui.Center)
			return
		}
	}
	page.query = fmt.Sprintf("0x%04X", id)
	page.selected = -1
	page.refresh()
}

func (page *Page) View(c *ui.Context) {
	page.refresh()
	page.sidebar.Render(
		c,
		244,
		func() { page.navigation(c) },
		func() { page.detail(c) },
	)
}

func (page *Page) navigation(c *ui.Context) {
	ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
		components.TextDraft(
			page.state,
			ui.TextInput(c, &page.query).
				Placeholder(locales.PortraitSearch()).
				Label(locales.PortraitSearch()).
				Disabled(page.state.Busy).
				OnChange(func() { page.selected = -1; page.list = ui.ListState{} }),
		)
		if page.err != nil {
			ui.Text(c, components.ErrorText(page.err)).
				TextColor(c.Theme().Danger)
			return
		}
		page.list.Selected = &page.selected
		page.list.Key = func(index int) any { return page.rows[index].ID }
		columns := []ui.TableColumn{
			{ID: "preview", Title: "", Width: 38, Fixed: true},
			{ID: "id", Title: "ID", Width: 48, Fixed: true},
			{ID: "name", Title: locales.PortraitUses()},
		}
		ui.Table(c, &page.list, columns, len(page.rows), func(row, col int) {
			entry := page.rows[row]
			switch col {
			case 0:
				thumb, loaded := page.thumbnails[entry.ID]
				if !loaded {
					preview, _ := page.state.Session.Portraits.Mini(entry.ID)
					if entry.ClassCard {
						preview, _ = page.state.Session.Images.Preview(
							graphics.Ref{
								Kind: graphics.Portrait,
								ID:   entry.ID,
							},
						)
					}
					if preview.Image != nil {
						thumb = ui.NewBitmap(preview.Image)
						thumb.Pixelated()
					}
					page.thumbnails[entry.ID] = thumb
				}
				if thumb != nil {
					ui.Image(c, thumb).Size(28, 28)
				} else {
					ui.Box(c).Size(28, 28)
				}
			case 1:
				ui.Textf(c, "%04X", entry.ID).FontFeatures("tnum").FontSize(12)
			case 2:
				name := entry.Name
				if name == "" {
					name = portraitKind(entry)
				}
				ui.Text(c, name).SingleLine().FontSize(13)
			}
		}).Grow(1).Label(locales.PortraitTable()).Disabled(page.state.Busy)
		ui.Text(c, locales.RecordSummary(len(page.rows))).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	})
}

func portraitKind(entry portrait.Entry) string {
	if entry.Empty {
		return locales.PortraitEmpty()
	}
	if entry.ClassCard {
		return locales.PortraitClassCard()
	}
	return locales.PortraitFace()
}

func (page *Page) viewFor(id int) *viewState {
	v := page.views[id]
	if v == nil {
		v = &viewState{
			markers:    true,
			part:       1,
			zoom:       2,
			drafts:     make(map[string]*numberDraft),
			blinkValue: -1,
		}
		page.views[id] = v
	}
	return v
}

func (page *Page) load(id int) {
	if page.portrait != nil && page.portrait.Entry.ID == id &&
		page.portrait.Current(page.state.Session.Portraits) {
		return
	}
	page.portrait, page.err = page.state.Session.Portraits.Read(id)
	page.frameKey = ""
	page.mini, page.sheet = nil, nil
	if page.portrait != nil {
		if page.portrait.Mini != nil {
			page.mini = ui.NewBitmap(page.portrait.Mini)
			page.mini.Pixelated()
		}
		if page.portrait.Sheet != nil {
			page.sheet = ui.NewBitmap(page.portrait.Sheet)
			page.sheet.Pixelated()
		}
	}
}

func (page *Page) detail(c *ui.Context) {
	if page.err != nil || page.selected < 0 || page.selected >= len(page.rows) {
		ui.Text(c, locales.NoResults()).Padding(24)
		return
	}
	entry := page.rows[page.selected]
	page.load(entry.ID)
	if page.err != nil || page.portrait == nil {
		ui.Text(c, components.ErrorText(page.err)).Padding(24)
		return
	}
	p, v := page.portrait, page.viewFor(entry.ID)
	width, _ := c.Size()
	ui.Column(c.Key(entry.ID)).Fill().Children(func() {
		ui.Row(c).
			Wrap().
			Padding(14, 16).
			Gap(10).
			AlignItems(ui.Center).
			Children(func() {
				ui.Textf(c, "%04X", entry.ID).FontSize(20).FontFeatures("tnum")
				name := entry.Name
				if name == "" {
					name = portraitKind(entry)
				}
				ui.Text(c, name).FontSize(20).Grow(1)
				if page.sheet != nil {
					page.editor.RenderButtons(
						c,
						page.state,
						graphics.Ref{Kind: graphics.Portrait, ID: entry.ID},
						page.actions,
					)
				}
			})
		if width-page.sidebar.Width() >= 820 {
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				ui.Scroll(c.Key("workspace")).
					Grow(1).
					Padding(0, 16, 16, 16).
					Gap(16).
					Children(func() { page.workspace(c, p, v) })
				ui.Scroll(c.Key("inspector")).
					Width(300).
					Padding(16).
					Gap(20).
					BorderWidth(0, 0, 0, 1).
					BorderColor(c.Theme().Border).
					Children(func() { page.inspector(c, p, v) })
			})
		} else {
			ui.Scroll(c.Key("narrow")).
				Grow(1).
				Padding(0, 16, 16, 16).
				Gap(20).
				Children(func() { page.workspace(c, p, v); page.inspector(c, p, v) })
		}
	})
}
