// Package hexeditor owns the byte editor and its revision-bound draft.
package hexeditor

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"

	"github.com/egoist/mygo/ui"
)

const rawViewLimit = 4096

type rawDraft struct {
	address, length            string
	original, text             string
	region                     *game.RawRegion
	generation                 uint64
	originalInput, editedInput ui.Handle
	err                        error
}

func formatHex(data []byte) string {
	var text strings.Builder
	for i, b := range data {
		if i > 0 {
			if i%16 == 0 {
				text.WriteByte('\n')
			} else {
				text.WriteByte(' ')
			}
		}
		fmt.Fprintf(&text, "%02X", b)
	}
	return text.String()
}

func decodeHex(text string, length int) ([]byte, error) {
	compact := strings.Join(strings.Fields(text), "")
	if len(compact) != length*2 {
		return nil, errors.New("invalid_hex")
	}
	data, err := hex.DecodeString(compact)
	if err != nil {
		return nil, errors.New("invalid_hex")
	}
	return data, nil
}

func (a *Page) rawRange() (int, int, error) {
	address, err := editor.ParseAddress(a.raw.address)
	if err != nil {
		return 0, 0, err
	}
	length, err := editor.ParseInteger(a.raw.length)
	if err != nil || length < 1 || length > rawViewLimit {
		return 0, 0, errors.New("raw_length")
	}
	return address, int(length), nil
}

func (a *Page) readRaw() {
	if a.state.Busy || a.state.Session == nil {
		return
	}
	address, length, err := a.rawRange()
	if err != nil {
		a.raw.err = err
		return
	}
	region, err := a.state.Session.ReadRegion(address, length)
	if err != nil {
		a.raw.err = err
		return
	}
	a.raw.region = region
	a.raw.generation++
	a.raw.original = formatHex(region.Bytes())
	a.raw.text = a.raw.original
	a.raw.err = nil
}

func (a *Page) applyRaw() {
	if a.state.Busy || a.state.Session == nil || a.raw.region == nil {
		return
	}
	address, length, err := a.rawRange()
	if err != nil {
		a.raw.err = err
		return
	}
	if address != a.raw.region.Address() || length != len(a.raw.region.Bytes()) {
		a.raw.err = errors.New("raw_read_first")
		return
	}
	data, err := decodeHex(a.raw.text, length)
	if err != nil {
		a.raw.err = err
		return
	}
	if err := a.state.Session.ApplyRegion(a.raw.region, data); err != nil {
		a.raw.err = err
		return
	}
	a.state.HistoryEpoch++
	a.readRaw()
}

func (a *Page) Open(address, length int) {
	a.raw.address = fmt.Sprintf("%08X", address)
	a.raw.length = strconv.Itoa(length)
	a.readRaw()
}

func (a *Page) View(c *ui.Context) {
	ui.Column(c.Key("hex-editor")).Grow(1).Padding(20).Gap(12).Children(func() {
		ui.Text(c, locales.HexEditor()).FontSize(20).Bold()
		ui.Text(c, locales.HexScope()).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(12).AlignItems(ui.End).Children(func() {
			ui.Field(c, locales.RomAddress(), func() {
				components.TextDraft(a.state, ui.TextInput(c, &a.raw.address).Width(190).Disabled(a.state.Busy))
			})
			ui.Field(c, locales.ByteCount(), func() {
				components.TextDraft(a.state, ui.TextInput(c, &a.raw.length).Width(100).Disabled(a.state.Busy))
			})
			ui.Button(c, locales.ReadBytes()).Disabled(a.state.Busy).OnClick(a.readRaw)
		})
		if a.raw.err != nil {
			ui.Text(c, locales.ErrorText(a.raw.err))
		}
		if a.raw.region == nil {
			ui.Text(c, locales.RawReadFirst())
			return
		}
		original := a.raw.region.Bytes()
		replacement, err := decodeHex(a.raw.text, len(original))
		changed := 0
		if err == nil {
			for i, b := range replacement {
				if b != original[i] {
					changed++
				}
			}
		}
		address, length, rangeErr := a.rawRange()
		moved := rangeErr != nil || address != a.raw.region.Address() || length != len(original)
		stale := a.raw.region.Revision() != a.state.Session.Document.State().Revision
		if stale {
			ui.Text(c, locales.RawStale())
		} else if moved {
			ui.Text(c, locales.RawReadFirst())
		}
		ui.Textf(c, "0x%08X – 0x%08X", a.raw.region.Address(), a.raw.region.Address()+len(original)-1).Font("monospace")
		ui.Row(c).Grow(1).Gap(16).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c.Key("original")).Grow(1).Gap(6).Children(func() {
				ui.Text(c, locales.OriginalBytes()).OnClick(func() { a.raw.originalInput.Focus(c) })
				ui.TextArea(c.Key(a.raw.generation), &a.raw.original).ReadOnly(true).Font("monospace").Grow(1).Label(locales.OriginalBytes()).Bind(&a.raw.originalInput)
			})
			ui.Column(c.Key("replacement")).Grow(1).Gap(6).Children(func() {
				ui.Text(c, locales.EditedBytes()).OnClick(func() { a.raw.editedInput.Focus(c) })
				components.TextDraft(a.state, ui.TextArea(c.Key(a.raw.generation), &a.raw.text).Font("monospace").Grow(1).Disabled(a.state.Busy).Label(locales.EditedBytes()).Bind(&a.raw.editedInput))
			})
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Button(c, locales.ApplyBytes()).Disabled(a.state.Busy || stale || moved || err != nil || changed == 0).OnClick(a.applyRaw)
			if err != nil {
				ui.Text(c, locales.InvalidHex())
			} else {
				ui.Textf(c, "%d %s", changed, locales.ChangedBytes())
			}
		})
	})
}

type Page struct {
	state *editor.State
	raw   rawDraft
}

func New(state *editor.State) *Page {
	a := &Page{state: state}
	a.Reset()
	return a
}

func (a *Page) Reset() { a.raw = rawDraft{address: "00000000", length: "64"} }
