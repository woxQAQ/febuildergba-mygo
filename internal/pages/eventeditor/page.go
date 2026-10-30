// Package eventeditor owns chapter-script navigation and instruction drafts.
package eventeditor

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type (
	numberDraft struct {
		text    string
		value   int64
		invalid bool
	}
	referenceDraft struct {
		choice string
		value  int64
	}
	operandKey struct {
		instruction uint64
		field       string
	}
	draftState struct {
		data       *event.ScriptDraft
		selected   int
		list       ui.ListState
		inputs     map[operandKey]*numberDraft
		preview    *event.ScriptEdit
		command    string
		references map[operandKey]*referenceDraft
		movements  map[operandKey]*movementDraft
	}
)

func (draft *draftState) structuralChanged(selected int) {
	draft.selected = selected
	draft.preview = nil
	draft.list = ui.ListState{}
	draft.list.ScrollIntoView(selected)
	live := make(map[uint64]bool)
	for _, row := range draft.data.Instructions() {
		live[row.ID] = true
	}
	for key := range draft.inputs {
		if !live[key.instruction] {
			delete(draft.inputs, key)
		}
	}
	for key := range draft.movements {
		if !live[key.instruction] {
			delete(draft.movements, key)
		}
	}
	for key := range draft.references {
		if !live[key.instruction] {
			delete(draft.references, key)
		}
	}
}

func (draft *draftState) invalid() bool {
	for _, movement := range draft.movements {
		if movement.invalid {
			return true
		}
	}
	for _, input := range draft.inputs {
		if input.invalid {
			return true
		}
	}
	return false
}

type Page struct {
	state          *editor.State
	openHex        func(int, int)
	openText       func(uint16)
	openDeployment func(int, int)
	openMapChange  func(int, int)
	chapter        int
	chapterChoice  string
	site           int
	query          string
	selected       int
	sidebar        float32
	list           ui.ListState
	entries, rows  []event.ScriptEntry
	err            error
	drafts         map[int]*draftState
	showRaw        bool
}

func New(
	state *editor.State,
	openHex func(int, int),
	openText func(uint16),
	openDeployment func(int, int),
) *Page {
	return &Page{
		state:          state,
		openHex:        openHex,
		openText:       openText,
		openDeployment: openDeployment,
		sidebar:        330,
		drafts:         make(map[int]*draftState),
	}
}

func (page *Page) SetMapChangeOpener(
	open func(int, int),
) {
	page.openMapChange = open
}

func (page *Page) Reset() {
	mapChange := page.openMapChange
	state, hex, text, units, sidebar := page.state, page.openHex, page.openText, page.openDeployment, page.sidebar
	*page = *New(state, hex, text, units)
	page.sidebar = sidebar
	page.openMapChange = mapChange
}

func (page *Page) refresh() {
	if page.selected >= 0 && page.selected < len(page.rows) {
		page.site = page.rows[page.selected].Pointer
	}
	page.entries, page.err = page.state.Session.Events.ScriptEntries(
		page.chapter,
	)
	page.rows = nil
	query := strings.TrimPrefix(
		strings.ToLower(strings.TrimSpace(page.query)),
		"0x",
	)
	for _, entry := range page.entries {
		caption := fmt.Sprintf(
			"%s %08X %08X",
			locales.Label(entry.Source),
			entry.Pointer,
			entry.Address,
		)
		if query == "" || strings.Contains(strings.ToLower(caption), query) {
			page.rows = append(page.rows, entry)
		}
	}
	page.selected = -1
	for index, entry := range page.rows {
		if entry.Pointer == page.site {
			page.selected = index
			break
		}
	}
	if page.selected < 0 && len(page.rows) > 0 {
		page.selected = 0
		page.site = page.rows[0].Pointer
	}
}

func (page *Page) Open(chapter, site int) {
	page.chapter, page.site, page.query = chapter, site, ""
	page.selected = -1
	page.chapterChoice = ""
	page.refresh()
	if page.selected >= 0 {
		page.list.ScrollTo(page.selected, ui.Center)
	}
}

func (page *Page) View(c *ui.Context) {
	page.refresh()
	ui.Split(c, &page.sidebar, func() { page.navigation(c) }, func() { page.editor(c) }).
		Grow(1)
}

func (page *Page) navigation(c *ui.Context) {
	state := page.state
	ui.Column(c.Key("script-navigation")).
		Fill().
		Gap(8).
		Padding(12).
		Children(func() {
			var chapters []int
			var captions []string
			if table := state.Session.Records.Table(
				records.Chapters,
			); table != nil &&
				table.Err == nil {
				for _, row := range table.Rows {
					chapters = append(chapters, row.ID)
					captions = append(
						captions,
						fmt.Sprintf("%02X · %s", row.ID, row.Name),
					)
				}
			}
			selected := slices.Index(chapters, page.chapter)
			if selected >= 0 &&
				!slices.Contains(captions, page.chapterChoice) {
				page.chapterChoice = captions[selected]
			}
			ui.Select(c.Key("script-chapter"), &page.chapterChoice, captions).
				MaxWidthPercent(100).
				Label(locales.ScriptChapter()).
				Disabled(state.Busy).
				OnChange(func() {
					if index := slices.Index(
						captions,
						page.chapterChoice,
					); index >= 0 {
						page.chapter = chapters[index]
						page.selected = -1
						page.site = 0
						page.query = ""
						page.list = ui.ListState{}
					}
				})
			components.TextDraft(
				state,
				ui.TextInput(c, &page.query).
					Label(locales.ScriptSearch()).
					Placeholder(locales.ScriptSearch()).
					Disabled(state.Busy).
					OnChange(func() { page.selected = -1; page.site = 0; page.list = ui.ListState{} }),
			)
			if page.err != nil {
				ui.Text(c, components.ErrorText(page.err)).
					TextColor(c.Theme().Danger)
				return
			}
			page.list.Selected = &page.selected
			page.list.Key = func(index int) any { return page.rows[index].Pointer }
			columns := []ui.TableColumn{
				{ID: "source", Title: locales.ScriptSource()},
				{
					ID:    "address",
					Title: locales.ScriptAddress(),
					Width: 104,
					Fixed: true,
				},
			}
			ui.Table(c.Key("script-entries"), &page.list, columns, len(page.rows), func(index, column int) {
				entry := page.rows[index]
				if column == 0 {
					ui.Text(c, locales.Label(entry.Source)).
						SingleLine().
						Tooltip(fmt.Sprintf("0x%08X", entry.Pointer))
				} else {
					ui.Textf(c, "%08X", entry.Address).FontFeatures("tnum")
				}
			}).
				Grow(1).
				Label(locales.ScriptEntries()).
				OnChange(func() {
					if page.selected >= 0 && page.selected < len(page.rows) {
						page.site = page.rows[page.selected].Pointer
					}
				})
			ui.Text(c, locales.RecordSummary(len(page.rows))).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		})
}

func (page *Page) draft(entry event.ScriptEntry) (*draftState, error) {
	draft := page.drafts[entry.Pointer]
	if draft != nil && !draft.data.Current(page.state.Session.Events) &&
		!draft.data.Modified() &&
		!draft.invalid() {
		delete(page.drafts, entry.Pointer)
		draft = nil
	}
	if draft == nil {
		data, err := page.state.Session.Events.NewScriptDraft(
			entry.Chapter,
			entry.Pointer,
		)
		if err != nil {
			return nil, err
		}
		draft = &draftState{
			data:       data,
			inputs:     make(map[operandKey]*numberDraft),
			references: make(map[operandKey]*referenceDraft),
		}
		page.drafts[entry.Pointer] = draft
	}
	return draft, nil
}

func (page *Page) editor(c *ui.Context) {
	if page.err != nil || page.selected < 0 || page.selected >= len(page.rows) {
		ui.Text(c, locales.NoResults()).Padding(24)
		return
	}
	entry := page.rows[page.selected]
	draft, err := page.draft(entry)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).
			Padding(24).
			TextColor(c.Theme().Danger)
		return
	}
	ui.Scroll(c.Key(entry.Pointer)).
		Grow(1).
		Padding(16).
		Gap(12).
		Children(func() {
			ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.ScriptEditor()).FontSize(20).Grow(1)
				ui.Checkbox(c, &page.showRaw, locales.Advanced())
			})
			ui.Textf(c, "%s · 0x%08X → 0x%08X", locales.Label(entry.Source), entry.Pointer, entry.Address).
				Selectable().
				FontFeatures("tnum").
				Tooltip(locales.ScriptScope())
			if !draft.data.Current(page.state.Session.Events) {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			if !draft.data.Complete() {
				ui.Text(c, locales.ScriptPartial()).TextColor(c.Theme().Warning)
			} else if !draft.data.Relocatable() {
				ui.Text(c, locales.ScriptOpaque()).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
			}
			rows := draft.data.Instructions()
			draft.selected = min(draft.selected, max(0, len(rows)-1))
			if len(rows) == 0 {
				ui.Button(c, locales.ScriptCreate()).
					Disabled(page.state.Busy || !draft.data.Current(page.state.Session.Events) || !draft.data.Relocatable()).
					OnClick(func() {
						page.state.Err = draft.data.Insert(
							0,
							event.ScriptEnd,
						)
						if page.state.Err == nil {
							draft.structuralChanged(0)
						}
					})
			} else {
				page.instructionTable(c, draft, rows)
				page.commands(c, draft)
				page.operands(c, draft, rows[draft.selected])
			}
			page.actions(c, draft)
		})
}
