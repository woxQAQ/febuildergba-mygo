package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
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
			Host:    recordOwner(records.Classes, row.ID),
		}
	}
	return result
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

func (s *Session) terrainSource() terrain.Source {
	return terrain.Source{
		Classes: s.terrainClasses,
		ProtectPointer: func(data []byte, owner romwrite.Owner, site int) error {
			return s.guardPointer(data, owner, site, edit.ErrLayout)
		},
	}
}
