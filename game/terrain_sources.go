package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func terrainClasses(table records.Table) []terrain.Class {
	if table.Err != nil {
		return nil
	}
	result := make([]terrain.Class, len(table.Rows))
	for i, row := range table.Rows {
		result[i] = terrain.Class{
			ID:      row.ID,
			Name:    row.Name,
			Address: row.Address,
			Length:  table.Stride,
		}
	}
	return result
}

func (g format) terrainRanges(data []byte) []space.Range {
	for _, rule := range g.tables {
		if rule.Kind != records.Classes {
			continue
		}
		table := rule.Read(
			data,
			records.Layout{},
			false,
			func(uint16) (string, error) { return "", nil },
		)
		return terrain.ForVersion(g.id).Ranges(data, terrainClasses(table))
	}
	return nil
}

func (s *Session) terrainClasses(data []byte) ([]terrain.Class, error) {
	for _, table := range s.Records.Inspect(data).Tables {
		if table.Kind != records.Classes {
			continue
		}
		if table.Err != nil {
			return nil, table.Err
		}
		return terrainClasses(table), nil
	}
	return nil, edit.ErrLayout
}

func (s *Session) terrainRanges(data []byte) []space.Range {
	classes, _ := s.terrainClasses(data)
	return terrain.ForVersion(s.format.id).Ranges(data, classes)
}

func (s *Session) terrainSource() terrain.Source {
	return terrain.Source{
		Classes: s.terrainClasses,
		ProtectPointer: func(data []byte, ref terrain.Reference) error {
			if err := s.protectRecordPointer(
				data,
				ref.Pointer,
				records.Classes,
				ref.Class.ID,
				ref.Pointer-ref.Class.Address,
			); err != nil {
				return err
			}
			for _, region := range append(append(s.format.shopRanges(data), s.format.quotes.Ranges(data)...), s.supportRanges(data)...) {
				if ref.Pointer < region.End && region.Start < ref.Pointer+4 {
					return edit.ErrLayout
				}
			}
			return nil
		},
	}
}
