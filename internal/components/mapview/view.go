// Package mapview displays a chapter's base map and inspects its cells.
package mapview

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/locales"
)

type View struct {
	session          *game.Session
	loaded           bool
	revision         uint64
	chapter, address int
	resource         game.MapPreview
	bitmap           *ui.Bitmap
	err              error
	zoom             int
	grid             bool
	scroll           ui.ScrollState
	selection        image.Point
	selected         bool
	panning          bool
}

func (v *View) load(session *game.Session, row game.Row) {
	if v.session != session {
		*v = View{session: session, zoom: 1, grid: true}
	}
	revision := session.Document.State().Revision
	if v.loaded && v.revision == revision && v.chapter == row.ID &&
		v.address == row.Address {
		return
	}
	if !v.loaded || v.chapter != row.ID || v.address != row.Address {
		v.selected, v.panning = false, false
		v.scroll = ui.ScrollState{}
	}
	v.loaded, v.revision, v.chapter, v.address = true, revision, row.ID, row.Address
	v.resource, v.err = session.ChapterMap(row.ID)
	v.bitmap = nil
	if v.err == nil {
		v.bitmap = ui.NewBitmap(v.resource.Image).Pixelated()
	}
	if _, ok := v.resource.Cell(v.selection.X, v.selection.Y); !ok {
		v.selected = false
	}
}

func (v *View) Render(c *ui.Context, session *game.Session, row game.Row) {
	v.load(session, row)
	ui.Column(c.Key("chapter-map")).Gap(8).Children(func() {
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.MapPreview()).
				Bold().
				Tooltip(locales.MapPreviewScope())
			if v.err == nil {
				ui.Textf(c, "%d × %d", v.resource.Width, v.resource.Height).
					FontFeatures("tnum").
					TextColor(c.Theme().TextMuted)
			}
		})
		if v.err != nil {
			ui.Text(c, components.ErrorText(v.err)).
				TextColor(c.Theme().TextMuted)
			return
		}
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			before := v.zoom
			ui.Tabs(c.Key("zoom"), &v.zoom, "50%", "100%", "200%", "400%").
				Label(locales.MapZoom())
			if before != v.zoom {
				ratio := float32(
					uint32(1)<<uint(v.zoom),
				) / float32(
					uint32(1)<<uint(before),
				)
				v.scroll.X *= ratio
				v.scroll.Y *= ratio
			}
			ui.Checkbox(c, &v.grid, locales.MapGrid())
			if ui.Button(c, locales.MapResetView()).Clicked() {
				v.zoom = 1
				v.scroll.X, v.scroll.Y = 0, 0
			}
		})
		scale := float32(uint32(1)<<uint(v.zoom)) / 2
		width, height := float32(
			v.resource.Width*16,
		)*scale, float32(
			v.resource.Height*16,
		)*scale
		ui.ScrollBoth(c.Key("viewport")).
			Width(width + 2).
			MaxWidthPercent(100).
			Height(min(height+2, 360)).
			Shrink(0).
			TrackScroll(&v.scroll).
			BorderWidth(1).
			BorderColor(c.Theme().Border).
			Children(func() {
				canvas := ui.Image(c, v.bitmap).
					Size(width, height).
					Shrink(0).
					Label(locales.MapCanvas()).Cursor(ui.CursorCrosshair)
				if dx, dy, dragging := canvas.Dragged(); dragging {
					v.scroll.X -= dx
					v.scroll.Y -= dy
					if dx != 0 || dy != 0 {
						v.panning = true
						canvas.Cursor(ui.CursorGrabbing)
					}
				}
				if canvas.Clicked() && !v.panning {
					if x, y, over := canvas.PointerPosition(); over {
						v.selection = image.Pt(
							int(x/(16*scale)),
							int(y/(16*scale)),
						)
						_, v.selected = v.resource.Cell(
							v.selection.X,
							v.selection.Y,
						)
					}
				}
				if _, _, ended := canvas.DragEnded(); ended {
					v.panning = false
				}
				accent := c.Theme().Accent
				canvas.DrawOver(
					func(p *ui.Painter, r ui.Rect) { v.overlay(p, r, scale, accent) },
				)
			})
		if v.selected {
			cell, _ := v.resource.Cell(v.selection.X, v.selection.Y)
			ui.Textf(c, "X %d · Y %d · %s 0x%04X · %s 0x%02X", v.selection.X, v.selection.Y, locales.MapTile(), cell.Tile, locales.MapTerrain(), cell.Terrain).
				Label(locales.MapSelection()).
				FontFeatures("tnum").
				Selectable()
		} else {
			ui.Text(c, locales.MapChooseCell()).TextColor(c.Theme().TextMuted)
		}
	})
}

func (v *View) overlay(
	p *ui.Painter,
	r ui.Rect,
	scale float32,
	accent ui.Color,
) {
	cell := 16 * scale
	if v.grid {
		for x := 1; x < v.resource.Width; x++ {
			at := r.X + float32(x)*cell
			p.Line(at, r.Y, at, r.Y+r.H, 1, ui.RGBA(0, 0, 0, 0.4))
		}
		for y := 1; y < v.resource.Height; y++ {
			at := r.Y + float32(y)*cell
			p.Line(r.X, at, r.X+r.W, at, 1, ui.RGBA(0, 0, 0, 0.4))
		}
	}
	if v.selected {
		selected := ui.Rect{
			X: r.X + float32(v.selection.X)*cell,
			Y: r.Y + float32(v.selection.Y)*cell,
			W: cell,
			H: cell,
		}
		p.Stroke(selected, ui.RGB(255, 255, 255), 0, 3)
		p.Stroke(selected, accent, 0, 2)
	}
}
