// Package deployment edits native unit records and FE8 REDA movement paths.
package deployment

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// Source discovers unit references through chapter headers and condition/event
// graphs. Unit records and native movement payloads remain owned here.
type Source struct {
	Candidates func([]byte, int) ([]Group, error)
	Uses       func([]byte, int) []Use
}

type Editor struct {
	context *edit.Context
	records *records.Editor
	source  Source
	image   rom.Snapshot
	binding edit.Stamp
}

func New(
	context *edit.Context,
	records *records.Editor,
	source Source,
) *Editor {
	return &Editor{context: context, records: records, source: source}
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.binding = s.context.Bind(s.image.Revision)
	}
	s.records.Refresh()
}
