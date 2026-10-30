package maps

import (
	"bytes"
	"encoding/binary"
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

type mapLayoutChange struct {
	before, after []byte
}

// MapDraft owns a base layout. Each stroke, rectangle or resize is one
// local history operation; applying the draft creates one document edit.
type Draft struct {
	binding                   edit.Stamp
	chapter, pointer, address int
	resources                 mapResources
	content                   []byte
	width, height             int
	history                   []mapLayoutChange
	cursor                    int
}

func (s *Editor) NewDraft(chapter int) (*Draft, error) {
	s.Refresh()
	format := s.format
	if s.context.Validate(s.image.Data) != nil {
		return nil, edit.ErrVersion
	}
	_, row, _, err := s.records.Resolve(
		records.Chapters,
		chapter,
		LayoutField,
	)
	if err != nil {
		return nil, err
	}
	resources, err := format.resources(s.image.Data, row.Address)
	if err != nil {
		return nil, err
	}
	preview, err := resources.preview(resources.layout)
	if err != nil {
		return nil, err
	}
	list, valid := gba.Pointer(s.image.Data, format.Layout)
	if !valid {
		return nil, ErrMap
	}
	site := list + int(s.image.Data[row.Address+8])*4
	address, valid := gba.Pointer(s.image.Data, site)
	if !valid {
		return nil, ErrMap
	}
	// Retaining a draft need not retain the entire ROM snapshot through its palette.
	resources.palette = bytes.Clone(resources.palette)
	return &Draft{
		binding:   s.context.Bind(s.image.Revision),
		chapter:   chapter,
		pointer:   site,
		address:   address,
		resources: resources,
		content: bytes.Clone(
			resources.layout,
		),
		width:  preview.Width,
		height: preview.Height,
	}, nil
}

func (d *Draft) Current(s *Editor) bool {
	return d.binding.Current(s.context)
}

func (d *Draft) Modified() bool { return !bytes.Equal(d.content, d.resources.layout) }

func (d *Draft) Chapter() int { return d.chapter }

func (d *Draft) Width() int { return d.width }

func (d *Draft) Height() int { return d.height }

func (d *Draft) CanUndo() bool { return d.cursor > 0 }

func (d *Draft) CanRedo() bool { return d.cursor < len(d.history) }

func (d *Draft) Preview() (Preview, error) { return d.resources.preview(d.content) }

func (d *Draft) Cell(x, y int) (Cell, bool) {
	if x < 0 || y < 0 || x >= d.width || y >= d.height {
		return Cell{}, false
	}
	tile := gba.Uint16(d.content, 2+(y*d.width+x)*2)
	return Cell{
		Tile:    tile,
		Terrain: d.resources.config[0x2000+int(tile>>2)],
	}, true
}

// tile resolves a native word through the current tileset and graphics. An
// unused definition may be invalid without invalidating the other choices.
func (r mapResources) tile(word uint16) ([4]uint16, byte, bool) {
	var parts [4]uint16
	at, terrain := int(word)*2, 0x2000+int(word>>2)
	if at+8 > min(len(r.config), 0x2000) || terrain >= len(r.config) {
		return parts, 0, false
	}
	for i := range parts {
		parts[i] = gba.Uint16(r.config, at+i*2)
		if int(parts[i]&0x3FF)*32+32 > len(r.graphics) {
			return parts, 0, false
		}
	}
	return parts, r.config[terrain], true
}

// Tileset packs the readable 16-pixel definitions in native ID order. Cells
// retain native IDs even when unreadable definitions are omitted.
func (d *Draft) Tileset() (Preview, error) {
	result := Preview{Width: 16}
	var definitions [][4]uint16
	for word := 0; word < 4096; word += 4 {
		parts, terrain, valid := d.resources.tile(uint16(word))
		if valid {
			result.Cells = append(
				result.Cells,
				Cell{Tile: uint16(word), Terrain: terrain},
			)
			definitions = append(definitions, parts)
		}
	}
	if len(result.Cells) == 0 {
		return Preview{}, ErrMap
	}
	result.Height = (len(result.Cells) + result.Width - 1) / result.Width
	tiles := make([]uint16, result.Width*result.Height*4)
	for i, parts := range definitions {
		for part, word := range parts {
			x, y := (i%result.Width)*2+part%2, (i/result.Width)*2+part/2
			tiles[y*result.Width*2+x] = word
		}
	}
	var err error
	result.Image, err = gba.Background4BPP(
		d.resources.graphics,
		d.resources.palette,
		tiles,
		result.Width*2,
		result.Height*2,
	)
	return result, err
}

// Stroke joins consecutive samples with cell-sized steps so sparse pointer
// events cannot leave holes. All samples are validated before any mutation.
func (d *Draft) Stroke(samples []image.Point, tile uint16) error {
	if len(samples) == 0 {
		return edit.ErrValue
	}
	for _, p := range samples {
		if _, valid := d.Cell(p.X, p.Y); !valid {
			return edit.ErrValue
		}
	}
	return d.paint(mapStrokePoints(samples), tile)
}

func mapStrokePoints(samples []image.Point) []image.Point {
	points := []image.Point{samples[0]}
	for i := 1; i < len(samples); i++ {
		p, end := samples[i-1], samples[i]
		dx, dy := max(end.X-p.X, p.X-end.X), -max(end.Y-p.Y, p.Y-end.Y)
		sx, sy := 1, 1
		if p.X > end.X {
			sx = -1
		}
		if p.Y > end.Y {
			sy = -1
		}
		err := dx + dy
		for p != end {
			e2 := 2 * err
			if e2 >= dy {
				err += dy
				p.X += sx
			}
			if e2 <= dx {
				err += dx
				p.Y += sy
			}
			points = append(points, p)
		}
	}
	return points
}

// Fill uses a half-open rectangle, matching image.Rectangle.
func (d *Draft) Fill(bounds image.Rectangle, tile uint16) error {
	if bounds.Empty() || !bounds.In(image.Rect(0, 0, d.width, d.height)) {
		return edit.ErrValue
	}
	points := make([]image.Point, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			points = append(points, image.Pt(x, y))
		}
	}
	return d.paint(points, tile)
}

func (d *Draft) paint(points []image.Point, tile uint16) error {
	if _, _, valid := d.resources.tile(tile); !valid {
		return edit.ErrValue
	}
	content := bytes.Clone(d.content)
	for _, p := range points {
		binary.LittleEndian.PutUint16(content[2+(p.Y*d.width+p.X)*2:], tile)
	}
	d.record(content)
	return nil
}

// Layout snapshots keep strokes and resizes in one history. Every subsequent
// operation constructs new bytes, so both sides of an undo entry are immutable.
func (d *Draft) record(content []byte) {
	if bytes.Equal(content, d.content) {
		return
	}
	d.history = append(
		d.history[:d.cursor],
		mapLayoutChange{before: d.content, after: content},
	)
	d.cursor++
	d.restore(content)
}

func (d *Draft) restore(content []byte) {
	d.content = content
	d.width, d.height = int(content[0]), int(content[1])
}

func (d *Draft) Undo() bool {
	if !d.CanUndo() {
		return false
	}
	d.cursor--
	d.restore(d.history[d.cursor].before)
	return true
}

func (d *Draft) Redo() bool {
	if !d.CanRedo() {
		return false
	}
	d.restore(d.history[d.cursor].after)
	d.cursor++
	return true
}

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
