// Package deployment edits native unit records and FE8 REDA movement paths.
package deployment

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

// Source discovers unit references through chapter headers and condition/event
// graphs. Unit records and native movement payloads remain owned here.
type Source struct {
	Candidates func([]byte, int) ([]Group, error)
	Uses       func([]byte, int) []Use
	// ProtectPointer guards one chapter's group pointer site against
	// overwriting any claimed resource.
	ProtectPointer func(data []byte, owner romwrite.Owner, site int) error
}

// Owner names the claim identity of one deployment group pointer site.
// Chapters aliasing one group declare identical claims instead of conflicting.
func Owner(site int) romwrite.Owner {
	return romwrite.Owner{Resource: "deployment", ID: site}
}

type Editor struct {
	context *edit.Context
	records *records.Editor
	source  Source
	image   edit.Input
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
		s.image = s.context.Capture()
		s.binding = s.image.Stamp()
	}
	s.records.Refresh()
}
