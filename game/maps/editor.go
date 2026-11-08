// Package maps edits base maps, tilesets and map-change resources.
package maps

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

// Resize describes the dimensions and confirmed shared uses being reviewed.
// The chapter coordinator supplies impacts from deployments and event scripts.
type Resize struct {
	Before, After image.Point
	Uses          []Use
}

// Source supplies facts and checks owned by other chapter resources. It does
// not expose their private drafts or give maps access to a parent session.
type Source struct {
	Bindings   []Binding
	Protect    func([]byte, romwrite.Owner, int) error
	Resize     func(Resize) ([]Impact, []ImpactWarning)
	References func(*ChangesDraft, []Use) ChangeReferences
}

type Editor struct {
	context *edit.Context
	format  Format
	records *records.Editor
	source  Source
	image   edit.Input
	binding edit.Stamp
}

func New(
	context *edit.Context,
	format Format,
	records *records.Editor,
	source Source,
) *Editor {
	return &Editor{
		context: context,
		format:  format,
		records: records,
		source:  source,
	}
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.image = s.context.Capture()
		s.binding = s.image.Stamp()
	}
	s.records.Refresh()
}
