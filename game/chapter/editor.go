// Package chapter coordinates chapter resources and their shared editing effects.
package chapter

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

// Config connects the native chapter table to its event, map and text formats.
// ProtectPointer is the session's claim check; when it is nil, only record
// tables are protected. ProtectRange is required for in-place script edits.
type Config struct {
	WorldMap       *worldmap.Editor
	Portraits      *portrait.Editor
	Table          Format
	Events         event.Format
	Maps           maps.Format
	Text           text.Format
	Shops          shop.Format
	ProtectPointer func(data []byte, owner romwrite.Owner, site int) error
	ProtectRange   func(data []byte, owner romwrite.Owner, region romwrite.Range) error
	InventoryUses  func([]byte, int) []shop.Reference
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
	image       edit.Input
	binding     edit.Stamp
}

func New(
	context *edit.Context,
	config Config,
	records *records.Editor,
) *Editor {
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
			WorldMap:      config.WorldMap,
			Portraits:     config.Portraits,
			DefaultShop:   config.Shops.Default,
			InventoryUses: config.InventoryUses,
			ResolveMap:    s.Maps.ResolveTarget,
			ProtectRange:  config.ProtectRange,
			ProtectPointer: func(data []byte, owner romwrite.Owner, site int) error {
				err := config.protect(data, records, owner, site)
				if err != nil {
					return fault.New(
						event.ErrEventConditions,
						"game.protect_event_pointer",
						fault.Context{
							Range: &fault.Range{
								Offset: site,
								Length: 4,
								Size:   len(data),
							},
						},
						err,
					)
				}
				return nil
			},
		},
	)
	return s
}

// protect runs the session claim check, falling back to record tables alone
// when the config has none.
func (g Config) protect(
	data []byte,
	recordsEditor *records.Editor,
	owner romwrite.Owner,
	site int,
) error {
	if g.ProtectPointer == nil {
		return checkResourcePointer(
			data,
			recordsEditor.Inspect(data).Tables,
			site,
		)
	}
	return g.ProtectPointer(data, owner, site)
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.image = s.context.Capture()
		s.binding = s.image.Stamp()
	}
	s.records.Refresh()
}
