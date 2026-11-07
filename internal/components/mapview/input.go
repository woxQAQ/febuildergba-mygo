package mapview

import (
	"image"

	"github.com/egoist/mygo/ui"
)

func (v *View) inspectInput(
	canvas ui.Element,
	scale float32,
	choose func(image.Point),
) {
	v.stroke = nil
	// A press the system canceled ends the pan without a release; the
	// completed viewport movement stays.
	if canvas.PressCanceled() {
		v.panning = false
		return
	}
	if dx, dy, dragging := canvas.Dragged(); dragging {
		v.scroll.X -= dx
		if dx != 0 || dy != 0 {
			v.panning = true
			canvas.Cursor(ui.CursorGrabbing)
		}
	}
	if !v.panning {
		v.chooseCell(canvas, scale, choose)
	}
	if _, _, ended := canvas.DragEnded(); ended {
		v.panning = false
	}
}

func (v *View) chooseCell(
	canvas ui.Element,
	scale float32,
	choose func(image.Point),
) {
	if !canvas.Clicked() {
		return
	}
	x, y, over := canvas.PointerPosition()
	if !over {
		return
	}
	point := image.Pt(int(x/(16*scale)), int(y/(16*scale)))
	if _, valid := v.resource.Cell(point.X, point.Y); !valid {
		return
	}
	v.selection, v.selected = point, true
	if choose != nil {
		choose(point)
	}
}
