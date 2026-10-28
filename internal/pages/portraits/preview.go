package portraits

import (
	"fmt"
	"image"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type positionDrag struct {
	portrait *game.Portrait
	part     int
	start    image.Point
	dx, dy   float32
	rejected bool
}

func (page *Page) positions(p *game.Portrait) (image.Point, image.Point) {
	mouth, eyes := p.Mouth, p.Eyes
	if d := page.drag; d != nil && !d.rejected && d.portrait == p {
		scale := float32(page.viewFor(p.Entry.ID).zoom + 1)
		point := image.Pt(
			max(0, min(8, d.start.X+int(math.Round(float64(d.dx/(8*scale)))))),
			max(0, min(8, d.start.Y+int(math.Round(float64(d.dy/(8*scale)))))),
		)
		if d.part == 0 {
			mouth = point
		} else {
			eyes = point
		}
	}
	return mouth, eyes
}

func (page *Page) frames(p *game.Portrait, v *viewState) {
	mouth, eyes := page.positions(p)
	key := fmt.Sprint(p.Entry.ID, p.Revision(), v.eye, v.mouth, mouth, eyes)
	if key == page.frameKey {
		return
	}
	page.frameKey = key
	page.main, page.check = nil, nil
	frame, err := p.FrameAt(v.eye-1, v.mouth, mouth, eyes)
	page.frameErr = err
	if err != nil {
		frame, _ = p.FrameAt(v.eye-1, 0, mouth, eyes)
		if frame == nil {
			frame = p.Main
		}
	}
	if frame != nil {
		page.main = ui.NewBitmap(frame)
		page.main.Pixelated()
	}
	checkEye, checkMouth := 0, min(3, p.MouthFrames())
	if p.HasEyes {
		checkEye, checkMouth = 2, 2
	}
	check, _ := p.FrameAt(checkEye, checkMouth, mouth, eyes)
	if check != nil && p.MouthFrames() > 0 {
		page.check = ui.NewBitmap(check)
		page.check.Pixelated()
	}
}

func (page *Page) workspace(c *ui.Context, p *game.Portrait, v *viewState) {
	if page.drag != nil && page.drag.portrait != p {
		page.drag = nil
	}
	page.frames(p, v)
	ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Tabs(c.Key("section"), &v.section, locales.Label("portrait_effect"), locales.Label("portrait_sheet")).
			Label(locales.Label("portrait_view"))
		ui.Box(c).Grow(1)
		ui.Segmented(c.Key("zoom"), &v.zoom, "1×", "2×", "3×").
			Label(locales.PreviewZoom())
	})
	if v.section == 1 {
		page.sheetView(c, p, v)
	} else {
		page.effectView(c, p, v)
	}
	if page.frameErr != nil && page.main != nil {
		page.imageError(c, page.frameErr)
	}
	if len(p.Palette) > 0 {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, locales.Label("portrait_palette")).Bold().FontSize(13)
			ui.Row(c).Wrap().Gap(4).Children(func() {
				for index, value := range p.Palette {
					box := ui.Box(c.Key(index)).
						Size(24, 24).
						Background(ui.Hex(fmt.Sprintf("#%02X%02X%02X", value.R, value.G, value.B))).
						BorderWidth(1).
						BorderColor(c.Theme().Border).
						Tooltip(fmt.Sprintf("%d", index))
					if index == 0 {
						color := c.Theme().TextMuted
						box.Tooltip(locales.Label("portrait_transparent")).
							DrawOver(func(p *ui.Painter, r ui.Rect) { p.Line(r.X, r.Y+r.H, r.X+r.W, r.Y, 1, color) })
					}
				}
			})
		})
	}
	if page.sheet != nil {
		page.editor.RenderPreview(c, page.state)
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, locales.PortraitUses()).Bold().FontSize(13)
		if len(p.Entry.Uses) == 0 {
			ui.Text(c, locales.PortraitNoKnownUses()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, use := range p.Entry.Uses {
				ui.Button(c.Key(use), page.useLabel(use)).
					Tooltip(locales.Label(use.Field)).
					Disabled(page.state.Busy).
					OnClick(func() { page.openRecord(use.Kind, use.ID) })
			}
		})
	})
}

func (page *Page) useLabel(use game.AssetUse) string {
	label := fmt.Sprintf("%s %02X", locales.Label(string(use.Kind)), use.ID)
	table := page.state.Session.Table(use.Kind)
	if table == nil {
		return label
	}
	for _, row := range table.Rows {
		if row.ID == use.ID && row.Name != "" {
			return label + " " + row.Name
		}
	}
	return label
}

func (page *Page) sheetView(c *ui.Context, p *game.Portrait, v *viewState) {
	if page.sheet != nil {
		bounds := p.Sheet.Bounds()
		scale := float32(v.zoom + 1)
		ui.Image(c, page.sheet).
			Size(float32(bounds.Dx())*scale, float32(bounds.Dy())*scale).
			MaxWidthPercent(100).
			Background(c.Theme().Surface).
			Label(locales.Label("portrait_sheet"))
	} else {
		page.imageError(c, p.SheetErr)
	}
}

func (page *Page) effectView(c *ui.Context, p *game.Portrait, v *viewState) {
	ui.Row(c).Wrap().Gap(20).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, locales.Label("portrait_main")).Bold().FontSize(13)
			if page.main != nil {
				page.canvas(c, p, v)
			} else {
				page.imageError(c, page.frameErr)
			}
			if len(p.Fields) > 0 {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Checkbox(
						c,
						&v.markers,
						locales.Label("portrait_markers"),
					)
					if p.HasEyes {
						ui.Segmented(c.Key("part"), &v.part, locales.Label(mouthPart), locales.Label("portrait_eyes")).
							Label(locales.Label("portrait_part"))
					} else {
						v.part = 0
					}
				})
			}
		})
		if !p.Entry.ClassCard {
			page.secondaryPreview(c, p)
		}
	})
	page.expressionControls(c, p, v)
}

func (page *Page) expressionControls(
	c *ui.Context,
	p *game.Portrait,
	v *viewState,
) {
	if !p.HasEyes && p.MouthFrames() == 0 {
		return
	}
	ui.Column(c.Key("expression-controls")).Gap(6).Children(func() {
		if p.HasEyes {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.Label("portrait_eye_label")).
					Width(40).
					FontSize(13)
				ui.Segmented(c.Key("eye"), &v.eye, locales.Label("portrait_default"), locales.Label("portrait_open"), locales.Label("portrait_half"), locales.Label("portrait_closed")).
					Label(locales.Label("portrait_eye_frame"))
			})
		}
		if p.MouthFrames() > 0 {
			labels := []string{locales.Label("portrait_original")}
			for frame := 1; frame <= p.MouthFrames(); frame++ {
				labels = append(labels, fmt.Sprint(frame))
			}
			ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.Label("portrait_mouth_label")).
					Width(40).
					FontSize(13)
				ui.Segmented(c.Key("mouth"), &v.mouth, labels...).
					Label(locales.Label("portrait_mouth_frame"))
			})
		}
	})
}

func (page *Page) secondaryPreview(c *ui.Context, p *game.Portrait) {
	ui.Column(c).Gap(16).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, locales.Label("portrait_mini")).
				Bold().
				FontSize(13)
			if page.mini != nil {
				ui.Image(c, page.mini).
					Size(64, 64).
					Background(c.Theme().Surface).
					Label(locales.Label("portrait_mini"))
			} else {
				page.imageError(c, p.MiniErr)
			}
		})
		if page.check != nil {
			label := "portrait_expression"
			if p.HasEyes {
				label = "portrait_closed_mouth"
			}
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, locales.Label(label)).Bold().FontSize(13)
				ui.Image(c, page.check).
					Size(192, 160).
					Background(c.Theme().Surface).
					Label(locales.Label("portrait_expression"))
			})
		}
	})
}

func (page *Page) imageError(c *ui.Context, err error) {
	if err != nil {
		ui.Text(c, components.ErrorText(err)).
			FontSize(12).
			TextColor(c.Theme().Danger)
	} else {
		ui.Text(c, locales.PortraitEmpty()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
	}
}

func (page *Page) canvas(c *ui.Context, p *game.Portrait, v *viewState) {
	bounds := p.Main.Bounds()
	scale := float32(v.zoom + 1)
	mouth, eyes := page.positions(p)
	point := mouth
	if v.part == 1 {
		point = eyes
	}
	canvas := ui.Image(c, page.main).
		Size(float32(bounds.Dx())*scale, float32(bounds.Dy())*scale).
		Background(c.Theme().Surface).
		Label(locales.ImagePreview())
	if !v.markers || len(p.Fields) == 0 || page.state.Busy {
		return
	}
	canvas.Cursor(ui.CursorCrosshair)
	if dx, dy, dragging := canvas.Dragged(); dragging {
		if page.drag == nil {
			x, y, over := canvas.PointerPosition()
			inside := over && x-dx >= float32(point.X*8)*scale &&
				x-dx < float32(point.X*8+32)*scale &&
				y-dy >= float32(point.Y*8)*scale &&
				y-dy < float32(point.Y*8+16)*scale
			page.drag = &positionDrag{
				portrait: p,
				part:     v.part,
				start:    point,
				rejected: !inside,
			}
		}
		page.drag.dx += dx
		page.drag.dy += dy
	}
	if _, _, ended := canvas.DragEnded(); ended && page.drag != nil {
		d := page.drag
		if !d.rejected {
			mouthAt, eyesAt := page.positions(p)
			name, position := mouthPart, mouthAt
			if d.part == 1 {
				name, position = eyePart, eyesAt
			}
			page.state.Err = page.state.Session.SetPortrait(
				d.portrait,
				map[string]int64{
					name + "_x": int64(position.X),
					name + "_y": int64(position.Y),
				},
			)
		}
		page.drag = nil
	}
	accent := c.Theme().Accent
	canvas.DrawOver(func(painter *ui.Painter, r ui.Rect) {
		painter.Stroke(
			ui.Rect{
				X: r.X + float32(point.X*8)*scale,
				Y: r.Y + float32(point.Y*8)*scale,
				W: 32 * scale,
				H: 16 * scale,
			},
			accent,
			0,
			1,
		)
	})
}
