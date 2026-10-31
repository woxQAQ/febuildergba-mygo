package records

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// RecordResource is a confirmed pointer operand within a character or class
// record. Empty and damaged resources stay distinct, and only readable targets
// expose a byte-view navigation range.
type RecordResource struct {
	Label                    string
	Pointer, Address, Length int
	Empty                    bool
	Err                      error
}

type recordResourceSpec struct {
	label          string
	offset, length int
}

func (s *Editor) RecordResources(kind Kind, id int) ([]RecordResource, error) {
	s.Refresh()
	if kind != Units && kind != Classes {
		return nil, edit.ErrField
	}
	table, row, _, err := s.Resolve(kind, id, "id")
	if err != nil {
		return nil, err
	}
	specs := []recordResourceSpec{{"record_support", 44, 4}}
	if kind == Classes {
		animation := 52
		if s.definition.Version == version.FE6 {
			animation = 48
		}
		specs = []recordResourceSpec{{"record_battle_animation", animation, 4}}
		format := terrain.ForVersion(s.definition.Version)
		for _, column := range format.Columns() {
			specs = append(
				specs,
				recordResourceSpec{
					"record_" + string(column.Kind),
					column.Offset,
					format.Count(),
				},
			)
		}
	}
	var result []RecordResource
	for _, spec := range specs {
		resource := RecordResource{
			Label:   spec.label,
			Pointer: row.Address + spec.offset,
			Length:  spec.length,
		}
		if spec.offset+4 > table.Stride ||
			!gba.Within(s.image.Data, resource.Pointer, 4) {
			resource.Err = edit.ErrLayout
			result = append(result, resource)
			continue
		}
		value := gba.Uint32(s.image.Data, resource.Pointer)
		if value == 0 {
			resource.Empty = true
			result = append(result, resource)
			continue
		}
		target, valid := gba.Pointer(s.image.Data, resource.Pointer)
		if !valid || target < 0xC0 ||
			!gba.Within(s.image.Data, target, resource.Length) {
			resource.Err = edit.ErrValue
		} else {
			resource.Address = target
		}
		result = append(result, resource)
	}
	return result, nil
}
