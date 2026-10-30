// Package mapview displays a chapter's base map and inspects its cells.
package mapview

import (
	"fmt"
	"image"
	"math"
	"slices"

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
	// Tools builds page-owned controls alongside zoom and grid controls.
	Tools  func(*ui.Context)
	Prompt string
	// Viewport opts into a canvas fitted to this size in logical pixels.
	// A zero size retains the compact map preview.
	Viewport image.Point
}

type View struct {
	loaded     bool
	resource   maps.Preview
	bitmap     *ui.Bitmap
	err        error
	zoom       int
	zoomChoice string
	fit        bool
	viewport   image.Point
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
	scale := v.scale()
	centerX, centerY := float32(120), float32(120)
	if v.viewport.X > 0 {
		centerX, centerY = float32(v.viewport.X)/2, float32(v.viewport.Y)/2
	}
	v.scroll.X = max(0, (float32(point.X)+0.5)*16*scale-centerX)
	v.scroll.Y = max(0, (float32(point.Y)+0.5)*16*scale-centerY)
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
	if overlay.Viewport.X > 0 && v.viewport.X == 0 {
		v.fit = true
		v.zoomChoice = locales.MapFit()
	}
	v.viewport = overlay.Viewport
	v.err = err
	v.render(c, overlay)
}

func (v *View) scale() float32 {
	if v.fit && v.viewport.X > 0 && v.resource.Width > 0 &&
		v.resource.Height > 0 {
		scale := min(
			float64(v.viewport.X-2)/float64(v.resource.Width*16),
			float64(v.viewport.Y-2)/float64(v.resource.Height*16),
		)
		if scale >= 1 {
			return float32(math.Floor(scale))
		}
		return float32(1 / math.Ceil(1/max(scale, 0.01)))
	}
	return float32(uint32(1)<<uint(v.zoom)) / 2
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
		if v.viewport.X == 0 {
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
		}
		if v.err == nil || overlay.Tools != nil {
			ui.Row(c.Key("map-tools")).
				Wrap().
				Gap(12).
				AlignItems(ui.Center).
				Children(func() {
					if overlay.Tools != nil {
						overlay.Tools(c)
					}
					if v.err != nil {
						return
					}
					before, beforeScale := v.zoom, v.scale()
					if v.viewport.X > 0 {
						choices := []string{
							locales.MapFit(),
							"50%",
							"100%",
							"200%",
							"400%",
						}
						if !slices.Contains(choices, v.zoomChoice) {
							v.zoomChoice = choices[0]
						}
						ui.Select(c.Key("canvas-zoom"), &v.zoomChoice, choices).
							Width(132).Label(locales.MapZoom()).
							OnChange(func() {
								for i, choice := range choices {
									if choice != v.zoomChoice {
										continue
									}
									v.fit = i == 0
									if !v.fit {
										v.zoom = i - 1
									}
								}
								v.scroll = ui.ScrollState{}
							})
					} else {
						ui.Tabs(c.Key("zoom"), &v.zoom, "50%", "100%", "200%", "400%").
							Label(locales.MapZoom())
						if before != v.zoom {
							ratio := v.scale() / beforeScale
							v.scroll.X *= ratio
							v.scroll.Y *= ratio
						}
					}
					ui.Checkbox(c.Key("grid"), &v.grid, locales.MapGrid())
					if ui.Button(c.Key("reset-view"), locales.MapResetView()).
						Clicked() {
						v.zoom, v.fit = 1, v.viewport.X > 0
						v.zoomChoice = locales.MapFit()
						v.scroll.X, v.scroll.Y = 0, 0
					}
					if v.viewport.X > 0 {
						ui.Textf(c, "%d × %d", v.resource.Width, v.resource.Height).
							FontSize(12).
							TextColor(c.Theme().TextMuted)
					}
				})
		}
		if overlay.Prompt != "" {
			ui.Text(c.Key("map-prompt"), overlay.Prompt).FontSize(13)
		}
		if v.err != nil {
			ui.Text(c, components.ErrorText(v.err)).
				TextColor(c.Theme().TextMuted)
			return
		}
		scale := v.scale()
		width, height := float32(
			v.resource.Width*16,
		)*scale, float32(
			v.resource.Height*16,
		)*scale
		viewWidth, viewHeight := width+2, min(height+2, 360)
		if v.viewport.X > 0 {
			viewWidth, viewHeight = float32(v.viewport.X), float32(v.viewport.Y)
		}
		ui.ScrollBoth(c.Key("viewport")).
			Width(viewWidth).
			MaxWidthPercent(100).
			Height(viewHeight).
			Background(c.Theme().Surface).
			Shrink(0).
			TrackScroll(&v.scroll).
			BorderWidth(1).
			BorderColor(c.Theme().Border).
			Children(func() {
				ui.Column(c.Key("map-surface")).Width(max(width, viewWidth-2)).
					Height(max(height, viewHeight-2)).Center().Children(func() {
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
			})
		if v.selected && v.viewport.X > 0 {
			ui.Textf(c, "X %d · Y %d", v.selection.X, v.selection.Y).
				Label(locales.MapSelection()).
				FontFeatures("tnum").FontSize(12).TextColor(c.Theme().TextMuted)
		} else if v.selected {
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
