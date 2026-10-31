package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (s *Session) quoteSource() quote.Source {
	return quote.Source{
		ProtectPointer: func(data []byte, owner space.Owner, site int) error {
			return s.guardPointer(data, owner, site, edit.ErrLayout)
		},
		ValidateText: func(data []byte, id uint16) error {
			_, err := s.format.text.Read(data, id)
			return err
		},
	}
}
