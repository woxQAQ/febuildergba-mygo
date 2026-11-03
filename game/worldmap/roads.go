package worldmap

import (
	"encoding/binary"
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
)

const (
	RoadTileSize = 8
	RoadColumns  = MapWidth / RoadTileSize
	RoadRows     = MapHeight / RoadTileSize
)

// RoadCell distinguishes omission from an explicit tile, including transparent
// tiles. An explicit tile replaces earlier roads at this position in the game.
type RoadCell struct {
	Tile                  int
	FlipX, FlipY, Present bool
}

type roadGrid [RoadColumns * RoadRows]RoadCell

type roadStroke struct{ before, after *roadGrid }

// RoadDraft belongs to a RouteDraft. Its history covers complete paint strokes;
// applying the owning route commits roads, endpoints and movement together.
type RoadDraft struct {
	original, cells *roadGrid
	tiles           []*image.RGBA
	palette         *image.RGBA
	preview         *image.RGBA
	history         []roadStroke
	cursor          int
}

func newRoadDraft(
	data []byte,
	format SceneFormat,
	address int,
) (*RoadDraft, error) {
	stream, _, err := readRoad(data, address)
	if err != nil {
		return nil, err
	}
	tiles, err := format.roadTiles(data)
	if err != nil {
		return nil, err
	}
	grid := new(roadGrid)
	for _, tile := range stream {
		if tile.word&0xF000 != 0 || int(tile.word&0x3FF) >= len(tiles) {
			return nil, ErrScene
		}
		grid[tile.index] = RoadCell{
			Tile:    int(tile.word & 0x3FF),
			FlipX:   tile.word&0x400 != 0,
			FlipY:   tile.word&0x800 != 0,
			Present: true,
		}
	}
	return &RoadDraft{original: grid, cells: grid, tiles: tiles}, nil
}

func (f SceneFormat) roadTiles(data []byte) ([]*image.RGBA, error) {
	resource, err := f.readRoadTiles(data)
	if err != nil {
		return nil, err
	}
	return resource.tiles()
}

func (d *RoadDraft) Modified() bool { return *d.cells != *d.original }
func (d *RoadDraft) CanUndo() bool  { return d.cursor > 0 }
func (d *RoadDraft) CanRedo() bool  { return d.cursor < len(d.history) }
func (d *RoadDraft) TileCount() int { return len(d.tiles) }

func (d *RoadDraft) Cell(x, y int) (RoadCell, bool) {
	if x < 0 || y < 0 || x >= RoadColumns || y >= RoadRows {
		return RoadCell{}, false
	}
	return d.cells[y*RoadColumns+x], true
}

// Tileset uses native tile order in sixteen columns. Cells beyond TileCount
// in the final row are padding and cannot be selected.
func (d *RoadDraft) Tileset() *image.RGBA {
	if d.palette == nil {
		d.palette = roadTilesImage(d.tiles)
	}
	return d.palette
}

func roadTilesImage(tiles []*image.RGBA) *image.RGBA {
	out := image.NewRGBA(
		image.Rect(0, 0, 16*RoadTileSize, ((len(tiles)+15)/16)*RoadTileSize),
	)
	for i, tile := range tiles {
		x, y := (i%16)*RoadTileSize, (i/16)*RoadTileSize
		draw.Draw(
			out,
			image.Rect(x, y, x+RoadTileSize, y+RoadTileSize),
			tile,
			image.Point{},
			draw.Src,
		)
	}
	return out
}

func (d *RoadDraft) Preview() *image.RGBA {
	if d.preview == nil {
		d.preview = roadImage(d.cells, d.tiles)
	}
	return d.preview
}

func roadImage(cells *roadGrid, tiles []*image.RGBA) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, MapWidth, MapHeight))
	for i, cell := range cells {
		if cell.Present {
			paintRoadTile(
				out,
				image.Pt(
					i%RoadColumns*RoadTileSize,
					i/RoadColumns*RoadTileSize,
				),
				tiles[cell.Tile],
				cell,
			)
		}
	}
	return out
}

func (d *RoadDraft) Brush(
	cell RoadCell,
) (*image.RGBA, error) {
	return roadBrush(d.tiles, cell)
}

func roadBrush(tiles []*image.RGBA, cell RoadCell) (*image.RGBA, error) {
	if !cell.Present || cell.Tile < 0 || cell.Tile >= len(tiles) {
		return nil, edit.ErrValue
	}
	out := image.NewRGBA(image.Rect(0, 0, RoadTileSize, RoadTileSize))
	paintRoadTile(out, image.Point{}, tiles[cell.Tile], cell)
	return out, nil
}

func paintRoadTile(
	out *image.RGBA,
	at image.Point,
	tile *image.RGBA,
	cell RoadCell,
) {
	for y := range RoadTileSize {
		for x := range RoadTileSize {
			sx, sy := x, y
			if cell.FlipX {
				sx = RoadTileSize - 1 - x
			}
			if cell.FlipY {
				sy = RoadTileSize - 1 - y
			}
			out.SetRGBA(at.X+x, at.Y+y, tile.RGBAAt(sx, sy))
		}
	}
}

// RoadStrokeCells joins sparse pointer samples at cell resolution. It also
// supplies the same pending-stroke geometry to the canvas before commitment.
func RoadStrokeCells(samples []image.Point) ([]image.Point, error) {
	if len(samples) == 0 {
		return nil, edit.ErrValue
	}
	for _, p := range samples {
		if !p.In(image.Rect(0, 0, RoadColumns, RoadRows)) {
			return nil, edit.ErrValue
		}
	}
	var used [RoadColumns * RoadRows]bool
	var points []image.Point
	add := func(p image.Point) {
		index := p.Y*RoadColumns + p.X
		if !used[index] {
			used[index] = true
			points = append(points, p)
		}
	}
	add(samples[0])
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
			add(p)
		}
	}
	return points, nil
}

// Stroke validates every sample and the selected tile before recording one
// atomic draft operation. An absent cell erases; it never substitutes tile zero.
func (d *RoadDraft) Stroke(samples []image.Point, cell RoadCell) error {
	if cell.Present && (cell.Tile < 0 || cell.Tile >= len(d.tiles)) {
		return edit.ErrValue
	}
	if !cell.Present {
		cell = RoadCell{}
	}
	points, err := RoadStrokeCells(samples)
	if err != nil {
		return err
	}
	cells := new(*d.cells)
	for _, point := range points {
		cells[point.Y*RoadColumns+point.X] = cell
	}
	if *cells == *d.cells {
		return nil
	}
	d.history = append(d.history[:d.cursor], roadStroke{d.cells, cells})
	d.cursor++
	d.cells, d.preview = cells, nil
	return nil
}

func (d *RoadDraft) Undo() bool {
	if !d.CanUndo() {
		return false
	}
	d.cursor--
	d.cells, d.preview = d.history[d.cursor].before, nil
	return true
}

func (d *RoadDraft) Redo() bool {
	if !d.CanRedo() {
		return false
	}
	d.cells, d.preview = d.history[d.cursor].after, nil
	d.cursor++
	return true
}

type RoadChange struct {
	Position      image.Point
	Before, After RoadCell
}

func (d *RoadDraft) changes() []RoadChange {
	var changes []RoadChange
	for i, after := range d.cells {
		if before := d.original[i]; before != after {
			changes = append(
				changes,
				RoadChange{
					image.Pt(i%RoadColumns, i/RoadColumns),
					before,
					after,
				},
			)
		}
	}
	return changes
}

// encode emits maximal contiguous runs within each row. Gaps are omitted, not
// filled with transparent tiles; duplicate source writes have already resolved
// in engine order. The original stream and its trailing bytes remain untouched.
func (d *RoadDraft) encode() []byte {
	var out []byte
	for y := range RoadRows {
		for x := 0; x < RoadColumns; {
			if !d.cells[y*RoadColumns+x].Present {
				x++
				continue
			}
			end := x + 1
			for end < RoadColumns && d.cells[y*RoadColumns+end].Present {
				end++
			}
			out = append(
				out,
				byte(x),
				byte(y),
				byte(end-x),
				1,
			) // #nosec G115 -- Runs fit within the fixed RoadColumns by RoadRows grid.
			for ; x < end; x++ {
				cell := d.cells[y*RoadColumns+x]
				word := uint16(cell.Tile & 0x3FF)
				if cell.FlipX {
					word |= 0x400
				}
				if cell.FlipY {
					word |= 0x800
				}
				out = binary.LittleEndian.AppendUint16(out, word)
			}
		}
	}
	return append(out, 0xFF, 0, 0, 0)
}
