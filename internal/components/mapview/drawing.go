package mapview

import (
	"image"
	"slices"

	"github.com/egoist/mygo/ui"
)

// Tool selects pointer behavior; Pan retains the inspection view's gestures.
type Tool int

const (
	Brush Tool = iota
	Rectangle
	Eyedropper
	Pan
)

// Drawing commits a complete gesture once. Brush is an immutable tile bitmap
// used only for the pending gesture overlay.
type Drawing struct {
	Tool    Tool
	Enabled bool
	Brush   *ui.Bitmap
	Commit  func([]image.Point)
	Pick    func(image.Point)
}

func (v *View) drawInput(canvas ui.Element, scale float32, drawing *Drawing) {
	canvas.Focusable().Disabled(!drawing.Enabled)
	if !drawing.Enabled {
		v.stroke = nil
		return
	}
	if drawing.Tool == Eyedropper {
		v.stroke = nil
		v.chooseCell(canvas, scale, drawing.Pick)
		return
	}
	_, _, pressed := canvas.Dragged()
	if len(v.stroke) > 0 && v.strokeTool != drawing.Tool {
		v.stroke = nil
	}
	if !pressed {
		v.finishStroke(canvas, drawing)
		return
	}
	x, y, over := canvas.PointerPosition()
	point := image.Pt(int(x/(16*scale)), int(y/(16*scale)))
	if _, valid := v.resource.Cell(point.X, point.Y); !over || !valid {
		return
	}
	if len(v.stroke) == 0 {
		v.strokeTool = drawing.Tool
	}
	if len(v.stroke) == 0 || v.stroke[len(v.stroke)-1] != point {
		v.stroke = append(v.stroke, point)
	}
	v.selection, v.selected = point, true
}

func (v *View) finishStroke(canvas ui.Element, drawing *Drawing) {
	if len(v.stroke) == 0 {
		return
	}
	// Consume the release notice so changes made after the toolbar was
	// built are visible in this frame as well.
	canvas.Clicked()
	canvas.DragEnded()
	points := slices.Clone(v.stroke)
	v.stroke = nil
	if drawing.Commit != nil {
		drawing.Commit(points)
	}
}

func (v *View) drawStroke(
	p *ui.Painter,
	r ui.Rect,
	size float32,
	accent ui.Color,
	drawing *Drawing,
) {
	if drawing == nil || len(v.stroke) == 0 {
		return
	}
	box := func(point image.Point) ui.Rect {
		return ui.Rect{
			X: r.X + float32(point.X)*size,
			Y: r.Y + float32(point.Y)*size,
			W: size,
			H: size,
		}
	}
	paint := func(point image.Point) {
		at := box(point)
		if drawing.Brush != nil {
			p.Image(drawing.Brush, at, ui.FillBox)
		}
		p.Stroke(at, accent, 0, 1)
	}
	if drawing.Tool == Rectangle {
		a, b := v.stroke[0], v.stroke[len(v.stroke)-1]
		for y := min(a.Y, b.Y); y <= max(a.Y, b.Y); y++ {
			for x := min(a.X, b.X); x <= max(a.X, b.X); x++ {
				paint(image.Pt(x, y))
			}
		}
		return
	}
	for i, point := range v.stroke {
		if i > 0 {
			a, b := box(v.stroke[i-1]), box(point)
			p.Line(
				a.X+size/2,
				a.Y+size/2,
				b.X+size/2,
				b.Y+size/2,
				max(1, size/3),
				accent,
			)
		}
		paint(point)
	}
}
