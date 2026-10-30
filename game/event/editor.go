// Package event interprets and edits native conditions and event scripts.
package event

import (
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// Source resolves chapter resources after the event interpreter has established
// their native operands and context.
type Source struct {
	ResolveMap     func(int, maps.Target) (maps.Change, error)
	ProtectPointer func([]byte, int) error
}

type Editor struct {
	context       *edit.Context
	format        Format
	text          text.Format
	records       *records.Editor
	deployments   *deployment.Editor
	source        Source
	image         rom.Snapshot
	binding       edit.Stamp
	scriptIndexes map[int]scriptIndex
}

func New(
	context *edit.Context,
	format Format,
	text text.Format,
	records *records.Editor,
	deployments *deployment.Editor,
	source Source,
) *Editor {
	return &Editor{
		context:     context,
		format:      format,
		text:        text,
		records:     records,
		deployments: deployments,
		source:      source,
	}
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.binding = s.context.Bind(s.image.Revision)
		s.scriptIndexes = nil
	}
	s.records.Refresh()
}
