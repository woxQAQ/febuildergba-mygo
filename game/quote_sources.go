package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func (s *Session) quoteScriptEntries(data []byte) ([]event.ScriptEntry, bool) {
	references, complete := s.format.quotes.ScriptReferences(data)
	entries := make([]event.ScriptEntry, len(references))
	for i, reference := range references {
		entries[i] = event.ScriptEntry{
			Chapter: event.QuoteScripts,
			Pointer: reference.Pointer,
			Source: string(
				reference.Kind,
			),
			Record: reference.Record,
			Err:    reference.Err,
		}
	}
	return entries, complete
}

func (s *Session) quoteSource() quote.Source {
	return quote.Source{
		ProtectPointer: func(data []byte, owner romwrite.Owner, site int) error {
			return s.guardPointer(data, owner, site, edit.ErrLayout)
		},
		ValidateText: func(data []byte, id uint16) error {
			_, err := s.format.text.Read(data, id)
			return err
		},
	}
}
