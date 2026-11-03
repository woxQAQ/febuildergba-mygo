package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func supportUnits(data []byte, table records.Table) []support.Unit {
	var result []support.Unit
	if table.Err != nil || table.Stride < 48 {
		return result
	}
	var affinity records.Field
	for _, field := range table.Fields {
		if field.ID == "affinity" {
			affinity = field
			break
		}
	}
	for _, row := range table.Rows {
		if row.ID > 0 && row.ID <= 255 {
			result = append(
				result,
				support.Unit{
					ID: row.ID,
					Affinity: uint8( // #nosec G115 -- Native affinity field is one byte.
						affinity.Value(data, row.Address),
					),
					Pointer: row.Address + 44,
					Name:    row.Name,
					Host:    recordOwner(records.Units, row.ID),
				},
			)
		}
	}
	return result
}

func (s *Session) supportUnits(data []byte) ([]support.Unit, error) {
	for _, table := range s.Records.Inspect(data).Tables {
		if table.Kind != records.Units {
			continue
		}
		if table.Err != nil {
			return nil, table.Err
		}
		if table.Stride < 48 {
			return nil, edit.ErrLayout
		}
		return supportUnits(data, table), nil
	}
	return nil, edit.ErrLayout
}

func (s *Session) supportSource() support.Source {
	return support.Source{
		Units:        s.supportUnits,
		ValidateText: func(data []byte, id uint16) error { _, err := s.format.text.Read(data, id); return err },
		ProtectPointer: func(data []byte, owner space.Owner, site int) error {
			return s.guardPointer(data, owner, site, edit.ErrLayout)
		},
	}
}
