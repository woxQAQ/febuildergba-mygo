package worldmaps

import (
	"fmt"
	"image"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/icons"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type roadTool int

const (
	roadInspect roadTool = iota
	roadPaint
	roadErase
	roadPick
	roadPan
)

type roadPainter struct {
	tool                        int
	grid                        bool
	brush                       worldmap.RoadCell
	brushBitmap                 *ui.Bitmap
	shownBrush                  worldmap.RoadCell
	palette                     []*ui.Bitmap
	atlas                       *image.RGBA
	stroke                      []image.Point
	strokeTool                  roadTool
	strokeBrush                 worldmap.RoadCell
	selection                   image.Point
	selected                    bool
	canceled                    bool
	history, canvas             ui.Handle
	historyActive, restoreFocus bool
}

func (p *Page) activateRoute(s *routeState) {
	if p.routes.active == s {
		return
	}
	if previous := p.routes.active; previous != nil {
		previous.clearRoadGesture()
		previous.road.historyActive = false
	}
	p.routes.active = s
}

func (s *routeState) clearRoadGesture() {
	s.road.stroke = nil
	s.road.canceled = false
}

func (p *Page) roadHistory(c *ui.Context, s *routeState, disabled bool) {
	r := &s.road
	road, err := s.draft.Road()
	if err != nil {
		return
	}
	r.historyActive = r.history.FocusWithin(c) ||
		r.historyActive && !c.Root().FocusWithin()
	undo := func() {
		if !p.state.Busy && s.draft.Current(p.state.Session.WorldMap) &&
			road.Undo() {
			s.plan, s.err = nil, nil
			s.clearRoadGesture()
			r.restoreFocus = roadTool(r.tool) != roadInspect
		}
	}
	redo := func() {
		if !p.state.Busy && s.draft.Current(p.state.Session.WorldMap) &&
			road.Redo() {
			s.plan, s.err = nil, nil
			s.clearRoadGesture()
			r.restoreFocus = roadTool(r.tool) != roadInspect
		}
	}
	if r.historyActive {
		p.state.DraftHistory = &editor.History{}
		if !disabled && road.CanUndo() {
			p.state.DraftHistory.Undo = undo
		}
		if !disabled && road.CanRedo() {
			p.state.DraftHistory.Redo = redo
		}
	}
	if disabled {
		s.clearRoadGesture()
	}
	ui.Row(c.Key("road-history")).Wrap().Gap(8).Children(func() {
		components.ActionButton(c, icons.Lucide.CornerUpLeft(), locales.WorldmapUndoRoad()).
			Disabled(disabled || !road.CanUndo()).
			OnClick(undo)
		components.ActionButton(c, icons.Lucide.CornerUpRight(), locales.WorldmapRedoRoad()).
			Disabled(disabled || !road.CanRedo()).
			OnClick(redo)
	})
}

func (p *Page) roadTools(c *ui.Context, s *routeState, disabled bool) {
	r := &s.road
	road, err := s.draft.Road()
	if err != nil {
		return
	}
	if !r.brush.Present {
		r.brush = worldmap.RoadCell{Present: true}
	}
	atlas := road.Tileset()
	if p.previewRoadTileset() != nil {
		atlas = p.roadTileset.image
	}
	if r.atlas != atlas {
		r.atlas, r.palette, r.brushBitmap = atlas, nil, nil
		columns := atlas.Bounds().Dx() / worldmap.RoadTileSize
		for i := range road.TileCount() {
			x, y := (i%columns)*worldmap.RoadTileSize, (i/columns)*worldmap.RoadTileSize
			r.palette = append(
				r.palette,
				ui.NewBitmap(atlas.SubImage(image.Rect(x, y, x+worldmap.RoadTileSize, y+worldmap.RoadTileSize))).
					Pixelated(),
			)
		}
	}
	ui.Row(c.Key("road-tools")).
		Wrap().
		Gap(8).
		AlignItems(ui.Center).
		Children(func() {
			components.IconSegments(
				c.Key("tool"),
				&r.tool,
				components.IconOption{
					Icon:  icons.Lucide.MousePointer2(),
					Label: locales.WorldmapInspect(),
				},
				components.IconOption{
					Icon:  icons.Lucide.Paintbrush(),
					Label: locales.MapBrush(),
				},
				components.IconOption{
					Icon:  icons.Lucide.Eraser(),
					Label: locales.WorldmapEraseRoad(),
				},
				components.IconOption{
					Icon:  icons.Lucide.Pipette(),
					Label: locales.MapEyedropper(),
				},
				components.IconOption{
					Icon:  icons.Lucide.Hand(),
					Label: locales.MapPan(),
				},
			).
				Label(locales.WorldmapRoadTools()).
				Disabled(disabled).
				OnChange(func() { s.clearRoadGesture(); s.placing = false })
			components.ActionToggle(c, &r.grid, icons.Lucide.Grid2x2(), locales.MapGrid()).
				Disabled(disabled)
		})
	if roadTool(r.tool) == roadInspect || roadTool(r.tool) == roadPan {
		return
	}
	ui.Text(c, locales.WorldmapRoadHelp()).
		FontSize(components.FontCaption).
		TextColor(c.Theme().TextMuted)
	if roadTool(r.tool) != roadErase {
		ui.Row(c.Key("brush-properties")).
			Wrap().
			Gap(8).
			AlignItems(ui.Center).
			Children(func() {
				ui.Textf(c, "%s %02X", locales.MapTile(), r.brush.Tile).
					FontFeatures("tnum")
				components.ActionToggle(c, &r.brush.FlipX, icons.Lucide.FlipHorizontal(), locales.WorldmapFlipX()).
					Disabled(disabled)
				components.ActionToggle(c, &r.brush.FlipY, icons.Lucide.FlipVertical(), locales.WorldmapFlipY()).
					Disabled(disabled)
			})
		width, _ := c.Size()
		columns := max(1, int((width-p.sidebar.Width()-36)/31))
		height := min(96, float32((len(r.palette)+columns-1)/columns)*31)
		ui.Scroll(c.Key("road-palette")).
			Height(height).
			MaxWidthPercent(100).
			Children(func() {
				ui.Row(c).Wrap().Gap(3).Children(func() {
					for i, bitmap := range r.palette {
						label := fmt.Sprintf(
							"%s %02X",
							locales.WorldmapRoadTile(),
							i,
						)
						ui.ButtonBase(c.Key(i)).
							Size(28, 28).
							Padding(2).
							Label(label).
							Tooltip(label).
							Selected(i == r.brush.Tile).
							Disabled(disabled).
							BorderWidth(1).
							BorderColor(c.Theme().Border).
							OnClick(func() { r.brush.Tile = i; r.tool = int(roadPaint); s.clearRoadGesture() }).
							Children(func() { ui.Image(c, bitmap).Size(24, 24) })
					}
				})
			})
	}
	if r.brushBitmap == nil || r.shownBrush != r.brush {
		brush, err := road.Brush(r.brush)
		if replacement := p.previewRoadTileset(); replacement != nil {
			brush, err = replacement.Brush(r.brush)
		}
		if err == nil {
			r.brushBitmap = ui.NewBitmap(brush).Pixelated()
			r.shownBrush = r.brush
		}
	}
}

// roadInput accumulates one gesture. The domain joins sparse samples;
// no ROM or draft writes occur before release. Cancellation discards the stroke.
func (p *Page) roadInput(
	c *ui.Context,
	canvas ui.Element,
	s *routeState,
	scale float32,
	disabled bool,
) bool {
	r := &s.road
	road, err := s.draft.Road()
	if err != nil || roadTool(r.tool) == roadInspect {
		s.clearRoadGesture()
		return false
	}
	canvas.Focusable().Bind(&r.canvas).Disabled(disabled)
	if r.restoreFocus && !disabled {
		r.canvas.Focus(c)
		r.restoreFocus = false
	}
	canvas.OnShortcut(
		0,
		ui.KeyEscape,
		func() { r.stroke = nil; r.canceled = true },
	)
	if disabled {
		s.clearRoadGesture()
		return true
	}
	if roadTool(r.tool) == roadPan {
		if dx, dy, dragging := canvas.Dragged(); dragging {
			p.routes.canvas.scroll.X -= dx
			p.routes.canvas.scroll.Y -= dy
		}
		canvas.Cursor(ui.CursorGrab)
		return true
	}
	canvas.Cursor(ui.CursorCrosshair)
	for _, move := range []struct {
		key    ui.Key
		offset image.Point
	}{{ui.KeyLeft, image.Pt(-1, 0)}, {ui.KeyRight, image.Pt(1, 0)}, {ui.KeyUp, image.Pt(0, -1)}, {ui.KeyDown, image.Pt(0, 1)}} {
		canvas.OnShortcut(0, move.key, func() {
			point := r.selection.Add(move.offset)
			if _, ok := road.Cell(point.X, point.Y); ok {
				r.selection = point
				r.selected = true
			}
		})
	}
	canvas.OnShortcut(0, ui.KeySpace, func() {
		if p.state.Busy || !s.draft.Current(p.state.Session.WorldMap) ||
			!r.selected {
			return
		}
		if roadTool(r.tool) == roadPick {
			p.pickRoad(s, road, r.selection)
			return
		}
		cell := r.brush
		if roadTool(r.tool) == roadErase {
			cell = worldmap.RoadCell{}
		}
		s.err = road.Stroke([]image.Point{r.selection}, cell)
		s.plan = nil
	})
	x, y, over := canvas.PointerPosition()
	point := image.Pt(
		int(x/(worldmap.RoadTileSize*scale)),
		int(y/(worldmap.RoadTileSize*scale)),
	)
	_, valid := road.Cell(point.X, point.Y)
	if roadTool(r.tool) == roadPick {
		if canvas.Clicked() && over && valid {
			r.selection, r.selected = point, true
			p.pickRoad(s, road, point)
		}
		return true
	}
	_, _, pressed := canvas.Dragged()
	if r.canceled {
		if !pressed {
			r.canceled = false
		}
		return true
	}
	if pressed {
		if !over || !valid {
			return true
		}
		if len(r.stroke) == 0 {
			r.strokeTool = roadTool(r.tool)
			r.strokeBrush = r.brush
		}
		if len(r.stroke) == 0 || r.stroke[len(r.stroke)-1] != point {
			r.stroke = append(r.stroke, point)
		}
		r.selection, r.selected = point, true
		return true
	}
	if len(r.stroke) > 0 {
		clicked := canvas.Clicked()
		_, _, released := canvas.DragEnded()
		samples := slices.Clone(r.stroke)
		cell := r.strokeBrush
		if r.strokeTool == roadErase {
			cell = worldmap.RoadCell{}
		}
		s.clearRoadGesture()
		if !clicked && !released {
			return true
		}
		if !p.state.Busy && s.draft.Current(p.state.Session.WorldMap) {
			s.err = road.Stroke(samples, cell)
			s.plan = nil
		}
	}
	return true
}

func (p *Page) pickRoad(
	s *routeState,
	road *worldmap.RoadDraft,
	point image.Point,
) {
	cell, ok := road.Cell(point.X, point.Y)
	if ok && cell.Present {
		s.road.brush = cell
		s.road.tool = int(roadPaint)
	}
}

func (s *routeState) roadOverlay(
	p *ui.Painter,
	r ui.Rect,
	scale float32,
	accent ui.Color,
) {
	v := &s.road
	size := float32(worldmap.RoadTileSize) * scale
	box := func(point image.Point) ui.Rect {
		return ui.Rect{
			X: r.X + float32(point.X)*size,
			Y: r.Y + float32(point.Y)*size,
			W: size,
			H: size,
		}
	}
	if v.grid {
		for x := 1; x < worldmap.RoadColumns; x++ {
			p.Line(
				r.X+float32(x)*size,
				r.Y,
				r.X+float32(x)*size,
				r.Y+r.H,
				0.5,
				accent,
			)
		}
		for y := 1; y < worldmap.RoadRows; y++ {
			p.Line(
				r.X,
				r.Y+float32(y)*size,
				r.X+r.W,
				r.Y+float32(y)*size,
				0.5,
				accent,
			)
		}
	}
	points, _ := worldmap.RoadStrokeCells(v.stroke)
	for _, point := range points {
		at := box(point)
		if v.strokeTool == roadPaint && v.brushBitmap != nil {
			p.Image(v.brushBitmap, at, ui.FillBox)
		}
		p.Stroke(at, accent, 0, 1)
		if v.strokeTool == roadErase {
			p.Line(at.X, at.Y, at.X+at.W, at.Y+at.H, 1, accent)
		}
	}
	if v.selected && roadTool(v.tool) != roadInspect {
		p.Stroke(box(v.selection), ui.RGB(255, 255, 255), 0, 3)
		p.Stroke(box(v.selection), accent, 0, 1)
	}
}
