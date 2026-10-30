package chapter

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

const maxMapImpacts = 4096

var ErrMapImpactLimit = fault.Code{
	ID:   "game.map_impact_limit",
	Kind: fault.Capacity,
}

type mapImpactKey struct {
	chapter, address, step int
	kind                   maps.ImpactKind
}

type mapImpactReport struct {
	before, after image.Rectangle
	items         []maps.Impact
	warnings      []maps.ImpactWarning
	seen          map[mapImpactKey]bool
	remaining     int
}

func (r *mapImpactReport) full() bool { return len(r.items) >= maxMapImpacts || r.remaining <= 0 }

func (r *mapImpactReport) warn(
	chapter, address int,
	kind maps.ImpactKind,
	err error,
) {
	for _, warning := range r.warnings {
		if warning.Chapter == chapter && warning.Kind == kind &&
			fault.CodeOf(warning.Err) == fault.CodeOf(err) {
			return
		}
	}
	r.warnings = append(
		r.warnings,
		maps.ImpactWarning{
			Chapter: chapter,
			Address: address,
			Kind:    kind,
			Err:     err,
		},
	)
}

func (r *mapImpactReport) add(item maps.Impact) {
	if item.Bounds.Empty() {
		return
	}
	if r.full() {
		r.warn(item.Chapter, item.Address, item.Kind, ErrMapImpactLimit)
		return
	}
	r.remaining--
	key := mapImpactKey{item.Chapter, item.Address, item.Step, item.Kind}
	if r.seen[key] {
		return
	}
	r.seen[key] = true
	item.BeforeOutside, item.AfterOutside = !item.Bounds.In(
		r.before,
	), !item.Bounds.In(
		r.after,
	)
	if !item.BeforeOutside && !item.AfterOutside {
		return
	}
	before, after := item.Bounds.Intersect(
		r.before,
	), item.Bounds.Intersect(
		r.after,
	)
	common := before.Intersect(after)
	item.LostCells = before.Dx()*before.Dy() - common.Dx()*common.Dy()
	item.GainedCells = after.Dx()*after.Dy() - common.Dx()*common.Dy()
	r.items = append(r.items, item)
}

func (r *mapImpactReport) point(
	kind maps.ImpactKind,
	chapter, reference, address, index, step int,
	point image.Point,
) {
	r.add(maps.Impact{
		Kind:      kind,
		Chapter:   chapter,
		Reference: reference,
		Address:   address,
		Index:     index,
		Step:      step,
		Bounds:    image.Rectangle{Min: point, Max: point.Add(image.Pt(1, 1))},
	})
}

func (s *Editor) mapResizeImpacts(
	format Config,
	request maps.Resize,
) mapImpactReport {
	r := mapImpactReport{
		before: image.Rectangle{
			Max: request.Before,
		},
		after:     image.Rectangle{Max: request.After},
		seen:      make(map[mapImpactKey]bool),
		remaining: 65536,
	}
	if r.before == r.after {
		return r
	}
	for _, use := range request.Uses {
		if !use.Updated {
			continue
		}
		if r.full() {
			r.warn(
				use.Chapter,
				use.Pointer,
				maps.UnitStart,
				ErrMapImpactLimit,
			)
			break
		}
		groups, err := s.Events.Deployments(use.Chapter)
		if err != nil {
			r.warn(use.Chapter, use.Pointer, maps.UnitStart, err)
		}
		for _, warning := range groups.Warnings {
			r.warn(use.Chapter, use.Pointer, maps.UnitStart, warning)
		}
		areas, err := s.Events.Conditions(use.Chapter, event.AlwaysEvents)
		if err != nil {
			r.warn(use.Chapter, areas.Pointer, maps.TriggerArea, err)
		}
		for index, area := range areas.Events {
			r.add(maps.Impact{
				Kind:      maps.TriggerArea,
				Chapter:   use.Chapter,
				Reference: areas.Pointer,
				Address:   area.Address,
				Index:     index,
				Step:      -1,
				Bounds:    area.Area(),
			})
			groups.Groups = append(groups.Groups, area.Groups...)
			for _, warning := range area.Warnings {
				r.warn(use.Chapter, area.Address, maps.UnitStart, warning)
			}
		}
		if ending, ok := areas.Ending(); ok {
			groups.Groups = append(groups.Groups, ending.Groups...)
			for _, warning := range ending.Warnings {
				r.warn(use.Chapter, ending.Address, maps.UnitStart, warning)
			}
		}
		locations, locationErr := s.Events.Conditions(
			use.Chapter,
			event.LocationEvents,
		)
		if locationErr != nil {
			r.warn(
				use.Chapter,
				locations.Pointer,
				maps.TriggerLocation,
				locationErr,
			)
		}
		for index, condition := range locations.Events {
			if condition.Location.Kind == "" {
				r.warn(
					use.Chapter,
					condition.Address,
					maps.TriggerLocation,
					event.ErrEventConditions,
				)
				continue
			}
			position := condition.Location.Position
			r.add(
				maps.Impact{
					Kind:      maps.TriggerLocation,
					Chapter:   use.Chapter,
					Reference: locations.Pointer,
					Address:   condition.Address,
					Index:     index,
					Step:      -1,
					Bounds: image.Rect(
						position.X,
						position.Y,
						position.X+1,
						position.Y+1,
					),
				},
			)
		}
		r.deploymentGroups(use.Chapter, groups.Groups)
		s.mapChangeImpacts(format, use.Chapter, &r)
		s.mapScriptImpacts(use.Chapter, &r)
	}
	return r
}

func (r *mapImpactReport) deploymentGroups(
	chapter int,
	groups []deployment.Group,
) {
	seen := make(map[int]bool)
	for _, group := range groups {
		if group.Err != nil {
			r.warn(chapter, group.Pointer, maps.UnitStart, group.Err)
		}
		if seen[group.Address] {
			continue
		}
		seen[group.Address] = true
		for index, unit := range group.Units {
			if r.full() {
				r.warn(
					chapter,
					group.Pointer,
					maps.MovementNode,
					ErrMapImpactLimit,
				)
				return
			}
			address := group.Address + index*group.Stride
			r.point(
				maps.UnitStart,
				chapter,
				group.Pointer,
				address,
				index,
				-1,
				unit.Start,
			)
			if group.Stride != 20 {
				r.point(
					maps.UnitEnd,
					chapter,
					group.Pointer,
					address,
					index,
					-1,
					unit.End,
				)
				continue
			}
			origin, nodes := group.Movement(index)
			for step, node := range nodes {
				at := origin + step*8
				r.point(
					maps.MovementNode,
					chapter,
					group.Pointer,
					at,
					index,
					step,
					node.Position,
				)
			}
		}
	}
}

// Map-change geometry is inspected without rewriting its table or payload.
// Native records are 12 bytes. A negative signed ID ends the list; the
// common FF marker is not the only terminating byte.
func (s *Editor) mapChangeImpacts(
	format Config,
	chapter int,
	report *mapImpactReport,
) {
	id, err := s.records.Value(records.Chapters, chapter, "map_changes_id")
	if err != nil {
		report.warn(chapter, 0, maps.ChangeArea, err)
		return
	}
	geometry := format.Maps.ChangeGeometry(s.image.Data, id)
	for _, change := range geometry.Entries {
		report.add(
			maps.Impact{
				Kind:    maps.ChangeArea,
				Chapter: chapter,
				Address: change.Address,
				Index:   change.ID,
				Step:    -1,
				Bounds:  change.Bounds,
			},
		)
	}
	if geometry.Err != nil {
		report.warn(chapter, geometry.Address, maps.ChangeArea, geometry.Err)
	}
}

// Only a straight-line prefix with a known chapter context is inspected.
// Calls, branches, ASM and unknown instructions require runtime context and
// are reported as incomplete, rather than assigning their coordinates to a map.
func (s *Editor) mapScriptImpacts(
	chapter int,
	report *mapImpactReport,
) {
	entries, err := s.Events.ScriptEntries(chapter)
	if err != nil {
		report.warn(chapter, 0, maps.ScriptPosition, err)
		return
	}
	seen := make(map[int]bool)
	for _, entry := range entries {
		if entry.Err != nil {
			report.warn(
				chapter,
				entry.Pointer,
				maps.ScriptPosition,
				entry.Err,
			)
			continue
		}
		if entry.Address == 0 || seen[entry.Address] {
			continue
		}
		if entry.Source == "script_call_entry" {
			report.warn(
				chapter,
				entry.Pointer,
				maps.ScriptPosition,
				event.ErrMapImpactContext,
			)
			continue
		}
		seen[entry.Address] = true
		program, err := s.Events.Script(chapter, entry.Pointer)
		if err != nil {
			report.warn(chapter, entry.Pointer, maps.ScriptPosition, err)
			continue
		}
		s.mapScriptPrefix(program, report)
	}
}

func (s *Editor) mapScriptPrefix(
	program event.Script,
	report *mapImpactReport,
) {
	entry := program.Entry
	if !program.Complete {
		report.warn(
			entry.Chapter,
			entry.Address,
			maps.ScriptPosition,
			event.ErrMapImpactContext,
		)
	}
	for _, trace := range program.Coordinates(s.image.Data) {
		if report.full() {
			report.warn(
				entry.Chapter,
				trace.Address,
				maps.ScriptPosition,
				ErrMapImpactLimit,
			)
			return
		}
		for _, coordinate := range trace.Coordinates {
			kind := maps.ScriptPosition
			if coordinate.Path {
				kind = maps.ScriptPath
			}
			if coordinate.Err != nil {
				report.warn(entry.Chapter, trace.Address, kind, coordinate.Err)
				continue
			}
			report.point(
				kind,
				entry.Chapter,
				entry.Pointer,
				trace.Address,
				trace.Instruction,
				coordinate.Step,
				coordinate.Position,
			)
		}
	}
}
