package game

import (
	"bytes"
	"encoding/binary"
	"image"
	"maps"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type mapLayoutChange struct {
	before, after []byte
}

// MapDraft owns a base layout. Each stroke, rectangle or resize is one
// local history operation; applying the draft creates one document edit.
type MapDraft struct {
	document                  *rom.Document
	revision                  uint64
	layouts                   map[Kind]Layout
	chapter, pointer, address int
	resources                 mapResources
	content                   []byte
	width, height             int
	history                   []mapLayoutChange
	cursor                    int
}

func (s *Session) NewMapDraft(chapter int) (*MapDraft, error) {
	s.Refresh()
	format, ok := s.Adapter.(gameFormat)
	if !ok || !format.Match(s.image.Data) {
		return nil, ErrVersion
	}
	_, row, _, err := s.locate(Chapters, chapter, chapterMapField)
	if err != nil {
		return nil, err
	}
	resources, err := format.maps.resources(s.image.Data, row.Address)
	if err != nil {
		return nil, err
	}
	preview, err := resources.preview(resources.layout)
	if err != nil {
		return nil, err
	}
	list, valid := pointer(s.image.Data, format.maps.layout)
	if !valid {
		return nil, ErrMap
	}
	site := list + int(s.image.Data[row.Address+8])*4
	address, valid := pointer(s.image.Data, site)
	if !valid {
		return nil, ErrMap
	}
	// Retaining a draft need not retain the entire ROM snapshot through its palette.
	resources.palette = bytes.Clone(resources.palette)
	return &MapDraft{
		document:  s.Document,
		revision:  s.image.Revision,
		layouts:   maps.Clone(s.layouts),
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

func (d *MapDraft) Current(s *Session) bool {
	return d.document == s.Document &&
		d.revision == s.Document.State().Revision &&
		maps.Equal(d.layouts, s.layouts)
}

func (d *MapDraft) Modified() bool { return !bytes.Equal(d.content, d.resources.layout) }
func (d *MapDraft) Chapter() int   { return d.chapter }
func (d *MapDraft) Width() int     { return d.width }
func (d *MapDraft) Height() int    { return d.height }
func (d *MapDraft) CanUndo() bool  { return d.cursor > 0 }
func (d *MapDraft) CanRedo() bool  { return d.cursor < len(d.history) }

func (d *MapDraft) Preview() (MapPreview, error) { return d.resources.preview(d.content) }

func (d *MapDraft) Cell(x, y int) (MapCell, bool) {
	if x < 0 || y < 0 || x >= d.width || y >= d.height {
		return MapCell{}, false
	}
	tile := u16(d.content, 2+(y*d.width+x)*2)
	return MapCell{
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
		parts[i] = u16(r.config, at+i*2)
		if int(parts[i]&0x3FF)*32+32 > len(r.graphics) {
			return parts, 0, false
		}
	}
	return parts, r.config[terrain], true
}

// Tileset packs the readable 16-pixel definitions in native ID order. Cells
// retain native IDs even when unreadable definitions are omitted.
func (d *MapDraft) Tileset() (MapPreview, error) {
	result := MapPreview{Width: 16}
	var definitions [][4]uint16
	for word := 0; word < 4096; word += 4 {
		parts, terrain, valid := d.resources.tile(uint16(word))
		if valid {
			result.Cells = append(
				result.Cells,
				MapCell{Tile: uint16(word), Terrain: terrain},
			)
			definitions = append(definitions, parts)
		}
	}
	if len(result.Cells) == 0 {
		return MapPreview{}, ErrMap
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
func (d *MapDraft) Stroke(samples []image.Point, tile uint16) error {
	if len(samples) == 0 {
		return ErrValue
	}
	for _, p := range samples {
		if _, valid := d.Cell(p.X, p.Y); !valid {
			return ErrValue
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
func (d *MapDraft) Fill(bounds image.Rectangle, tile uint16) error {
	if bounds.Empty() || !bounds.In(image.Rect(0, 0, d.width, d.height)) {
		return ErrValue
	}
	points := make([]image.Point, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			points = append(points, image.Pt(x, y))
		}
	}
	return d.paint(points, tile)
}

func (d *MapDraft) paint(points []image.Point, tile uint16) error {
	if _, _, valid := d.resources.tile(tile); !valid {
		return ErrValue
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
func (d *MapDraft) record(content []byte) {
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

func (d *MapDraft) restore(content []byte) {
	d.content = content
	d.width, d.height = int(content[0]), int(content[1])
}

func (d *MapDraft) Undo() bool {
	if !d.CanUndo() {
		return false
	}
	d.cursor--
	d.restore(d.history[d.cursor].before)
	return true
}

func (d *MapDraft) Redo() bool {
	if !d.CanRedo() {
		return false
	}
	d.restore(d.history[d.cursor].after)
	d.cursor++
	return true
}
