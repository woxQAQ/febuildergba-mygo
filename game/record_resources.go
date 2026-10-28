package game

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

func (s *Session) RecordResources(kind Kind, id int) ([]RecordResource, error) {
	s.Refresh()
	if kind != Units && kind != Classes {
		return nil, ErrField
	}
	table, row, _, err := s.locate(kind, id, "id")
	if err != nil {
		return nil, err
	}
	specs := []recordResourceSpec{{"record_support", 44, 4}}
	if kind == Classes {
		specs = []recordResourceSpec{
			{"record_battle_animation", 52, 4},
			{"record_move_normal", 56, 0x41},
			{"record_move_rain", 60, 0x41},
			{"record_move_snow", 64, 0x41},
			{"record_terrain_avoid", 68, 0x41},
			{"record_terrain_defense", 72, 0x41},
			{"record_terrain_resistance", 76, 0x41},
		}
		if s.Adapter.ID() == fe6ID {
			specs = []recordResourceSpec{
				{"record_battle_animation", 48, 4},
				{"record_move_normal", 52, 4},
				{"record_terrain_avoid", 56, 4},
				{"record_terrain_defense", 60, 4},
				{"record_terrain_resistance", 64, 4},
			}
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
			!within(s.image.Data, resource.Pointer, 4) {
			resource.Err = ErrLayout
			result = append(result, resource)
			continue
		}
		value := u32(s.image.Data, resource.Pointer)
		if value == 0 {
			resource.Empty = true
			result = append(result, resource)
			continue
		}
		target, valid := pointer(s.image.Data, resource.Pointer)
		if !valid || target < 0xC0 ||
			!within(s.image.Data, target, resource.Length) {
			resource.Err = ErrValue
		} else {
			resource.Address = target
		}
		result = append(result, resource)
	}
	return result, nil
}
