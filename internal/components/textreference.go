package components

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// TextCaption keeps selectors short; TextPreview retains the complete body.
func TextCaption(id uint32, plain string) string {
	body := []rune(strings.Join(strings.Fields(plain), " "))
	if len(body) > 48 {
		body = append(body[:48], '…')
	}
	if id == 0 {
		body = []rune(locales.QuoteNoText())
	} else if len(body) == 0 {
		body = []rune(locales.QuoteControlOnly())
	}
	return fmt.Sprintf("%04X · %s", id, string(body))
}

// TextReferenceState retains the selected caption across deferred input notices.
// The observed ID follows external edits, undo and record navigation.
type TextReferenceState struct {
	choice      string
	id          uint32
	initialized bool
}

// TextReference selects an existing text ID and reads it in place. The domain
// editor owns the index cache and invalidates it with document/layout changes.
// Typing filters choices; only choosing an entry changes the reference.
func TextReference(
	c *ui.Context,
	state *editor.State,
	reference *TextReferenceState,
	id uint32,
	label string,
	disabled bool,
	choose func(uint16),
	open func(uint16),
) ui.Element {
	var control ui.Element
	entries, err := state.Session.Texts.Entries()
	labels := []string{TextCaption(0, "")}
	values := map[string]uint16{labels[0]: 0}
	caption := fmt.Sprintf("%04X", id)
	if id == 0 {
		caption = labels[0]
	}
	for _, entry := range entries {
		if entry.Err != nil {
			continue
		}
		option := TextCaption(uint32(entry.ID), entry.Plain)
		labels = append(labels, option)
		values[option] = entry.ID
		if uint32(entry.ID) == id {
			caption = option
		}
	}
	if !reference.initialized || reference.id != id ||
		!slices.Contains(labels, reference.choice) {
		reference.choice = caption
		reference.id, reference.initialized = id, true
	}
	selected := &reference.choice
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(0).AlignItems(ui.Center).Children(func() {
			control = TextDraft(
				state,
				ui.Combobox(c.Key("text-id"), selected, labels).
					Width(240).Grow(1).MinWidth(0).MaxWidthPercent(100).
					Label(label).Tooltip(locales.QuoteReferenceHelp()).
					Disabled(state.Busy || disabled || err != nil).
					OnChange(func() {
						if value, ok := values[*selected]; ok {
							choose(value)
						}
					}),
			)
			if open != nil {
				ReferenceLink(
					c,
					locales.QuoteOpenText()+fmt.Sprintf(" %04X", id),
					state.Busy || id == 0 || id > 65535,
					func() { open(uint16(id)) },
				)
				// #nosec G115 -- Enabled only for bounded IDs.
			}
		})
		if err != nil {
			ui.Text(c, ErrorText(err)).
				FontSize(FontCaption).
				TextColor(c.Theme().Danger)
		}
		TextPreview(c, state.Session.Texts, id)
	})
	return control
}

// TextReferences browses texts discovered in an existing script. Choosing a
// reference changes only the reader, never the script or its operands.
func TextReferences(
	c *ui.Context,
	state *editor.State,
	selected *string,
	ids []uint16,
	open func(uint16),
) {
	if len(ids) == 0 {
		return
	}
	labels := make([]string, len(ids))
	for i, id := range ids {
		value, err := state.Session.Texts.Value(id)
		labels[i] = fmt.Sprintf("%04X", id)
		if err == nil {
			labels[i] = TextCaption(uint32(id), value.Plain)
		}
	}
	if !slices.Contains(labels, *selected) {
		*selected = labels[0]
	}
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(0).AlignItems(ui.Center).Children(func() {
			ui.Select(c.Key("text-reference"), selected, labels).
				Width(240).Grow(1).MinWidth(0).MaxWidthPercent(100).
				Label(locales.EventTextRefs()).Disabled(state.Busy)
			id := ids[max(0, slices.Index(labels, *selected))]
			ReferenceLink(c, locales.QuoteOpenText()+fmt.Sprintf(" %04X", id),
				state.Busy || id == 0 || open == nil, func() { open(id) },
			)
		})
		id := ids[max(0, slices.Index(labels, *selected))]
		TextPreview(c, state.Session.Texts, uint32(id))
	})
}

// TextUses renders the same confirmed usage scope in browsing and write reviews.
func TextUses(
	c *ui.Context,
	state *editor.State,
	uses []text.Use,
	complete, disabled bool,
	open func(text.Use),
) {
	ui.Text(c, locales.TextKnownUses(len(uses))).
		FontSize(FontCaption)
	if !complete {
		ui.Text(c, locales.TextUsesIncomplete()).
			FontSize(FontCaption).
			TextColor(c.Theme().Warning)
	}
	if len(uses) == 0 {
		return
	}
	ui.List(c.Key("uses"), nil, len(uses)).
		Label(locales.TextKnownUses(len(uses))).
		Height(min(140, c.Theme().Space(9)*float32(len(uses)))).
		ItemKey(func(index int) any { return uses[index] }).
		Rows(func(row ui.ListRow) {
			use := uses[row.Index]
			c := row.Context
			caption := fmt.Sprintf(
				"%s %02X · %s",
				locales.Label(use.Kind),
				use.ID,
				locales.Label(use.Field),
			)
			if use.Kind == event.TextUse {
				scope := fmt.Sprintf(
					"%s %02X",
					locales.Label("chapters"),
					use.ID,
				)
				if use.ID == event.WorldMapScripts {
					scope = locales.ScriptWorldmapDirectory()
				}
				caption = fmt.Sprintf(
					"%s · %s · %s · %08X",
					scope,
					locales.Label(use.Source),
					locales.Label(use.Field),
					use.Address,
				)
			}
			if open == nil {
				ui.Text(c.Key(use), caption).Selectable()
				return
			}
			tooltip := caption
			if use.Pointer != 0 {
				tooltip += fmt.Sprintf(
					" · %s %08X",
					locales.Address(),
					use.Pointer,
				)
			}
			ui.Button(c.Key(use), caption).
				FillWidth().MinWidth(0).
				Tooltip(tooltip).
				Disabled(state.Busy || disabled).
				OnClick(func() { open(use) })
		})
}

// TextPreview uses the decoded reading view without truncation. Long content
// scrolls inside a bounded surface, including in a surrounding page scroller.
func TextPreview(c *ui.Context, texts *text.Editor, id uint32) {
	if id > 65535 {
		ui.Text(c, locales.GameInvalidValue()).TextColor(c.Theme().Danger)
		return
	}
	value, err := texts.Value(uint16(id))
	if err != nil {
		ui.Text(c, ErrorText(err)).
			FontSize(FontCaption).
			TextColor(c.Theme().Danger)
		return
	}
	body := value.Plain
	if id == 0 {
		body = locales.QuoteNoText()
	} else if strings.TrimSpace(body) == "" {
		body = locales.QuoteControlOnly()
	}
	height := min(
		float32(144),
		max(float32(52), float32(strings.Count(body, "\n")+1)*18+16),
	)
	ui.Scroll(c.Key(id)).
		Height(height).Padding(8).Background(c.Theme().Surface).
		Border(1, c.Theme().Border).Radius(c.Theme().Radius).
		Label(locales.QuoteReadingPreview()).Children(func() {
		ui.Text(c, body).Selectable()
	})
}
