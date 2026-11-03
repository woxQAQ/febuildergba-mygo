package worldmaps

import (
	"fmt"
	"image"
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// The viewport keeps its identity while the selected node's fields change.
// Only the current, valid draft is overlaid on the committed node positions.
type nodeCanvas struct {
	draft      *worldmap.Draft
	background *image.RGBA
	bitmap     *ui.Bitmap
	zoom       string
	scroll     ui.ScrollState
	handle     ui.Handle
	placing    bool
}

// Map pixels have already been bounded and numeric drafts validated before
// positioning. Both setters accept these coordinates without further I/O.
func (s *nodeState) setPosition(point image.Point) {
	s.plan, s.err = nil, nil
	for _, coordinate := range []struct {
		field worldmap.Field
		value int
	}{{worldmap.X, point.X}, {worldmap.Y, point.Y}} {
		if err := s.draft.Set(coordinate.field, coordinate.value); err != nil {
			s.err = err
			return
		}
		inputState := s.inputs[coordinate.field]
		if inputState == nil {
			inputState = &valueInput{}
			s.inputs[coordinate.field] = inputState
		}
		inputState.text = strconv.Itoa(coordinate.value)
	}
}

func (p *Page) nodeMap(c *ui.Context, entry *nodeState) {
	if entry.draft == nil {
		return
	}
	e, v := p.state.Session.WorldMap, &p.nodeCanvas
	current := entry.draft.Current(e)
	mapBounds := image.Rect(0, 0, worldmap.MapWidth, worldmap.MapHeight)
	disabled := p.state.Busy || !current || entry.invalid()
	if v.draft != entry.draft || disabled {
		v.draft, v.placing = entry.draft, false
	}
	background, err := e.BackgroundImage(
		worldmap.BackgroundTone(p.backgroundTone),
	)
	if v.bitmap == nil || background != v.background {
		v.background = background
		if background == nil {
			background = image.NewRGBA(mapBounds)
		}
		v.bitmap = ui.NewBitmap(background).Pixelated()
	}
	nodes, _ := e.Nodes()
	selected := entry.draft.Node().ID
	if current {
		for i, node := range nodes {
			if node.ID == selected {
				nodes[i] = entry.draft.Node()
			}
		}
	}
	ui.Column(c.Key("node-map")).Gap(6).Children(func() {
		ui.Row(c.Key("tools")).
			Wrap().
			Gap(8).
			AlignItems(ui.Center).
			Children(func() {
				if v.zoom == "" {
					v.zoom = "100%"
				}
				ui.Select(c.Key("zoom"), &v.zoom, []string{"50%", "100%", "200%"}).
					Label(locales.MapZoom()).
					Width(115)
				ui.Tabs(c.Key("tone"), &p.backgroundTone, locales.WorldmapDaytime(), locales.WorldmapDarkened()).
					Label(locales.WorldmapBackgroundTone()).
					Disabled(p.state.Busy)
				ui.Checkbox(c.Key("place"), &v.placing, locales.WorldmapPlaceNode()).
					Disabled(disabled).
					OnClick(func() {
						if v.placing {
							v.handle.Focus(c)
						}
					})
			})
		ui.Text(c, locales.WorldmapNodeMapHelp()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		scale := float32(1)
		switch v.zoom {
		case "50%":
			scale = 0.5
		case "200%":
			scale = 2
		}
		width, height := float32(
			worldmap.MapWidth,
		)*scale, float32(
			worldmap.MapHeight,
		)*scale
		windowWidth, _ := c.Size()
		ui.ScrollBoth(c.Key("viewport")).
			Width(min(width+2, max(120, windowWidth-p.sidebar.Width()-36))).
			Height(min(height+2, 342)).Shrink(0).
			TrackScroll(&v.scroll).
			Background(c.Theme().Surface).
			BorderWidth(1).BorderColor(c.Theme().Border).
			Children(func() {
				canvas := ui.Image(c.Key("canvas"), v.bitmap).
					Size(width, height).Shrink(0).
					Bind(&v.handle).Focusable().
					Label(locales.WorldmapNodeCanvas()).Disabled(p.state.Busy)
				if v.placing {
					canvas.Cursor(ui.CursorCrosshair)
				}
				v.handle.OnShortcut(
					c,
					0,
					ui.KeyEscape,
					func() { v.placing = false },
				)
				if canvas.Clicked() && !p.state.Busy {
					x, y, over := canvas.PointerPosition()
					point := image.Pt(int(x/scale), int(y/scale))
					if over && point.In(mapBounds) {
						if v.placing && !disabled {
							entry.setPosition(point)
						} else {
							var hits []int
							for _, node := range nodes {
								if math.Hypot(
									float64(node.Value(worldmap.X)-point.X),
									float64(node.Value(worldmap.Y)-point.Y),
								)*float64(
									scale,
								) <= 9 {
									hits = append(hits, node.ID)
								}
							}
							if len(hits) > 0 {
								next := hits[0]
								for i, id := range hits {
									if id == selected {
										next = hits[(i+1)%len(hits)]
										break
									}
								}
								p.Open(next)
							}
						}
					}
				}
				accent := c.Theme().Accent
				canvas.DrawOver(func(painter *ui.Painter, bounds ui.Rect) {
					painter.Clip(bounds, 0, func() {
						// Paint the selection last so overlapping nodes remain visible.
						for _, active := range []bool{false, true} {
							for _, node := range nodes {
								if (node.ID == selected) != active {
									continue
								}
								x := bounds.X + float32(
									node.Value(worldmap.X),
								)*scale
								y := bounds.Y + float32(
									node.Value(worldmap.Y),
								)*scale
								box := ui.Rect{X: x - 7, Y: y - 7, W: 14, H: 14}
								fill := ui.RGB(40, 40, 40)
								if active {
									fill = accent
									painter.Stroke(
										ui.Rect{
											X: x - 10,
											Y: y - 10,
											W: 20,
											H: 20,
										},
										accent,
										10,
										2,
									)
								}
								painter.Fill(box, fill, 7)
								painter.Stroke(box, ui.RGB(255, 255, 255), 7, 1)
								painter.Fill(
									ui.Rect{X: x + 7, Y: y - 8, W: 22, H: 16},
									fill,
									2,
								)
								painter.Text(
									x+9,
									y-7,
									fmt.Sprintf("%02X", node.ID),
									12,
									ui.RGB(255, 255, 255),
								)
							}
						}
					})
				})
			})
		if current {
			node := entry.draft.Node()
			if !image.Pt(node.Value(worldmap.X), node.Value(worldmap.Y)).
				In(mapBounds) {
				ui.Text(c, locales.WorldmapNodeOffMap()).FontSize(12)
			}
		}
		if err != nil {
			ui.Text(c, components.ErrorText(err)).
				FontSize(12).
				TextColor(c.Theme().Danger)
		}
	})
}
