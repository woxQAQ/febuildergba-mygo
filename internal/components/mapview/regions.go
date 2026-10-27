package mapview

import (
	"fmt"
	"image"

	"github.com/egoist/mygo/ui"
)

func (v *View) regions(
	p *ui.Painter,
	r ui.Rect,
	cell float32,
	accent ui.Color,
	regions []Region,
) {
	for _, selected := range []bool{false, true} {
		for _, region := range regions {
			if region.Selected != selected {
				continue
			}
			bounds := region.Bounds.Intersect(
				image.Rect(0, 0, v.resource.Width, v.resource.Height),
			)
			if bounds.Empty() {
				continue
			}
			box := ui.Rect{
				X: r.X + float32(bounds.Min.X)*cell,
				Y: r.Y + float32(bounds.Min.Y)*cell,
				W: float32(bounds.Dx()) * cell,
				H: float32(bounds.Dy()) * cell,
			}
			color := ui.RGBA(40, 40, 40, 0.65)
			width := float32(1)
			if selected {
				color = accent
				width = 2
				p.Fill(box, color.Alpha(0.14), 0)
			}
			p.Stroke(box, ui.RGBA(255, 255, 255, 0.8), 0, width+2)
			p.Stroke(box, color, 0, width)
			if cell >= 16 {
				label := ui.Rect{X: box.X, Y: box.Y, W: 19, H: 15}
				p.Fill(label, color, 0)
				p.Text(
					label.X+1,
					label.Y,
					fmt.Sprintf("%02X", region.Index),
					12,
					ui.RGB(255, 255, 255),
				)
			}
		}
	}
}
