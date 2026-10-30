package chapter

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
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

func (s *Editor) mapProtection(data []byte, site int) error {
	if err := checkResourcePointer(
		data,
		s.records.Inspect(data).Tables,
		site,
	); err != nil {
		return fault.New(
			edit.ErrLayout,
			"game.map_pointer_write",
			fault.Context{},
			err,
		)
	}
	g := s.format
	overlap := func(at, length int) bool { return at >= 0xC0 && length > 0 && site < at+length && at < site+4 }
	for _, at := range []int{g.Text.Pointer, g.Text.RootPointer, g.Text.TreePointer} {
		if overlap(at, 4) {
			return edit.ErrLayout
		}
	}
	for _, region := range g.Reserved {
		if overlap(region.Start, region.End-region.Start) {
			return edit.ErrLayout
		}
	}
	if table, err := g.Text.Table(
		data,
	); err == nil &&
		overlap(table.Address, table.Count*4) {
		return edit.ErrLayout
	}
	for _, region := range g.Ranges(data) {
		if overlap(region.Start, region.End-region.Start) {
			return edit.ErrLayout
		}
	}
	return nil
}
