package records

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type portraitChoice struct {
	open     bool
	kind     records.Kind
	record   int
	source   records.PortraitSelection
	entries  []portrait.Entry
	query    string
	selected int
	list     ui.ListState
	search   ui.Handle
	returnTo *ui.Handle
	previews map[int]*portraitChoicePreview
	err      error
}

type portraitChoicePreview struct {
	main, mini    *ui.Bitmap
	width, height float32
	err           error
}

func (a *form) profilePortrait(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	entry *profileState,
	preview *assetPreview,
) {
	source, err := a.state.Session.Records.Portrait(kind, row.ID)
	zero := source.Inherited || source.Ref.ID == 0
	help := locales.PortraitInteractionHelp()
	placeholder := locales.OpenReference(locales.PortraitTable())
	if source.Ref.ID == 0 {
		help = locales.PortraitEmptyInteractionHelp()
		placeholder = locales.PortraitSelect()
	}
	width, height := float32(96), float32(80)
	if preview != nil && preview.bitmap != nil {
		bounds := preview.resource.Image.Bounds()
		scale := min(float32(1), 96/float32(bounds.Dx()))
		width, height = float32(bounds.Dx())*scale, float32(bounds.Dy())*scale
	}
	ui.Column(c.Key("portrait")).Width(136).Shrink(0).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			control := ui.ButtonBase(c.Key("portrait-link")).
				Size(width+6, height+6).
				Margin(-3).
				Padding(2).
				Radius(3).
				Border(1, ui.RGBA(0, 0, 0, 0)).
				Bind(&entry.portrait).
				FocusRing(true).
				Label(locales.Label("portrait_id")).
				Tooltip(help).
				Disabled(a.state.Busy || err != nil).
				OnClick(func() {
					if source.Ref.ID != 0 && a.openPortrait != nil {
						a.openPortrait(source.Ref.ID)
					} else {
						a.beginPortraitChoice(kind, row.ID, &entry.portrait)
					}
				}).
				ContextMenu(func(menu *ui.Menu) {
					if menu.Item(locales.PortraitChoose()).
						Disabled(a.state.Busy || err != nil).
						Chosen() {
						a.beginPortraitChoice(kind, row.ID, &entry.portrait)
					}
					label := locales.PortraitInheritClass()
					if kind == records.Classes {
						label = locales.PortraitClear()
					}
					if menu.Item(label).Checked(zero).
						Disabled(a.state.Busy || err != nil || zero).Chosen() {
						a.state.SetErr(
							a.setProfilePortrait(kind, row.ID, source, 0),
						)
					}
				})
			if control.Hovered() {
				control.Cursor(ui.CursorPointer).
					Background(c.Theme().SurfaceHover).
					BorderColor(c.Theme().Accent)
			}
			if control.Pressed() {
				control.Background(c.Theme().SurfacePressed).
					BorderColor(c.Theme().AccentPressed)
			}
			control.Children(func() {
				if preview != nil && preview.bitmap != nil {
					ui.Image(c, preview.bitmap).Size(width, height)
				} else {
					ui.Box(c).
						Size(width, height).
						Background(c.Theme().Surface).
						Center().
						Children(func() {
							ui.Text(c, placeholder).
								FontSize(components.FontCaption).
								TextColor(c.Theme().TextMuted)
						})
				}
			})
			if preview != nil && preview.mini != nil {
				ui.Image(c, preview.mini).Size(32, 32).
					Label(locales.Label("portrait_mini"))
			}
		})
		if source.Inherited {
			ui.Textf(c, "%s %04X", locales.Label("portrait_inherit"), source.Ref.ID).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
	})
}

func (a *form) setProfilePortrait(
	kind records.Kind,
	id int,
	source records.PortraitSelection,
	value int,
) error {
	if a.state.Busy || !source.Current(a.state.Session.Records) {
		return rom.ErrStale
	}
	return a.state.Edit(func() error {
		return a.state.Session.Records.Set(
			kind,
			[]int{id},
			"portrait_id",
			int64(value),
		)
	})
}

func (a *form) beginPortraitChoice(
	kind records.Kind,
	id int,
	returnTo *ui.Handle,
) {
	if a.state.Busy {
		return
	}
	source, err := a.state.Session.Records.Portrait(kind, id)
	if err != nil {
		a.state.SetErr(err)
		return
	}
	entries, err := a.state.Session.Portraits.Entries()
	choice := &portraitChoice{
		open: true, kind: kind, record: id, source: source,
		selected: -1, returnTo: returnTo,
		previews: make(map[int]*portraitChoicePreview), err: err,
	}
	for _, entry := range entries {
		if entry.ID == 0 || entry.Empty {
			continue
		}
		if entry.ID == source.Ref.ID {
			choice.selected = len(choice.entries)
		}
		choice.entries = append(choice.entries, entry)
	}
	if choice.selected >= 0 {
		choice.list.ScrollTo(choice.selected, ui.Center)
	}
	a.portraitChoice = choice
	choice.search.Focus()
}

func (a *form) closePortraitChoice() {
	if choice := a.portraitChoice; choice != nil {
		choice.open = false
		choice.returnTo.Focus()
		a.portraitChoice = nil
	}
}

func (a *form) choosePortrait(c *ui.Context) {
	choice := a.portraitChoice
	if choice == nil {
		return
	}
	width, height := c.Size()
	modal := ui.Modal(c.Key(choice), &choice.open, func() {
		ui.Column(c).
			Width(min(float32(520), max(float32(0), width-120))).
			Gap(12).
			Children(func() {
				ui.Scroll(c).
					MaxHeight(max(float32(80), height-200)).
					Gap(10).
					Children(func() {
						ui.Text(c, locales.PortraitChoose()).
							FontSize(components.FontPanelTitle).
							Bold()
						if !choice.source.Current(a.state.Session.Records) {
							ui.Text(c, components.ErrorText(rom.ErrStale)).
								TextColor(c.Theme().Danger)
							return
						}
						components.TextDraft(
							a.state,
							ui.TextInput(c.Key("portrait-search"), &choice.query).
								Bind(&choice.search).
								Label(locales.PortraitSearch()).
								Placeholder(locales.PortraitSearch()).
								Disabled(a.state.Busy).
								OnChange(func() { choice.selected = -1; choice.list = ui.ListState{} }).
								OnSubmit(func() { a.commitPortraitChoice(choice) }),
						)
						rows := choice.rows()
						if len(rows) == 1 && choice.query != "" {
							choice.selected = 0
						}
						choice.list.Selected = &choice.selected
						choice.list.Key = func(index int) any { return rows[index].ID }
						choice.list.Label = func(index int) string {
							return fmt.Sprintf(
								"%04X · %s",
								rows[index].ID,
								portraitChoiceName(rows[index]),
							)
						}
						columns := []ui.TableColumn{
							{ID: "preview", Title: "", Width: 34, Fixed: true},
							{ID: "id", Title: "ID", Width: 48, Fixed: true},
							{ID: "name", Title: locales.PortraitUses()},
						}
						ui.Table(c.Key("portrait-choices"), &choice.list, columns, len(rows), func(row, col int) {
							candidate := rows[row]
							switch col {
							case 0:
								preview := choice.preview(a, candidate.ID)
								bitmap := preview.mini
								if bitmap == nil {
									bitmap = preview.main
								}
								if bitmap != nil {
									ui.Image(c, bitmap).Size(28, 28)
								}
							case 1:
								ui.Textf(c, "%04X", candidate.ID).
									FontFeatures("tnum")
							case 2:
								ui.Text(c, portraitChoiceName(candidate)).
									SingleLine()
							}
						}).
							Height(min(float32(240), max(float32(96), height/3))).
							Label(locales.PortraitTable()).
							Disabled(a.state.Busy).
							OnSubmit(func() { a.commitPortraitChoice(choice) })
						if len(rows) == 0 {
							ui.Text(c, locales.NoResults()).
								TextColor(c.Theme().TextMuted)
						}
						if choice.selected >= 0 && choice.selected < len(rows) {
							candidate := rows[choice.selected]
							preview := choice.preview(a, candidate.ID)
							ui.Row(c).
								Wrap().
								Gap(10).
								AlignItems(ui.Start).
								Children(func() {
									if preview.main != nil {
										ui.Image(c, preview.main).
											Size(preview.width, preview.height).
											Label(locales.ImagePreview())
									}
									ui.Textf(c, "%04X · %s", candidate.ID, portraitChoiceName(candidate)).
										MinWidth(0).
										MaxWidthPercent(100)
								})
							if preview.err != nil {
								ui.Text(c, components.ErrorText(preview.err)).
									TextColor(c.Theme().Danger)
							}
						}
						if choice.err != nil {
							ui.Text(c, components.ErrorText(choice.err)).
								TextColor(c.Theme().Danger)
						}
					})
				ui.Row(c).Wrap().Gap(8).Children(func() {
					ui.PrimaryButton(c, locales.PortraitChooseApply()).
						Disabled(a.state.Busy || !choice.canApply(a)).
						OnClick(func() { a.commitPortraitChoice(choice) })
					ui.Button(c, locales.Cancel()).
						OnClick(a.closePortraitChoice)
				})
			})
	})
	if modal.Dismissed() {
		a.closePortraitChoice()
	}
}

func portraitChoiceName(entry portrait.Entry) string {
	if entry.Name != "" {
		return entry.Name
	}
	if entry.ClassCard {
		return locales.PortraitClassCard()
	}
	return locales.PortraitFace()
}

func (choice *portraitChoice) rows() []portrait.Entry {
	query := strings.ToLower(strings.TrimSpace(choice.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 16)
	var rows []portrait.Entry
	for _, entry := range choice.entries {
		if query == "" ||
			strings.Contains(strings.ToLower(entry.Name), query) ||
			idErr == nil && entry.ID == int(id) {
			rows = append(rows, entry)
		}
	}
	return rows
}

func (choice *portraitChoice) preview(a *form, id int) *portraitChoicePreview {
	if preview := choice.previews[id]; preview != nil {
		return preview
	}
	resource, err := a.state.Session.Images.Preview(
		graphics.Ref{Kind: graphics.Portrait, ID: id},
	)
	preview := &portraitChoicePreview{err: err}
	if resource.Image != nil {
		bounds := resource.Image.Bounds()
		scale := min(float32(1), 96/float32(bounds.Dx()))
		preview.width, preview.height = float32(
			bounds.Dx(),
		)*scale, float32(
			bounds.Dy(),
		)*scale
		preview.main = ui.NewBitmap(resource.Image)
		preview.main.Pixelated()
	}
	mini, _ := a.state.Session.Portraits.Mini(id)
	if mini.Image != nil {
		preview.mini = ui.NewBitmap(mini.Image)
		preview.mini.Pixelated()
	}
	choice.previews[id] = preview
	return preview
}

func (choice *portraitChoice) canApply(a *form) bool {
	if !choice.open || !choice.source.Current(a.state.Session.Records) {
		return false
	}
	rows := choice.rows()
	if choice.selected < 0 || choice.selected >= len(rows) {
		return false
	}
	preview := choice.preview(a, rows[choice.selected].ID)
	return preview.main != nil && preview.err == nil
}

func (a *form) commitPortraitChoice(choice *portraitChoice) {
	if a.portraitChoice != choice || a.state.Busy || !choice.canApply(a) {
		return
	}
	candidate := choice.rows()[choice.selected]
	if err := a.setProfilePortrait(
		choice.kind,
		choice.record,
		choice.source,
		candidate.ID,
	); err != nil {
		choice.err = err
		return
	}
	a.closePortraitChoice()
}
