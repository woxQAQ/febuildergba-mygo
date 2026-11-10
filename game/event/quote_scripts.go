package event

import "github.com/woxQAQ/febuildergba-mygo/game/edit"

// QuoteScripts selects native battle and defeat script roots independently of
// chapter and world-map contexts. These entries support browsing only.
const QuoteScripts = -2

func (s *Editor) QuoteScriptsAvailable() bool { return s.source.QuoteScripts != nil }

func (s *Editor) quoteScriptIndex(format Format) (scriptIndex, error) {
	if !s.QuoteScriptsAvailable() {
		return scriptIndex{}, edit.ErrField
	}
	entries, complete := s.source.QuoteScripts(s.image.Data)
	for i, entry := range entries {
		if entry.Err != nil {
			continue
		}
		parsed := scriptEntry(
			s.image.Data,
			QuoteScripts,
			entry.Pointer,
			entry.Source,
		)
		parsed.Record = entry.Record
		entries[i] = parsed
	}
	result := format.indexScripts(s.image.Data, entries)
	result.partial = result.partial || !complete
	return result, nil
}
