package game

import (
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

// MapImpactKind identifies a confirmed coordinate contract, never a UI label.
type MapImpactKind string

const (
	MapUnitStart      MapImpactKind = "map_impact_unit_start"
	MapUnitEnd        MapImpactKind = "map_impact_unit_end"
	MapMovementNode   MapImpactKind = "map_impact_movement"
	MapTriggerArea    MapImpactKind = "map_impact_area"
	MapChangeArea     MapImpactKind = "map_impact_change"
	MapScriptPosition MapImpactKind = "map_impact_script"
	MapScriptPath     MapImpactKind = "map_impact_script_path"
)

// MapImpact describes unchanged coordinates against the old and new map.
// LostCells counts previously in-bounds cells that this resize excludes, even
// when a region was already partly outside the original map.
type MapImpact struct {
	Kind                                     MapImpactKind
	Chapter, Reference, Address, Index, Step int
	Bounds                                   image.Rectangle
	BeforeOutside, AfterOutside              bool
	LostCells, GainedCells                   int
}

// MapImpactWarning records a part of discovery that could not be completed.
// Err retains the stable domain error and its cause chain.
type MapImpactWarning struct {
	Chapter, Address int
	Kind             MapImpactKind
	Err              error
}

const maxMapImpacts = 4096

var (
	ErrMapImpactLimit = fault.Code{
		ID:   "game.map_impact_limit",
		Kind: fault.Capacity,
	}
	ErrMapImpactContext = fault.Code{
		ID:   "game.map_impact_context",
		Kind: fault.Unsupported,
	}
)

type mapImpactKey struct {
	chapter, address, step int
	kind                   MapImpactKind
}

type mapImpactReport struct {
	before, after image.Rectangle
	items         []MapImpact
	warnings      []MapImpactWarning
	seen          map[mapImpactKey]bool
	remaining     int
}

func (r *mapImpactReport) full() bool { return len(r.items) >= maxMapImpacts || r.remaining <= 0 }

func (e *MapEdit) BeforeSize() image.Point { return e.beforeSize }
func (e *MapEdit) AfterSize() image.Point  { return e.afterSize }

func (e *MapEdit) Resized() bool { return e.beforeSize != e.afterSize }

func (e *MapEdit) Impacts() []MapImpact { return slices.Clone(e.impacts) }

func (e *MapEdit) ImpactWarnings() []MapImpactWarning { return slices.Clone(e.warnings) }

func (r *mapImpactReport) warn(
	chapter, address int,
	kind MapImpactKind,
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
		MapImpactWarning{
			Chapter: chapter,
			Address: address,
			Kind:    kind,
			Err:     err,
		},
	)
}

func (r *mapImpactReport) add(item MapImpact) {
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
	kind MapImpactKind,
	chapter, reference, address, index, step int,
	point image.Point,
) {
	r.add(MapImpact{
		Kind:      kind,
		Chapter:   chapter,
		Reference: reference,
		Address:   address,
		Index:     index,
		Step:      step,
		Bounds:    image.Rectangle{Min: point, Max: point.Add(image.Pt(1, 1))},
	})
}

func (s *Session) mapResizeImpacts(
	format gameFormat,
	draft *MapDraft,
	uses []MapUse,
) mapImpactReport {
	r := mapImpactReport{
		before: image.Rectangle{
			Max: draft.originalSize(),
		},
		after:     image.Rect(0, 0, draft.width, draft.height),
		seen:      make(map[mapImpactKey]bool),
		remaining: 65536,
	}
	if r.before == r.after {
		return r
	}
	for _, use := range uses {
		if !use.Updated {
			continue
		}
		if r.full() {
			r.warn(use.Chapter, use.Pointer, MapUnitStart, ErrMapImpactLimit)
			break
		}
		groups, err := s.ChapterDeployments(use.Chapter)
		if err != nil {
			r.warn(use.Chapter, use.Pointer, MapUnitStart, err)
		}
		for _, warning := range groups.Warnings {
			r.warn(use.Chapter, use.Pointer, MapUnitStart, warning)
		}
		areas, err := s.ChapterEvents(use.Chapter, AlwaysEvents)
		if err != nil {
			r.warn(use.Chapter, areas.Pointer, MapTriggerArea, err)
		}
		for index, area := range areas.Events {
			r.add(MapImpact{
				Kind:      MapTriggerArea,
				Chapter:   use.Chapter,
				Reference: areas.Pointer,
				Address:   area.Address,
				Index:     index,
				Step:      -1,
				Bounds:    area.Area(),
			})
			groups.Groups = append(groups.Groups, area.Groups...)
			for _, warning := range area.Warnings {
				r.warn(use.Chapter, area.Address, MapUnitStart, warning)
			}
		}
		if areas.ending != nil {
			groups.Groups = append(groups.Groups, areas.ending.Groups...)
			for _, warning := range areas.ending.Warnings {
				r.warn(use.Chapter, areas.ending.Address, MapUnitStart, warning)
			}
		}
		r.deploymentGroups(use.Chapter, groups.Groups)
		s.mapChangeImpacts(format, use.Chapter, &r)
		s.mapScriptImpacts(format, use.Chapter, &r)
	}
	return r
}

func (r *mapImpactReport) deploymentGroups(
	chapter int,
	groups []DeploymentGroup,
) {
	seen := make(map[int]bool)
	for _, group := range groups {
		if group.Err != nil {
			r.warn(chapter, group.Pointer, MapUnitStart, group.Err)
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
					MapMovementNode,
					ErrMapImpactLimit,
				)
				return
			}
			address := group.Address + index*group.Stride
			r.point(
				MapUnitStart,
				chapter,
				group.Pointer,
				address,
				index,
				-1,
				unit.Start,
			)
			if group.Stride != 20 {
				r.point(
					MapUnitEnd,
					chapter,
					group.Pointer,
					address,
					index,
					-1,
					unit.End,
				)
				continue
			}
			record := group.records[index]
			for step, node := range movementSteps(record.movement) {
				at := int(u32(record.data, 8)&0x01FFFFFF) + step*8
				r.point(
					MapMovementNode,
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
func (s *Session) mapChangeImpacts(
	format gameFormat,
	chapter int,
	report *mapImpactReport,
) {
	id, err := s.Value(Chapters, chapter, "map_changes_id")
	if err != nil {
		report.warn(chapter, 0, MapChangeArea, err)
		return
	}
	// Absence is a null PLIST value, including a nonzero resource ID. Entry
	// zero can also contain a real table after a list has been relocated.
	list, valid := pointer(s.image.Data, format.maps.changes)
	if valid && within(s.image.Data, list+int(id)*4, 4) &&
		u32(s.image.Data, list+int(id)*4) == 0 {
		return
	}
	at, err := mapResource(
		s.image.Data,
		format.maps.changes,
		byte(id),
		"map_changes_id",
	) // #nosec G115 -- Chapter resource IDs are bytes.
	if err != nil {
		report.warn(chapter, 0, MapChangeArea, err)
		return
	}
	if at < 0xC0 || at%4 != 0 {
		report.warn(chapter, at, MapChangeArea, ErrMap)
		return
	}
	for index := 0; index <= 256; index++ {
		if !within(s.image.Data, at, 1) {
			break
		}
		if s.image.Data[at] >= 0x80 {
			return
		}
		if index == 256 || !within(s.image.Data, at, 12) {
			break
		}
		data := s.image.Data[at : at+12]
		x, y, w, h := int(data[1]), int(data[2]), int(data[3]), int(data[4])
		report.add(MapImpact{
			Kind:    MapChangeArea,
			Chapter: chapter,
			Address: at,
			Index:   int(data[0]),
			Step:    -1,
			Bounds:  image.Rect(x, y, x+w, y+h),
		})
		at += 12
	}
	report.warn(chapter, at, MapChangeArea, ErrMap)
}
