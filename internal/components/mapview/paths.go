package mapview

import (
	"fmt"
	"image"

	"github.com/egoist/mygo/ui"
)

// Path joins encoded absolute destinations; it does not simulate terrain routing.
type Path struct {
	Start    image.Point
	Steps    []image.Point
	Selected int
}

func (v *View) path(
	p *ui.Painter,
	r ui.Rect,
	cell float32,
	accent ui.Color,
	path *Path,
) {
	if path == nil {
		return
	}
	center := func(point image.Point) (float32, float32) {
		return r.X + (float32(point.X)+0.5)*cell, r.Y + (float32(point.Y)+0.5)*cell
	}
	p.Clip(r, 0, func() {
		previous := path.Start
		for _, point := range path.Steps {
			x1, y1 := center(previous)
			x2, y2 := center(point)
			p.Line(x1, y1, x2, y2, 4, ui.RGB(255, 255, 255))
			p.Line(x1, y1, x2, y2, 2, accent)
			previous = point
		}
		for index, point := range path.Steps {
			if _, valid := v.resource.Cell(point.X, point.Y); !valid {
				continue
			}
			x, y := center(point)
			size := min(cell-3, float32(17))
			box := ui.Rect{X: x - size/2, Y: y - size/2, W: size, H: size}
			fill, text := ui.RGB(255, 255, 255), ui.RGB(30, 30, 30)
			border := float32(1)
			if index == path.Selected {
				fill, text, border = accent, ui.RGB(255, 255, 255), 2
			}
			p.Fill(box, fill, 0)
			p.Stroke(box, ui.RGB(255, 255, 255), 0, border+2)
			p.Stroke(box, accent, 0, border)
			if cell >= 16 {
				p.Text(
					box.X+1,
					box.Y,
					fmt.Sprintf("%02X", index),
					min(cell*0.6, 12),
					text,
				)
			}
		}
	})
}
