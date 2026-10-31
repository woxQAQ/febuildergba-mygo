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
				Field:    event.HeaderField,
				Root:     g.Events.Root(),
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

// mapProtection guards a map PLIST entry with every claimed resource. No map
// claim declares the site, so any overlap conflicts; a map's own PLIST
// siblings are checked by the maps package itself.
func (s *Editor) mapProtection(data []byte, site int) error {
	if err := s.format.protect(
		data,
		s.records,
		space.Owner{Resource: "maps"},
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
