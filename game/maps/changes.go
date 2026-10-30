package maps

import (
	"bytes"
	"encoding/binary"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

const (
	ChangesField  = "map_changes_id"
	maxMapChanges = 256
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
type Change struct {
	ID, Address int
	Bounds      image.Rectangle
}

type mapChangeRecord struct {
	header               [12]byte
	address              int
	tiles, originalTiles []byte
}

func (r mapChangeRecord) info() Change {
	x, y := int(r.header[1]), int(r.header[2])
	return Change{
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
	list, ok := gba.Pointer(data, root)
	if !ok || list < 0xC0 || list%4 != 0 || id < 0 || id > 255 ||
		!gba.Within(data, list+int(id)*4, 4) {
		return mapChangeTable{}, ErrMapChanges
	}
	result := mapChangeTable{pointer: list + int(id)*4}
	if gba.Uint32(data, result.pointer) == 0 {
		result.terminator = make([]byte, 12)
		result.terminator[0] = 0xFF
		return result, nil
	}
	result.address, ok = gba.Pointer(data, result.pointer)
	if !ok || result.address < 0xC0 || result.address%4 != 0 {
		return mapChangeTable{}, ErrMapChanges
	}
	for index := 0; index <= maxMapChanges; index++ {
		at := result.address + index*12
		if !gba.Within(data, at, 1) {
			return mapChangeTable{}, ErrMapChanges
		}
		if data[at] >= 0x80 {
			result.terminator = bytes.Clone(data[at:min(at+12, len(data))])
			return result, nil
		}
		if index == maxMapChanges || !gba.Within(data, at, 12) {
			return mapChangeTable{}, ErrMapChanges
		}
		row := mapChangeRecord{address: at}
		copy(row.header[:], data[at:at+12])
		area := int(row.header[3]) * int(row.header[4])
		if area > MaxMapCells {
			return mapChangeTable{}, ErrMapChanges
		}
		if area != 0 {
			payload, valid := gba.Pointer(data, at+8)
			if !valid || payload < 0xC0 || payload%2 != 0 ||
				!gba.Within(data, payload, area*2) {
				return mapChangeTable{}, ErrMapChanges
			}
			row.tiles = bytes.Clone(data[payload : payload+area*2])
			row.originalTiles = row.tiles
		}
		result.rows = append(result.rows, row)
	}
	return mapChangeTable{}, ErrMapChanges
}

func (s *Editor) mapChanges(chapter int) (mapChangeTable, error) {
	format := s.format
	if s.context.Validate(s.image.Data) != nil {
		return mapChangeTable{}, edit.ErrVersion
	}
	id, err := s.records.Value(records.Chapters, chapter, ChangesField)
	if err != nil {
		return mapChangeTable{}, err
	}
	return readMapChanges(s.image.Data, format.Changes, id)
}

func (s *Editor) Changes(chapter int) ([]Change, error) {
	s.Refresh()
	table, err := s.mapChanges(chapter)
	if err != nil {
		return nil, err
	}
	return mapChangeInfos(table.rows), nil
}

func mapChangeInfos(rows []mapChangeRecord) []Change {
	result := make([]Change, len(rows))
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
type ChangesDraft struct {
	base     *Draft
	original mapChangeTable
	rows     []mapChangeRecord
	history  []mapChangesHistory
	cursor   int
}

func (s *Editor) NewChangesDraft(chapter int) (*ChangesDraft, error) {
	base, err := s.NewDraft(chapter)
	if err != nil {
		return nil, err
	}
	table, err := s.mapChanges(chapter)
	if err != nil {
		return nil, err
	}
	return &ChangesDraft{
		base:     base,
		original: table,
		rows:     slices.Clone(table.rows),
	}, nil
}

func (d *ChangesDraft) Current(
	s *Editor,
) bool {
	return d.base.Current(s)
}

func (d *ChangesDraft) Chapter() int { return d.base.Chapter() }

func (d *ChangesDraft) Entries() []Change { return mapChangeInfos(d.rows) }

func (d *ChangesDraft) Tileset() (Preview, error) { return d.base.Tileset() }

func (d *ChangesDraft) Size() image.Point { return image.Pt(d.base.Width(), d.base.Height()) }

func (d *ChangesDraft) CanUndo() bool { return d.cursor > 0 }

func (d *ChangesDraft) CanRedo() bool { return d.cursor < len(d.history) }

func (d *ChangesDraft) Modified() bool { return !equalMapChanges(d.rows, d.original.rows) }

func equalMapChanges(a, b []mapChangeRecord) bool {
	return slices.EqualFunc(a, b, func(x, y mapChangeRecord) bool {
		return x.header == y.header && bytes.Equal(x.tiles, y.tiles)
	})
}

func (d *ChangesDraft) record(rows []mapChangeRecord) {
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

func (d *ChangesDraft) Undo() bool {
	if !d.CanUndo() {
		return false
	}
	d.cursor--
	d.rows = d.history[d.cursor].before
	return true
}

func (d *ChangesDraft) Redo() bool {
	if !d.CanRedo() {
		return false
	}
	d.rows = d.history[d.cursor].after
	d.cursor++
	return true
}

func (d *ChangesDraft) validID(id, except int) bool {
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

func (d *ChangesDraft) validBounds(bounds image.Rectangle) bool {
	return !bounds.Empty() && bounds.In(image.Rectangle{Max: d.Size()})
}

func (d *ChangesDraft) FreeID() (int, error) {
	for id := range 128 {
		if d.validID(id, -1) {
			return id, nil
		}
	}
	return 0, edit.ErrValue
}

func (d *ChangesDraft) Add(id int, bounds image.Rectangle) error {
	if len(d.rows) >= maxMapChanges || !d.validID(id, -1) ||
		!d.validBounds(bounds) {
		return edit.ErrValue
	}
	row := mapChangeRecord{tiles: make([]byte, bounds.Dx()*bounds.Dy()*2)}
	setMapChangeHeader(&row, id, bounds)
	d.record(append(slices.Clone(d.rows), row))
	return nil
}

func (d *ChangesDraft) Copy(index, id int) error {
	if index < 0 || index >= len(d.rows) || len(d.rows) >= maxMapChanges ||
		!d.validID(id, -1) {
		return edit.ErrValue
	}
	row := d.rows[index]
	row.address = 0
	row.header[0] = byte(
		id,
	) // #nosec G115 -- validID bounds native IDs to 0–127.
	d.record(append(slices.Clone(d.rows), row))
	return nil
}

func (d *ChangesDraft) Delete(index int) error {
	if index < 0 || index >= len(d.rows) {
		return edit.ErrValue
	}
	d.record(slices.Delete(slices.Clone(d.rows), index, index+1))
	return nil
}

// Set moves the rectangle without moving its tiles. Size changes retain the
// top-left overlap and fill new cells with zero (keep the base map).
func (d *ChangesDraft) Set(index, id int, bounds image.Rectangle) error {
	if index < 0 || index >= len(d.rows) || !d.validBounds(bounds) {
		return edit.ErrValue
	}
	old := d.rows[index]
	if id != int(old.header[0]) && !d.validID(id, index) {
		return edit.ErrValue
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

func (d *ChangesDraft) Cell(index, x, y int) (uint16, bool) {
	if index < 0 || index >= len(d.rows) {
		return 0, false
	}
	row := d.rows[index]
	bounds := row.info().Bounds
	if !image.Pt(x, y).In(bounds) {
		return 0, false
	}
	return gba.Uint16(
		row.tiles,
		((y-bounds.Min.Y)*bounds.Dx()+x-bounds.Min.X)*2,
	), true
}

// Preview composes one selected record over the base, clipping only the view.
// Write planning separately rejects edited rectangles outside shared maps.
func (d *ChangesDraft) Preview(index int, applied bool) (Preview, error) {
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

func (d *ChangesDraft) Stroke(
	index int,
	samples []image.Point,
	tile uint16,
) error {
	if len(samples) == 0 {
		return edit.ErrValue
	}
	for _, point := range samples {
		if _, ok := d.Cell(index, point.X, point.Y); !ok {
			return edit.ErrValue
		}
	}
	return d.paint(index, mapStrokePoints(samples), tile)
}

func (d *ChangesDraft) Fill(
	index int,
	bounds image.Rectangle,
	tile uint16,
) error {
	if index < 0 || index >= len(d.rows) || bounds.Empty() ||
		!bounds.In(d.rows[index].info().Bounds) {
		return edit.ErrValue
	}
	points := make([]image.Point, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			points = append(points, image.Pt(x, y))
		}
	}
	return d.paint(index, points, tile)
}

func (d *ChangesDraft) paint(
	index int,
	points []image.Point,
	tile uint16,
) error {
	if tile != 0 {
		if _, _, ok := d.base.resources.tile(tile); !ok {
			return edit.ErrValue
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
func (d *ChangesDraft) snapshot() *ChangesDraft {
	result := *d
	result.rows = slices.Clone(d.rows)
	for index := range result.rows {
		result.rows[index].tiles = bytes.Clone(result.rows[index].tiles)
	}
	base := *d.base
	result.base = &base
	return &result
}

// resolveMapTarget applies native ID and rectangle selection to an existing table.
func (s *Editor) ResolveTarget(chapter int, target Target) (Change, error) {
	s.Refresh()
	table, err := s.mapChanges(chapter)
	if err != nil {
		return Change{}, err
	}
	id := target.ID
	if target.ByPosition {
		id = mapChangeAt(table.rows, target.Position)
	}
	index := mapChangeIndex(table.rows, id)
	if index < 0 {
		return Change{}, ErrMapChanges
	}
	return table.rows[index].info(), nil
}
