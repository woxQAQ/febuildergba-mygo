// ui-probe exercises native capabilities needed by later map and script
// editors.
package main

import (
	"image"
	"image/color"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type probe struct {
	bitmap *ui.Bitmap
	code   *editor.State
	zoom   int
	x, y   int
	input  string
}

func newProbe() *probe {
	pixels := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			c := color.RGBA{R: 48, G: 85, B: 112, A: 255}
			if (x/8+y/8)%2 == 0 {
				c = color.RGBA{R: 216, G: 226, B: 232, A: 255}
			}
			pixels.SetRGBA(x, y, c)
		}
	}
	bitmap := ui.NewBitmap(pixels)
	bitmap.Pixelated()
	code := &editor.State{
		Buffer: editor.NewBuffer(
			strings.Repeat("// 章节脚本 / Chapter script\n", 10000),
		),
	}
	return &probe{bitmap: bitmap, code: code, zoom: 3, x: -1, y: -1}
}

func (p *probe) view(c *ui.Context) {
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Text(c, "原生交互验证 / Native interaction probe").FontSize(20).Bold()
		ui.Text(c, "此窗口使用合成图块和 10,000 行文本，不读写 ROM。")
		ui.Row(c).Gap(12).Children(func() {
			ui.Column(c).Width(300).Gap(10).Children(func() {
				ui.Tabs(c, &p.zoom, "1×", "2×", "4×", "8×")
				scale := float32(uint32(1) << p.zoom)
				canvas := ui.Image(c, p.bitmap).
					Size(32*scale, 32*scale).
					Label("Tile canvas").
					Cursor(ui.CursorCrosshair)
				if canvas.Clicked() {
					x, y, ok := canvas.PointerPosition()
					if ok {
						p.x, p.y = int(x/scale), int(y/scale)
					}
				}
				ui.Textf(c, "Pixel: %d, %d", p.x, p.y)
				ui.TextInput(c, &p.input).
					Label("IME input").
					Placeholder("中文输入法 / IME")
				ui.Text(c, p.input)
			})
			editor.View(c, p.code).Label("Script editor").Grow(1).Height(560)
		}).Grow(1)
	})
}

func main() {
	p := newProbe()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(
			mygo.WindowOptions{
				Title:   "FEBuilderGBA native probe",
				Width:   1100,
				Height:  700,
				Content: ui.View(p.view),
			},
		)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
