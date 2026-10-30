// Package mapview displays a chapter's base map and inspects its cells.
package mapview

import (
	"fmt"
	"image"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type Marker struct {
	Position       image.Point
	Index, Faction int
	Selected       bool
}

type Region struct {
	Bounds   image.Rectangle
	Index    int
	Selected bool
}

type Overlay struct {
	Title, Help string
	Regions     []Region
	Markers     []Marker
	Choose      func(image.Point)
	Focus       *image.Point
	Path        *Path
	Drawing     *Drawing
}

type View struct {
	loaded     bool
	resource   maps.Preview
	bitmap     *ui.Bitmap
	err        error
	zoom       int
	grid       bool
	scroll     ui.ScrollState
	selection  image.Point
	selected   bool
	panning    bool
	stroke     []image.Point
	strokeTool Tool
}

// ResetInteraction clears selection, scrolling and incomplete gestures while
// retaining the user's zoom and grid preferences.
func (v *View) ResetInteraction() {
	v.selected, v.panning = false, false
	v.stroke = nil
	v.scroll = ui.ScrollState{}
}

func (v *View) Selection() (image.Point, bool) {
	_, valid := v.resource.Cell(v.selection.X, v.selection.Y)
	return v.selection, v.selected && valid && v.err == nil
}

func (v *View) focus(point image.Point) {
	if _, valid := v.resource.Cell(point.X, point.Y); !valid {
		return
	}
	v.selection, v.selected = point, true
	scale := float32(uint32(1)<<uint(v.zoom)) / 2
	v.scroll.X = max(0, float32(point.X)*16*scale-120)
	v.scroll.Y = max(0, float32(point.Y)*16*scale-120)
}

// Render consumes a domain preview and page-owned error. It never reads ROMs.
func (v *View) Render(
	c *ui.Context,
	preview maps.Preview,
	err error,
	overlay Overlay,
) {
	if !v.loaded {
		v.loaded, v.zoom, v.grid = true, 1, true
	}
	if v.resource.Image != preview.Image || err != nil {
		if v.resource.Width != preview.Width ||
			v.resource.Height != preview.Height {
			v.ResetInteraction()
		}
		v.resource = preview
		v.bitmap = nil
		if err == nil {
			v.bitmap = ui.NewBitmap(preview.Image).Pixelated()
		}
	}
	v.err = err
	v.render(c, overlay)
}

func (v *View) render(c *ui.Context, overlay Overlay) {
	if overlay.Focus != nil {
		v.focus(*overlay.Focus)
	}
	title, help := overlay.Title, overlay.Help
	if title == "" {
		title = locales.MapPreview()
	}
	if help == "" {
		help = locales.MapPreviewScope()
	}
	ui.Column(c.Key("chapter-map")).Gap(8).Children(func() {
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).
				Bold().
				Tooltip(help)
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
				if overlay.Drawing != nil && overlay.Drawing.Tool != Pan {
					v.drawInput(canvas, scale, overlay.Drawing)
				} else {
					v.inspectInput(canvas, scale, overlay.Choose)
				}
				accent := c.Theme().Accent
				canvas.DrawOver(
					func(p *ui.Painter, r ui.Rect) { v.overlay(p, r, scale, accent, overlay) },
				)
			})
		if v.selected {
			cell, _ := v.resource.Cell(v.selection.X, v.selection.Y)
			ui.Textf(c, "X %d · Y %d · %s 0x%04X · %s 0x%02X", v.selection.X, v.selection.Y, locales.MapTile(), cell.Tile, locales.MapTerrain(), cell.Terrain).
				Label(locales.MapSelection()).
				FontFeatures("tnum").
				Selectable()
		} else if overlay.Drawing == nil {
			ui.Text(c, locales.MapChooseCell()).TextColor(c.Theme().TextMuted)
		}
	})
}

func (v *View) overlay(
	p *ui.Painter,
	r ui.Rect,
	scale float32,
	accent ui.Color,
	overlay Overlay,
) {
	cell := 16 * scale
	v.regions(p, r, cell, accent, overlay.Regions)
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
	colors := []ui.Color{
		ui.RGB(45, 91, 166),
		ui.RGB(45, 122, 74),
		ui.RGB(172, 55, 47),
		ui.RGB(90, 90, 90),
	}
	for _, marker := range overlay.Markers {
		if _, valid := v.resource.Cell(
			marker.Position.X,
			marker.Position.Y,
		); !valid {
			continue
		}
		box := ui.Rect{
			X: r.X + float32(marker.Position.X)*cell + 1,
			Y: r.Y + float32(marker.Position.Y)*cell + 1,
			W: cell - 2,
			H: cell - 2,
		}
		p.Fill(box, colors[marker.Faction&3], 0)
		p.Stroke(box, ui.RGB(255, 255, 255), 0, 1)
		if cell >= 16 {
			p.Text(
				box.X+1,
				box.Y,
				fmt.Sprintf("%02X", marker.Index),
				min(cell*0.6, 16),
				ui.RGB(255, 255, 255),
			)
		}
		if marker.Selected {
			p.Stroke(box, ui.RGB(255, 255, 255), 0, 3)
			p.Stroke(box, accent, 0, 1)
		}
	}
	v.path(p, r, cell, accent, overlay.Path)
	v.drawStroke(p, r, cell, accent, overlay.Drawing)
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
