package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func supportUnits(table records.Table) []support.Unit {
	var result []support.Unit
	if table.Err != nil || table.Stride < 48 {
		return result
	}
	for _, row := range table.Rows {
		if row.ID > 0 && row.ID <= 255 {
			result = append(
				result,
				support.Unit{
					ID:      row.ID,
					Pointer: row.Address + 44,
					Name:    row.Name,
				},
			)
		}
	}
	return result
}

func (g format) supportRanges(data []byte) []space.Range {
	var units []support.Unit
	for _, rule := range g.tables {
		if rule.Kind == records.Units {
			units = supportUnits(
				rule.Read(
					data,
					records.Layout{},
					false,
					func(uint16) (string, error) { return "", nil },
				),
			)
			break
		}
	}
	return g.supports.Ranges(data, units)
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
		return supportUnits(table), nil
	}
	return nil, edit.ErrLayout
}

func (s *Session) supportRanges(data []byte) []space.Range {
	units, _ := s.supportUnits(data)
	return s.format.supports.Ranges(data, units)
}

func (s *Session) supportSource() support.Source {
	return support.Source{
		Units:        s.supportUnits,
		ValidateText: func(data []byte, id uint16) error { _, err := s.format.text.Read(data, id); return err },
		ProtectPointer: func(data []byte, site, owner int) error {
			if err := s.protectCharacterPointer(data, site, owner); err != nil {
				return err
			}
			for _, region := range append(s.format.shopRanges(data), s.format.quotes.Ranges(data)...) {
				if site < region.End && region.Start < site+4 {
					return edit.ErrLayout
				}
			}
			return nil
		},
	}
}
