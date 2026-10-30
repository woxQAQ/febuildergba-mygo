package portraits

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func portraitValue(p *portrait.Portrait, name string) int64 {
	switch name {
	case mouthXField:
		return int64(p.Mouth.X)
	case mouthYField:
		return int64(p.Mouth.Y)
	case eyeXField:
		return int64(p.Eyes.X)
	case eyeYField:
		return int64(p.Eyes.Y)
	case blinkField:
		return int64(p.Blink)
	}
	return 0
}

func (page *Page) number(
	c *ui.Context,
	p *portrait.Portrait,
	v *viewState,
	name string,
) {
	value := portraitValue(p, name)
	draft := v.drafts[name]
	if draft == nil {
		draft = &numberDraft{
			text:  strconv.FormatInt(value, 10),
			value: value,
			epoch: page.state.HistoryEpoch,
		}
		v.drafts[name] = draft
	}
	if draft.value != value || draft.epoch != page.state.HistoryEpoch {
		draft.text = strconv.FormatInt(value, 10)
		draft.value = value
		draft.epoch = page.state.HistoryEpoch
		draft.invalid = false
	}
	commit := func(next int64) {
		if err := page.state.Session.Portraits.Set(
			p,
			map[string]int64{name: next},
		); err != nil {
			page.state.Err = err
			return
		}
		draft.value = next
		draft.invalid = false
		page.state.Err = nil
		if name == mouthXField || name == mouthYField {
			v.part = 0
		} else if name != blinkField {
			v.part = 1
		}
	}
	input := ui.TextInput(c.Key(name), &draft.text).
		Width(64).
		Height(26).
		Padding(3, 6).
		FontSize(13).
		FontFeatures("tnum").
		TextAlign(ui.End).
		Label(locales.Label(name)).
		Disabled(page.state.Busy).
		OnTextKey(func(k ui.TextKey) bool {
			switch {
			case k.Key == ui.KeyZ && k.Mods == ui.Cmd:
				page.state.Undo()
			case k.Key == ui.KeyZ && k.Mods == ui.Cmd|ui.Shift:
				page.state.Redo()
			case k.Key == ui.KeyUp && k.Mods == 0:
				if draft.value < 255 {
					commit(draft.value + 1)
				}
			case k.Key == ui.KeyDown && k.Mods == 0:
				if draft.value > 0 {
					commit(draft.value - 1)
				}
			default:
				return false
			}
			fresh, err := page.state.Session.Portraits.Read(p.Entry.ID)
			if err == nil {
				current := portraitValue(fresh, name)
				draft.value = current
				draft.invalid = false
				draft.epoch = page.state.HistoryEpoch
				k.SetText(strconv.FormatInt(current, 10))
			}
			return true
		}).
		OnChange(func() {
			parsed, err := input.ParseInteger(draft.text)
			if err != nil || parsed < 0 || parsed > 255 {
				draft.invalid = true
				return
			}
			commit(parsed)
		})
	if input.Focused() {
		if name == mouthXField || name == mouthYField {
			v.part = 0
		} else if name != blinkField {
			v.part = 1
		}
	}
	if draft.invalid {
		message := locales.AllowedRange() + ": 0…255"
		input.Error(message).Tooltip(message)
	}
}

func (page *Page) positionInputs(
	c *ui.Context,
	p *portrait.Portrait,
	v *viewState,
) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, locales.Label("portrait_position")).Bold().FontSize(13)
		parts := []struct{ label, name string }{
			{mouthPart, mouthPart},
		}
		if p.HasEyes {
			parts = append(
				parts,
				struct{ label, name string }{
					"portrait_eyes",
					eyePart,
				},
			)
		}
		for _, part := range parts {
			ui.Row(c.Key(part.name)).
				Gap(6).
				AlignItems(ui.Center).
				Children(func() {
					ui.Text(c, locales.Label(part.label)).
						Width(32).
						FontSize(13)
					ui.Text(c, "X").FontSize(12)
					page.number(c, p, v, part.name+"_x")
					ui.Text(c, "Y").FontSize(12)
					page.number(c, p, v, part.name+"_y")
				})
		}
		ui.Text(c, locales.Label("portrait_grid_help")).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	})
}

func (page *Page) blinkInput(
	c *ui.Context,
	p *portrait.Portrait,
	v *viewState,
) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, locales.Label(blinkField)).Bold().FontSize(13)
		label := fmt.Sprintf("%d", p.Blink)
		switch p.Blink {
		case 1:
			label = locales.Label("portrait_blink_normal")
		case 6:
			label = locales.Label("portrait_blink_closed")
		}
		if v.blinkValue != p.Blink || v.blinkLabel != label {
			v.blink, v.blinkLabel, v.blinkValue = label, label, p.Blink
		}
		options := []string{
			locales.Label("portrait_blink_normal"),
			locales.Label("portrait_blink_closed"),
		}
		if p.Blink != 1 && p.Blink != 6 {
			options = append(options, label)
		}
		ui.Select(c.Key("blink"), &v.blink, options).
			Label(locales.PortraitTable() + " / " + locales.Label(blinkField)).
			Disabled(page.state.Busy).
			OnChange(func() {
				value := int64(p.Blink)
				if v.blink == locales.Label("portrait_blink_normal") {
					value = 1
				} else if v.blink == locales.Label("portrait_blink_closed") {
					value = 6
				}
				page.state.Err = page.state.Session.Portraits.Set(
					p,
					map[string]int64{blinkField: value},
				)
			})
	})
}

func (page *Page) resourceAddress(
	c *ui.Context,
	resource records.RecordResource,
) {
	address := fmt.Sprintf("0x%08X", resource.Address)
	services := c.Services()
	copyAddress := func() { services.WriteClipboard(address) }
	control := ui.ButtonBase(c.Key("address")).
		Height(24).Padding(2, 0).Radius(2).
		Cursor(ui.CursorPointer).Role(ui.RoleLink).
		Label(locales.Label(resource.Label)+" / "+locales.EditBytes()).
		Tooltip(locales.EditBytes()+"; "+locales.Label("portrait_copy_address")).
		Disabled(page.state.Busy).
		OnClick(func() { page.openHex(resource.Address, resource.Length) }).
		OnShortcut(ui.Cmd, ui.KeyC, copyAddress).
		ContextMenu(func(menu *ui.Menu) {
			if menu.Item(locales.Label("portrait_copy_address")).Chosen() {
				copyAddress()
			}
		})
	if control.Hovered() {
		control.Background(c.Theme().SurfaceHover)
	}
	control.Children(
		func() { ui.Text(c, address).Font("monospace").FontSize(12) },
	)
}

func (page *Page) inspector(c *ui.Context, p *portrait.Portrait, v *viewState) {
	if len(p.Fields) > 0 {
		page.positionInputs(c, p, v)
		if p.HasEyes {
			page.blinkInput(c, p, v)
		}
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, locales.Label("portrait_parts")).Bold().FontSize(13)
		for _, resource := range p.Resources {
			ui.Row(c.Key(resource.Label)).
				Wrap().
				Gap(6).
				AlignItems(ui.Center).
				Children(func() {
					ui.Text(c, locales.Label(resource.Label)).
						Width(64).
						FontSize(13)
					switch {
					case resource.Empty:
						ui.Text(c, locales.RecordNoResource()).
							FontSize(12).
							TextColor(c.Theme().TextMuted)
					case resource.Err != nil:
						ui.Text(c, components.ErrorText(resource.Err)).
							FontSize(12).
							TextColor(c.Theme().Danger)
					default:
						page.resourceAddress(c, resource)
					}
				})
		}
	})
	ui.Column(c).Gap(8).Children(func() {
		ui.Checkbox(c, &v.advanced, locales.Advanced())
		if v.advanced {
			ui.Textf(c, "0x%08X  %d B", p.Entry.Address, p.Entry.Stride).
				Font("monospace").
				FontSize(12).
				Selectable()
			ui.Button(c, locales.EditBytes()).
				Disabled(page.state.Busy).
				OnClick(func() { page.openHex(p.Entry.Address, p.Entry.Stride) })
			if p.HasEyes {
				page.number(c, p, v, blinkField)
			}
			for _, resource := range p.Resources {
				ui.Textf(c, "%s  0x%08X", locales.Label(resource.Label), resource.Pointer).
					FontSize(12).
					FontFeatures("tnum")
			}
		}
	})
}
