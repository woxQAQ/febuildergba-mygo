package eventeditor

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type portraitPreview struct {
	source *portrait.Portrait
	bitmap *ui.Bitmap
}

func (page *Page) portraitOptions(field event.ScriptField) ([]int64, []string) {
	entries, _ := page.state.Session.Portraits.Entries()
	var values []int64
	var captions []string
	for _, entry := range entries {
		if entry.Empty || entry.ClassCard || entry.Err != nil ||
			int64(entry.ID) < field.Min ||
			int64(entry.ID) > field.Max {
			continue
		}
		caption := fmt.Sprintf("%04X", entry.ID)
		if entry.Name != "" {
			caption += " · " + entry.Name
		}
		values = append(values, int64(entry.ID))
		captions = append(captions, caption)
	}
	return values, captions
}

func (page *Page) portraitOperand(
	c *ui.Context,
	draft *draftState,
	field event.ScriptField,
) {
	value, _ := draft.data.Value(draft.selected, field.ID)
	preview := &page.portrait
	source := page.state.Session.Portraits
	var err error
	if preview.source == nil || preview.source.Entry.ID != int(value) ||
		!preview.source.Current(source) {
		preview.source, err = source.Read(int(value))
		preview.bitmap = nil
		if preview.source != nil && preview.source.Main != nil {
			preview.bitmap = ui.NewBitmap(preview.source.Main)
			preview.bitmap.Pixelated()
		}
	}
	if preview.source != nil {
		err = preview.source.MainErr
	}
	ui.Row(c.Key("portrait-preview")).
		Gap(12).
		AlignItems(ui.Start).
		Children(func() {
			if preview.bitmap != nil {
				bounds := preview.source.Main.Bounds()
				ui.Image(c, preview.bitmap).
					Size(float32(bounds.Dx()), float32(bounds.Dy())).
					Shrink(0).
					Label(locales.Label("script_portrait_preview"))
			}
			ui.Column(c).Gap(4).Grow(1).AlignItems(ui.Start).Children(func() {
				ui.Text(c, locales.Label("script_portrait_preview")).
					FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted)
				if err != nil {
					ui.Text(c, components.ErrorText(err)).
						FontSize(components.FontCaption)
				}
				ui.Button(c, locales.Label("script_open_portrait")).
					Disabled(page.state.Busy || page.openPortrait == nil || preview.source == nil || !draft.data.Current(page.state.Session.Events)).
					OnClick(func() { page.openPortrait(int(value)) })
			})
		})
}
