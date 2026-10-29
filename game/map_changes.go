package game

import (
	"bytes"
	"encoding/binary"
	"image"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

const (
	mapChangesField = "map_changes_id"
	maxMapChanges   = 256
)

var (
	ErrMapChanges = fault.Code{
		ID:   "game.invalid_map_changes",
		Kind: fault.InvalidData,
	}
	ErrMapChangeReference = fault.Code{
		ID:   "game.map_change_reference",
		Kind: fault.Conflict,
	}
)

// MapChange identifies a table record; ID is the native event ID, not its index.
// A zero tile in its payload preserves the current map cell.
type MapChange struct {
	ID, Address int
	Bounds      image.Rectangle
}

type mapChangeRecord struct {
	header               [12]byte
	address              int
	tiles, originalTiles []byte
}

func (r mapChangeRecord) info() MapChange {
	x, y := int(r.header[1]), int(r.header[2])
	return MapChange{
		ID: int(r.header[0]), Address: r.address,
		Bounds: image.Rect(x, y, x+int(r.header[3]), y+int(r.header[4])),
	}
}

type mapChangeTable struct {
	pointer, address int
	rows             []mapChangeRecord
	terminator       []byte
}

// readMapChanges follows the change-specific PLIST, including a valid ID zero.
// The signed terminal ID is sufficient even when the last record is short.
func readMapChanges(data []byte, root int, id int64) (mapChangeTable, error) {
	list, ok := pointer(data, root)
	if !ok || list < 0xC0 || list%4 != 0 || id < 0 || id > 255 ||
		!within(data, list+int(id)*4, 4) {
		return mapChangeTable{}, ErrMapChanges
	}
	result := mapChangeTable{pointer: list + int(id)*4}
	if u32(data, result.pointer) == 0 {
		result.terminator = make([]byte, 12)
		result.terminator[0] = 0xFF
		return result, nil
	}
	result.address, ok = pointer(data, result.pointer)
	if !ok || result.address < 0xC0 || result.address%4 != 0 {
		return mapChangeTable{}, ErrMapChanges
	}
	for index := 0; index <= maxMapChanges; index++ {
		at := result.address + index*12
		if !within(data, at, 1) {
			return mapChangeTable{}, ErrMapChanges
		}
		if data[at] >= 0x80 {
			result.terminator = bytes.Clone(data[at:min(at+12, len(data))])
			return result, nil
		}
		if index == maxMapChanges || !within(data, at, 12) {
			return mapChangeTable{}, ErrMapChanges
		}
		row := mapChangeRecord{address: at}
		copy(row.header[:], data[at:at+12])
		area := int(row.header[3]) * int(row.header[4])
		if area > MaxMapCells {
			return mapChangeTable{}, ErrMapChanges
		}
		if area != 0 {
			payload, valid := pointer(data, at+8)
			if !valid || payload < 0xC0 || payload%2 != 0 ||
				!within(data, payload, area*2) {
				return mapChangeTable{}, ErrMapChanges
			}
			row.tiles = bytes.Clone(data[payload : payload+area*2])
			row.originalTiles = row.tiles
		}
		result.rows = append(result.rows, row)
	}
	return mapChangeTable{}, ErrMapChanges
}

func (s *Session) mapChanges(chapter int) (mapChangeTable, error) {
	format, ok := s.Adapter.(gameFormat)
	if !ok || !format.Match(s.image.Data) {
		return mapChangeTable{}, ErrVersion
	}
	id, err := s.Value(Chapters, chapter, mapChangesField)
	if err != nil {
		return mapChangeTable{}, err
	}
	return readMapChanges(s.image.Data, format.maps.changes, id)
}

func (s *Session) MapChanges(chapter int) ([]MapChange, error) {
	s.Refresh()
	table, err := s.mapChanges(chapter)
	if err != nil {
		return nil, err
	}
	return mapChangeInfos(table.rows), nil
}

func mapChangeInfos(rows []mapChangeRecord) []MapChange {
	result := make([]MapChange, len(rows))
	for index, row := range rows {
		result[index] = row.info()
	}
	return result
}

// Native ID lookup chooses the first matching record. Coordinate lookup returns
// the ID of the last covering rectangle, then uses that same ID lookup.
func mapChangeIndex(rows []mapChangeRecord, id int) int {
	return slices.IndexFunc(
		rows,
		func(row mapChangeRecord) bool { return int(row.header[0]) == id },
	)
}

func mapChangeAt(rows []mapChangeRecord, point image.Point) int {
	id := -1
	for _, row := range rows {
		if point.In(row.info().Bounds) {
			id = int(row.header[0])
		}
	}
	return id
}

type mapChangesHistory struct{ before, after []mapChangeRecord }

// MapChangesDraft keeps table order, IDs, padding and independent payload
// ownership. Row snapshots and their tile slices are immutable after recording.
type MapChangesDraft struct {
	base     *MapDraft
	original mapChangeTable
	rows     []mapChangeRecord
	history  []mapChangesHistory
	cursor   int
}

func (s *Session) NewMapChangesDraft(chapter int) (*MapChangesDraft, error) {
	base, err := s.NewMapDraft(chapter)
	if err != nil {
		return nil, err
	}
	table, err := s.mapChanges(chapter)
	if err != nil {
		return nil, err
	}
	return &MapChangesDraft{
		base:     base,
		original: table,
		rows:     slices.Clone(table.rows),
	}, nil
}

func (d *MapChangesDraft) Current(
	s *Session,
) bool {
	return d.base.Current(s)
}

func (d *MapChangesDraft) Chapter() int { return d.base.Chapter() }

func (d *MapChangesDraft) Entries() []MapChange { return mapChangeInfos(d.rows) }

func (d *MapChangesDraft) Tileset() (MapPreview, error) { return d.base.Tileset() }

func (d *MapChangesDraft) Size() image.Point { return image.Pt(d.base.Width(), d.base.Height()) }
func (d *MapChangesDraft) CanUndo() bool     { return d.cursor > 0 }

func (d *MapChangesDraft) CanRedo() bool { return d.cursor < len(d.history) }

func (d *MapChangesDraft) Modified() bool { return !equalMapChanges(d.rows, d.original.rows) }

func equalMapChanges(a, b []mapChangeRecord) bool {
	return slices.EqualFunc(a, b, func(x, y mapChangeRecord) bool {
		return x.header == y.header && bytes.Equal(x.tiles, y.tiles)
	})
}

func (d *MapChangesDraft) record(rows []mapChangeRecord) {
	if equalMapChanges(d.rows, rows) {
		return
	}
	d.history = append(
		d.history[:d.cursor],
		mapChangesHistory{before: d.rows, after: rows},
	)
	d.cursor++
	d.rows = rows
}

func (d *MapChangesDraft) Undo() bool {
	if !d.CanUndo() {
		return false
	}
	d.cursor--
	d.rows = d.history[d.cursor].before
	return true
}

func (d *MapChangesDraft) Redo() bool {
	if !d.CanRedo() {
		return false
	}
	d.rows = d.history[d.cursor].after
	d.cursor++
	return true
}

func (d *MapChangesDraft) validID(id, except int) bool {
	if id < 0 || id > 127 {
		return false
	}
	for index, row := range d.rows {
		if index != except && int(row.header[0]) == id {
			return false
		}
	}
	return true
}

func (d *MapChangesDraft) validBounds(bounds image.Rectangle) bool {
	return !bounds.Empty() && bounds.In(image.Rectangle{Max: d.Size()})
}

func (d *MapChangesDraft) FreeID() (int, error) {
	for id := range 128 {
		if d.validID(id, -1) {
			return id, nil
		}
	}
	return 0, ErrValue
}

func (d *MapChangesDraft) Add(id int, bounds image.Rectangle) error {
	if len(d.rows) >= maxMapChanges || !d.validID(id, -1) ||
		!d.validBounds(bounds) {
		return ErrValue
	}
	row := mapChangeRecord{tiles: make([]byte, bounds.Dx()*bounds.Dy()*2)}
	setMapChangeHeader(&row, id, bounds)
	d.record(append(slices.Clone(d.rows), row))
	return nil
}

func (d *MapChangesDraft) Copy(index, id int) error {
	if index < 0 || index >= len(d.rows) || len(d.rows) >= maxMapChanges ||
		!d.validID(id, -1) {
		return ErrValue
	}
	row := d.rows[index]
	row.address = 0
	row.header[0] = byte(
		id,
	) // #nosec G115 -- validID bounds native IDs to 0–127.
	d.record(append(slices.Clone(d.rows), row))
	return nil
}

func (d *MapChangesDraft) Delete(index int) error {
	if index < 0 || index >= len(d.rows) {
		return ErrValue
	}
	d.record(slices.Delete(slices.Clone(d.rows), index, index+1))
	return nil
}

// Set moves the rectangle without moving its tiles. Size changes retain the
// top-left overlap and fill new cells with zero (keep the base map).
func (d *MapChangesDraft) Set(index, id int, bounds image.Rectangle) error {
	if index < 0 || index >= len(d.rows) || !d.validBounds(bounds) {
		return ErrValue
	}
	old := d.rows[index]
	if id != int(old.header[0]) && !d.validID(id, index) {
		return ErrValue
	}
	rows := slices.Clone(d.rows)
	row := &rows[index]
	if bounds.Size() != old.info().Bounds.Size() {
		row.tiles = make([]byte, bounds.Dx()*bounds.Dy()*2)
		w, h := min(
			bounds.Dx(),
			int(old.header[3]),
		), min(
			bounds.Dy(),
			int(old.header[4]),
		)
		for y := range h {
			copy(
				row.tiles[y*bounds.Dx()*2:][:w*2],
				old.tiles[y*int(old.header[3])*2:][:w*2],
			)
		}
	}
	setMapChangeHeader(row, id, bounds)
	d.record(rows)
	return nil
}

func setMapChangeHeader(row *mapChangeRecord, id int, bounds image.Rectangle) {
	// Values are bounded by validID and the native base-map dimensions.
	row.header[0], row.header[1], row.header[2] = byte(
		id,
	), byte(
		bounds.Min.X,
	), byte(
		bounds.Min.Y,
	) // #nosec G115
	row.header[3], row.header[4] = byte(
		bounds.Dx(),
	), byte(
		bounds.Dy(),
	) // #nosec G115
}

func (d *MapChangesDraft) Cell(index, x, y int) (uint16, bool) {
	if index < 0 || index >= len(d.rows) {
		return 0, false
	}
	row := d.rows[index]
	bounds := row.info().Bounds
	if !image.Pt(x, y).In(bounds) {
		return 0, false
	}
	return u16(row.tiles, ((y-bounds.Min.Y)*bounds.Dx()+x-bounds.Min.X)*2), true
}

// Preview composes one selected record over the base, clipping only the view.
// Write planning separately rejects edited rectangles outside shared maps.
func (d *MapChangesDraft) Preview(index int, applied bool) (MapPreview, error) {
	content := bytes.Clone(d.base.content)
	if applied && index >= 0 && index < len(d.rows) {
		row := d.rows[index]
		bounds := row.info().Bounds.Intersect(image.Rectangle{Max: d.Size()})
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				tile, _ := d.Cell(index, x, y)
				if tile != 0 {
					binary.LittleEndian.PutUint16(
						content[2+(y*d.base.Width()+x)*2:],
						tile,
					)
				}
			}
		}
	}
	return d.base.resources.preview(content)
}

func (d *MapChangesDraft) Stroke(
	index int,
	samples []image.Point,
	tile uint16,
) error {
	if len(samples) == 0 {
		return ErrValue
	}
	for _, point := range samples {
		if _, ok := d.Cell(index, point.X, point.Y); !ok {
			return ErrValue
		}
	}
	return d.paint(index, mapStrokePoints(samples), tile)
}

func (d *MapChangesDraft) Fill(
	index int,
	bounds image.Rectangle,
	tile uint16,
) error {
	if index < 0 || index >= len(d.rows) || bounds.Empty() ||
		!bounds.In(d.rows[index].info().Bounds) {
		return ErrValue
	}
	points := make([]image.Point, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			points = append(points, image.Pt(x, y))
		}
	}
	return d.paint(index, points, tile)
}

func (d *MapChangesDraft) paint(
	index int,
	points []image.Point,
	tile uint16,
) error {
	if tile != 0 {
		if _, _, ok := d.base.resources.tile(tile); !ok {
			return ErrValue
		}
	}
	rows := slices.Clone(d.rows)
	row := &rows[index]
	row.tiles = bytes.Clone(row.tiles)
	bounds := row.info().Bounds
	for _, point := range points {
		at := ((point.Y-bounds.Min.Y)*bounds.Dx() + point.X - bounds.Min.X) * 2
		binary.LittleEndian.PutUint16(row.tiles[at:], tile)
	}
	d.record(rows)
	return nil
}

// snapshot freezes row slices before a plan or reference inspection uses them.
func (d *MapChangesDraft) snapshot() *MapChangesDraft {
	result := *d
	result.rows = slices.Clone(d.rows)
	for index := range result.rows {
		result.rows[index].tiles = bytes.Clone(result.rows[index].tiles)
	}
	base := *d.base
	base.layouts = maps.Clone(base.layouts)
	result.base = &base
	return &result
}
