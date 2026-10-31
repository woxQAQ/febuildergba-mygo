package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (g format) nativeRanges(data []byte) []space.Range {
	return append(
		append(g.shopRanges(data), g.quotes.Ranges(data)...),
		g.supportRanges(data)...)
}

func (s *Session) quoteSource() quote.Source {
	return quote.Source{
		ProtectPointer: s.protectQuotePointer,
		ValidateText:   func(data []byte, id uint16) error { _, err := s.format.text.Read(data, id); return err },
	}
}

func (s *Session) protectQuotePointer(data []byte, site int) error {
	if err := s.protectResourcePointer(data, site); err != nil {
		return err
	}
	for _, region := range append(s.format.shopRanges(data), s.supportRanges(data)...) {
		if site < region.End && region.Start < site+4 {
			return edit.ErrLayout
		}
	}
	return nil
}
