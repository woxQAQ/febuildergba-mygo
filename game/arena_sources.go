package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/arena"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (s *Session) arenaTable(
	data []byte,
	kind records.Kind,
) (records.Table, error) {
	for _, table := range s.Records.Inspect(data).Tables {
		if table.Kind == kind {
			return table, table.Err
		}
	}
	return records.Table{}, edit.ErrLayout
}

func (s *Session) arenaIDs(data []byte, kind records.Kind) ([]int, error) {
	table, err := s.arenaTable(data, kind)
	if err != nil {
		return nil, err
	}
	var ids []int
	for _, row := range table.Rows {
		if row.ID > 0 && row.ID <= 255 {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}

func (s *Session) arenaSource() arena.Source {
	return arena.Source{
		Classes: func(data []byte) ([]arena.Class, error) {
			table, err := s.arenaTable(data, records.Classes)
			if err != nil {
				return nil, err
			}
			var flags *records.Field
			for _, field := range table.Fields {
				if field.ID == "flags" {
					flags = new(field)
					break
				}
			}
			if flags == nil {
				return nil, edit.ErrLayout
			}
			var classes []arena.Class
			for _, row := range table.Rows {
				if row.ID > 0 && row.ID <= 255 {
					classes = append(
						classes,
						arena.Class{
							ID:       row.ID,
							Promoted: flags.Value(data, row.Address)&0x100 != 0,
						},
					)
				}
			}
			return classes, nil
		},
		Items: func(data []byte) ([]int, error) { return s.arenaIDs(data, records.Items) },
		Units: func(data []byte) ([]int, error) { return s.arenaIDs(data, records.Units) },
		ProtectPointer: func(data []byte, owner space.Owner, site int) error {
			return s.guardPointer(data, owner, site, arena.ErrReference)
		},
	}
}
