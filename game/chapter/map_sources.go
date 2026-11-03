package chapter

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (s *Editor) mapSource() maps.Source {
	g := s.format
	return maps.Source{
		Bindings: []maps.Binding{
			{
				Field: event.WorldMapEventField,
				Root:  g.Events.WorldMapEvents.ChapterPLIST,
			},
			{
				Field:    event.HeaderField,
				Root:     g.Events.Root,
				Length:   g.Events.HeaderSize(),
				Optional: true,
			},
		},
		Protect: s.mapProtection,
		Resize: func(request maps.Resize) ([]maps.Impact, []maps.ImpactWarning) {
			s.Refresh()
			report := s.mapResizeImpacts(g, request)
			return report.items, report.warnings
		},
		References: s.mapChangeReferences,
	}
}

// mapProtection authorizes the selected map resource under the session claims.
func (s *Editor) mapProtection(data []byte, owner space.Owner, site int) error {
	if err := s.format.protect(
		data,
		s.records,
		owner,
		site,
	); err != nil {
		return fault.New(
			edit.ErrLayout,
			"game.map_pointer_write",
			fault.Context{},
			err,
		)
	}
	return nil
}
