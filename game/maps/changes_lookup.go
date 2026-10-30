package maps

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// MapChangeReference compares the record selected before and after a table edit.
// Before/After are record indexes (-1 means absent), never event IDs.
type ChangeReference struct {
	Chapter, ScriptChapter, Pointer, Address, Instruction int
	ID, AfterID                                           int
	Position                                              image.Point
	ByPosition, Retargeted                                bool
	Before, After                                         int
}

type (
	ChangeWarning struct {
		Chapter, Address int
		Err              error
	}
	ChangeReferences struct {
		References []ChangeReference
		Warnings   []ChangeWarning
	}
)

type Target struct {
	ID         int
	Position   image.Point
	ByPosition bool
}

func (s *Editor) References(
	draft *ChangesDraft,
) (ChangeReferences, error) {
	if draft == nil || !draft.Current(s) {
		return ChangeReferences{}, rom.ErrStale
	}
	format := s.format
	uses, err := s.mapResourceUses(
		format,
		ChangesField,
		draft.original.pointer,
		draft.original.address,
	)
	if err != nil {
		return ChangeReferences{}, err
	}
	report := s.source.References(draft, uses)
	if !draft.Current(s) {
		return ChangeReferences{}, rom.ErrStale
	}
	return report, nil
}

func (d *ChangesDraft) CompareTarget(
	operand Target,
) ChangeReference {
	beforeID, afterID := operand.ID, operand.ID
	if operand.ByPosition {
		beforeID = mapChangeAt(d.original.rows, operand.Position)
		afterID = mapChangeAt(d.rows, operand.Position)
	}
	before, after := mapChangeIndex(
		d.original.rows,
		beforeID,
	), mapChangeIndex(
		d.rows,
		afterID,
	)
	return ChangeReference{
		ID:         beforeID,
		AfterID:    afterID,
		Position:   operand.Position,
		ByPosition: operand.ByPosition,
		Before:     before,
		After:      after,
		Retargeted: before >= 0 && after >= 0 &&
			d.original.rows[before].address != d.rows[after].address,
	}
}

// MapChangeLink is a confirmed event target in its current chapter context.
type Link struct{ Chapter, ID, Address int }

// MapGeometry retains readable native rectangles even when later records are
// damaged. Address and Err describe the point where discovery stopped.
type Geometry struct {
	Entries []Change
	Address int
	Err     error
}

func (f Format) ChangeGeometry(data []byte, id int64) Geometry {
	var result Geometry
	list, valid := gba.Pointer(data, f.Changes)
	if valid && gba.Within(data, list+int(id)*4, 4) &&
		gba.Uint32(data, list+int(id)*4) == 0 {
		return result
	}
	at, err := mapResource(
		data,
		f.Changes,
		byte(id),
		"map_changes_id",
	) // #nosec G115 -- Chapter resource IDs are bytes.
	if err != nil {
		result.Err = err
		return result
	}
	if at < 0xC0 || at%4 != 0 {
		result.Address, result.Err = at, ErrMap
		return result
	}
	for index := 0; index <= 256; index++ {
		if !gba.Within(data, at, 1) {
			break
		}
		if data[at] >= 0x80 {
			return result
		}
		if index == 256 || !gba.Within(data, at, 12) {
			break
		}
		row := data[at : at+12]
		x, y, w, h := int(row[1]), int(row[2]), int(row[3]), int(row[4])
		result.Entries = append(
			result.Entries,
			Change{
				ID:      int(row[0]),
				Address: at,
				Bounds:  image.Rect(x, y, x+w, y+h),
			},
		)
		at += 12
	}
	result.Address, result.Err = at, ErrMap
	return result
}
