// Package deployment edits native unit records and FE8 REDA movement paths.
package deployment

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Source discovers unit references through chapter headers and condition/event
// graphs. Unit records and native movement payloads remain owned here.
type Source struct {
	Candidates func([]byte, int) ([]Group, error)
	Uses       func([]byte, int) []Use
	// ProtectPointer guards one chapter's group pointer site against
	// overwriting any claimed resource.
	ProtectPointer func(data []byte, owner space.Owner, site int) error
}

// Owner names the claim identity of one deployment group pointer site.
// Chapters aliasing one group declare identical claims instead of conflicting.
func Owner(site int) space.Owner {
	return space.Owner{Resource: "deployment", ID: site}
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
