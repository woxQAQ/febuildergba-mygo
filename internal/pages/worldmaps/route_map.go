package worldmaps

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type routeCanvas struct {
	loaded           bool
	background, road *image.RGBA
	bitmap           *ui.Bitmap
	zoom             string
	scroll           ui.ScrollState
}

func (p *Page) routeMap(c *ui.Context, s *routeState, disabled bool) {
	route := s.draft.Route()
	scene := p.state.Session.WorldMap.Scene(route.ID)
	if road, err := s.draft.Road(); err == nil {
		scene.Road = road.Preview()
		if replacement := p.previewRoadTileset(); replacement != nil {
			v := p.roadTileset
			if v.route != s.draft || v.roadInput != scene.Road {
				v.route, v.roadInput = s.draft, scene.Road
				v.road, scene.RoadErr = replacement.Preview(s.draft)
			}
			scene.Road = v.road
		}
	}
	if p.backgroundTone == int(worldmap.Darkened) {
		scene.Background, scene.BackgroundErr = scene.DarkBackground, scene.DarkBackgroundErr
	}
	if b := p.backgrounds[p.backgroundTone]; b.preview &&
		b.draft.Current(p.state.Session.WorldMap) {
		scene.Background, scene.BackgroundErr = b.image, nil
	}
	v := &p.routes.canvas
	if !v.loaded || scene.Background != v.background || scene.Road != v.road {
		v.loaded, v.background, v.road = true, scene.Background, scene.Road
		combined := image.NewRGBA(
			image.Rect(0, 0, worldmap.MapWidth, worldmap.MapHeight),
		)
		if scene.Background != nil {
			draw.Draw(
				combined,
				combined.Bounds(),
				scene.Background,
				image.Point{},
				draw.Src,
			)
		}
		if scene.Road != nil {
			draw.Draw(
				combined,
				combined.Bounds(),
				scene.Road,
				image.Point{},
				draw.Over,
			)
		}
		v.bitmap = ui.NewBitmap(combined).Pixelated()
	}
	ui.Column(c.Key("route-map")).Gap(6).Children(func() {
		p.roadTools(c, s, disabled)
		if scene.BackgroundErr != nil || scene.RoadErr != nil {
			ui.Text(c, locales.WorldmapSceneIncomplete()).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
		if v.zoom == "" {
			v.zoom = "100%"
		}
		ui.Row(c.Key("map-controls")).
			Wrap().
			Gap(8).
			AlignItems(ui.Center).
			Children(func() {
				ui.Select(c.Key("zoom"), &v.zoom, []string{"50%", "100%", "200%"}).
					Label(locales.MapZoom()).
					Width(115)
				ui.Text(c, locales.WorldmapControlLines()).
					FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted)
			})
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
		viewWidth := min(width+2, max(120, windowWidth-p.sidebar.Width()-36))
		ui.ScrollBoth(c.Key("route-viewport")).
			Width(viewWidth).
			Height(min(height+2, 342)).
			Shrink(0).
			TrackScroll(&v.scroll).
			Background(c.Theme().Surface).
			BorderWidth(1).
			BorderColor(c.Theme().Border).
			Children(func() {
				canvas := ui.Image(c.Key("route-canvas"), v.bitmap).
					Size(width, height).
					Shrink(0).
					Label(locales.WorldmapRouteCanvas())
				if s.placing && !disabled {
					canvas.Cursor(ui.CursorCrosshair)
				}
				nodes, _ := p.state.Session.WorldMap.Nodes()
				points := route.Points()
				editingRoad := p.roadInput(c, canvas, s, scale, disabled)
				if !editingRoad && canvas.Clicked() && !p.state.Busy {
					x, y, over := canvas.PointerPosition()
					if over {
						point := image.Pt(int(x/scale), int(y/scale))
						if s.placing && !disabled && !s.invalid() &&
							s.selected >= 0 &&
							s.selected < len(s.points) {
							selected := s.points[s.selected]
							selected.values[1].text, selected.values[2].text = strconv.Itoa(
								point.X,
							), strconv.Itoa(
								point.Y,
							)
							s.syncPoints()
						} else {
							nearest, distance := -1, float64(10/scale)
							for i, p := range points {
								if d := math.Hypot(
									float64(p.X-point.X),
									float64(p.Y-point.Y),
								); d < distance {
									nearest, distance = i, d
								}
							}
							if nearest >= 0 {
								s.selected = nearest
							} else {
								for _, node := range nodes {
									if math.Hypot(
										float64(node.Value(worldmap.X)-point.X),
										float64(node.Value(worldmap.Y)-point.Y),
									) < float64(
										8/scale,
									) {
										p.Open(node.ID)
										break
									}
								}
							}
						}
					}
				}
				accent := c.Theme().Accent
				canvas.DrawOver(func(painter *ui.Painter, bounds ui.Rect) {
					painter.Clip(bounds, 0, func() {
						s.roadOverlay(painter, bounds, scale, accent)
						position := func(x, y int) (float32, float32) { return bounds.X + float32(x)*scale, bounds.Y + float32(y)*scale }
						var line *ui.Path
						add := func(x, y int) {
							px, py := position(x, y)
							if line == nil {
								line = (&ui.Path{}).MoveTo(px, py)
							} else {
								line.LineTo(px, py)
							}
						}
						if id := route.Nodes[0]; id >= 0 && id < len(nodes) {
							add(
								nodes[id].Value(worldmap.X),
								nodes[id].Value(worldmap.Y),
							)
						}
						for _, point := range points {
							add(point.X, point.Y)
						}
						if id := route.Nodes[1]; id >= 0 && id < len(nodes) {
							add(
								nodes[id].Value(worldmap.X),
								nodes[id].Value(worldmap.Y),
							)
						}
						if line != nil && route.MovementErr == nil {
							painter.StrokePath(line, 4, ui.RGB(255, 255, 255))
							painter.StrokePath(line, 2, accent)
						}
						for _, node := range nodes {
							x, y := position(
								node.Value(worldmap.X),
								node.Value(worldmap.Y),
							)
							box := ui.Rect{X: x - 7, Y: y - 7, W: 14, H: 14}
							fill := ui.RGB(40, 40, 40)
							if node.ID == route.Nodes[0] ||
								node.ID == route.Nodes[1] {
								fill = accent
							}
							painter.Fill(box, fill, 7)
							painter.Stroke(box, ui.RGB(255, 255, 255), 7, 1)
							painter.Text(
								x+9,
								y-7,
								fmt.Sprintf("%02X", node.ID),
								12,
								ui.RGB(255, 255, 255),
							)
						}
						for i, point := range points {
							x, y := position(point.X, point.Y)
							box := ui.Rect{X: x - 8, Y: y - 8, W: 16, H: 16}
							fill, ink := ui.RGB(
								255,
								255,
								255,
							), ui.RGB(
								30,
								30,
								30,
							)
							if i == s.selected {
								fill, ink = accent, ui.RGB(255, 255, 255)
							}
							painter.Fill(box, fill, 0)
							painter.Stroke(box, accent, 0, 1)
							painter.Text(x-4, y-7, strconv.Itoa(i+1), 12, ink)
						}
					})
				})
			})
		if scene.BackgroundErr != nil {
			ui.Text(c, components.ErrorText(scene.BackgroundErr)).
				FontSize(components.FontCaption)
		}
		if scene.RoadErr != nil {
			ui.Text(c, components.ErrorText(scene.RoadErr)).
				FontSize(components.FontCaption)
		}
	})
}
