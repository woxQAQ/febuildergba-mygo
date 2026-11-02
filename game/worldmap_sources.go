package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
)

func (s *Session) worldmapSource() worldmap.Source {
	return worldmap.Source{
		ValidateChapter: func(data []byte, id int) error {
			for _, table := range s.Records.Inspect(data).Tables {
				if table.Kind != records.Chapters {
					continue
				}
				if table.Err != nil {
					return table.Err
				}
				for _, row := range table.Rows {
					if row.ID == id {
						return nil
					}
				}
				return edit.ErrValue
			}
			return edit.ErrLayout
		},
		ValidateText: func(data []byte, id uint16) error {
			_, err := s.format.text.Read(data, id)
			return err
		},
		ProtectRange: s.protectRange,
	}
}
