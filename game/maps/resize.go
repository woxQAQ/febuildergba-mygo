package maps

import (
	"encoding/binary"
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// MapAnchor aligns the old layout within the resized map. Center alignment
// puts an odd extra column or row on the right or bottom, including cropping.
type Anchor uint8

const (
	MapTopLeft Anchor = iota
	MapTop
	MapTopRight
	MapLeft
	MapCenter
	MapRight
	MapBottomLeft
	MapBottom
	MapBottomRight
)

// Resize preserves retained map words and the uninterpreted decoded tail.
// Only map cells move; deployment, event and path coordinates are unchanged.
func (d *Draft) Resize(
	width, height int,
	anchor Anchor,
	fill uint16,
) (err error) {
	defer func() { err = fault.Wrap(err, "game.resize_map", fault.Context{Resource: LayoutField}) }()
	if width < 1 || width > MaxMapWidth || height < 1 ||
		height > MaxMapHeight ||
		width*height > MaxMapCells ||
		anchor > MapBottomRight {
		return edit.ErrValue
	}
	if _, _, valid := d.resources.tile(fill); !valid {
		return edit.ErrValue
	}
	tail := d.content[2+d.width*d.height*2:]
	if 2+width*height*2+len(tail) > maxMapBytes {
		return edit.ErrValue
	}
	if width == d.width && height == d.height {
		return nil
	}
	content := make([]byte, 2+width*height*2, 2+width*height*2+len(tail))
	content[0], content[1] = byte(
		width,
	), byte(
		height,
	) // #nosec G115 -- Native dimensions are bounded above.
	dx, dy := (width-d.width)*int(anchor%3)/2, (height-d.height)*int(anchor/3)/2
	for y := range height {
		for x := range width {
			word := fill
			if cell, valid := d.Cell(x-dx, y-dy); valid {
				word = cell.Tile
			}
			binary.LittleEndian.PutUint16(content[2+(y*width+x)*2:], word)
		}
	}
	d.record(append(content, tail...))
	return nil
}

func (d *Draft) originalSize() image.Point {
	return image.Pt(int(d.resources.layout[0]), int(d.resources.layout[1]))
}

// Changes are counted by coordinate, including added and removed cells, rather
// than by packed offset; a new row stride does not reinterpret the old layout.
func (d *Draft) changedCells() int {
	before := d.originalSize()
	changed := 0
	for y := range max(before.Y, d.height) {
		for x := range max(before.X, d.width) {
			old := x < before.X && y < before.Y
			cell, current := d.Cell(x, y)
			if old != current ||
				(old && current && cell.Tile != gba.Uint16(d.resources.layout, 2+(y*before.X+x)*2)) {
				changed++
			}
		}
	}
	return changed
}
