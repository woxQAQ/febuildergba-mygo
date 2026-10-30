// Package chapter coordinates chapter resources and their shared editing effects.
package chapter

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Config connects the native chapter table to its event, map and text formats.
// Reserved contains confirmed external resources which chapter edits must protect.
type Config struct {
	Table    Format
	Events   event.Format
	Maps     maps.Format
	Text     text.Format
	Reserved []space.Range
}

// Editor coordinates shared chapter resources. The native editors retain their
// own encodings, drafts and immutable plans, all bound to one editing context.
type Editor struct {
	Events      *event.Editor
	Maps        *maps.Editor
	Deployments *deployment.Editor
	context     *edit.Context
	format      Config
	records     *records.Editor
	image       rom.Snapshot
	binding     edit.Stamp
}

func New(
	context *edit.Context,
	config Config,
	records *records.Editor,
) *Editor {
	config.Reserved = slices.Clone(config.Reserved)
	s := &Editor{context: context, format: config, records: records}
	s.Deployments = deployment.New(context, records, s.deploymentSource())
	s.Maps = maps.New(context, config.Maps, records, s.mapSource())
	s.Events = event.New(
		context,
		config.Events,
		config.Text,
		records,
		s.Deployments,
		event.Source{
			ResolveMap: s.Maps.ResolveTarget,
			ProtectPointer: func(data []byte, site int) error {
				return checkResourcePointer(
					data,
					records.Inspect(data).Tables,
					site,
				)
			},
		},
	)
	return s
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.binding = s.context.Bind(s.image.Revision)
	}
	s.records.Refresh()
}
