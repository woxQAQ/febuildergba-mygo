package maps

import (
	"encoding/binary"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// MapUse distinguishes a shared pointer entry from an independent entry which
// still points to the original compressed map.
type Use struct {
	Chapter, ResourceID, Pointer int
	Updated                      bool
}

// MapEdit owns an immutable compressed layout and one pointer update. All
// chapters using that pointer entry change together; independent entries stay.
type Edit struct {
	binding                               edit.Stamp
	source, destination, pointer, changed int
	uses                                  []Use
	plan                                  space.Plan
	beforeSize, afterSize                 image.Point
	impacts                               []Impact
	warnings                              []ImpactWarning
}

func (e *Edit) Source() int { return e.source }

func (e *Edit) Destination() int { return e.destination }

func (e *Edit) Pointer() int { return e.pointer }

func (e *Edit) ChangedCells() int { return e.changed }

func (e *Edit) Length() int { return e.plan.Destination.End - e.plan.Destination.Start }

func (e *Edit) Uses() []Use { return slices.Clone(e.uses) }

func (s *Editor) Plan(draft *Draft) (result *Edit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_map", fault.Context{Resource: LayoutField}) }()
	s.Refresh()
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if !draft.Modified() {
		return nil, edit.ErrValue
	}
	format := s.format
	if s.context.Validate(s.image.Data) != nil {
		return nil, edit.ErrVersion
	}
	if err = s.mapWriteSite(
		format,
		draft.pointer,
		LayoutField,
	); err != nil {
		return nil, err
	}
	uses, err := s.mapUses(format, draft)
	if err != nil {
		return nil, err
	}
	compressed, err := gba.CompressLZ77(draft.content)
	if err != nil {
		return nil, err
	}
	plan, err := space.PlanAppend(len(s.image.Data), compressed)
	if err != nil {
		return nil, err
	}
	updated := binary.LittleEndian.AppendUint32(
		nil,
		// #nosec G115 -- PlanAppend bounds the destination to the GBA ROM address space.
		uint32(0x08000000+plan.Destination.Start),
	)
	plan.Edit.Changes = append(
		plan.Edit.Changes,
		rom.Change{Offset: draft.pointer, Data: updated},
	)
	impacts, warnings := s.source.Resize(
		Resize{
			Before: draft.originalSize(),
			After:  image.Pt(draft.width, draft.height),
			Uses:   uses,
		},
	)
	if !draft.Current(s) {
		return nil, rom.ErrStale
	}
	return &Edit{
		binding:     draft.binding,
		source:      draft.address,
		destination: plan.Destination.Start,
		pointer:     draft.pointer,
		changed:     draft.changedCells(),
		beforeSize:  draft.originalSize(),
		afterSize:   image.Pt(draft.width, draft.height),
		impacts:     impacts,
		warnings:    warnings,
		uses:        uses,
		plan:        plan,
	}, nil
}

func (s *Editor) Apply(edit *Edit) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_map", fault.Context{Resource: LayoutField}) }()
	if edit == nil {
		return rom.ErrStale
	}
	if err = edit.binding.Check(s.context); err != nil {
		return err
	}
	if err = s.context.Document().ApplyEdit(
		edit.binding.Revision(),
		LayoutField,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

type Binding struct {
	Field                         string
	Root, CompressedLimit, Length int
	Optional                      bool
}

// These are the chapter's concrete PLIST consumers, including the separate
// roots used by split lists. No ownership is inferred from pointer-like bytes.
func (f Format) bindings(extra []Binding) []Binding {
	bindings := []Binding{
		{Field: GraphicsField, Root: f.Graphics, CompressedLimit: 0x8000},
		{
			Field:           "map_object_alt_id",
			Root:            f.Graphics,
			CompressedLimit: 0x8000,
			Optional:        true,
		},
		{Field: "map_palette_id", Root: f.Palette, Length: 512},
		{Field: "map_tileset_id", Root: f.Config, CompressedLimit: 0x2400},
		{Field: LayoutField, Root: f.Layout, CompressedLimit: maxMapBytes},
		{Field: "tile_animation_1_id", Root: f.Animation, Optional: true},
		{Field: "tile_animation_2_id", Root: f.Animation2, Optional: true},
		{Field: "map_changes_id", Root: f.Changes},
	}
	if f.WorldEvents != 0 {
		bindings = append(
			bindings,
			Binding{
				Field:    "world_map_events_id",
				Root:     f.WorldEvents,
				Optional: true,
			},
		)
	}
	return append(bindings, extra...)
}

func (s *Editor) mapWriteSite(
	format Format,
	site int,
	field string,
) error {
	if err := s.source.Protect(s.image.Data, site); err != nil {
		return err
	}
	overlap := func(at, length int) bool { return at >= 0xC0 && length > 0 && site < at+length && at < site+4 }
	chapters := s.records.Table(records.Chapters)
	if chapters == nil || chapters.Err != nil {
		return edit.ErrLayout
	}
	seen := make(map[[2]int]bool)
	for _, binding := range format.bindings(s.source.Bindings) {
		if overlap(binding.Root, 4) {
			return edit.ErrLayout
		}
		list, valid := gba.Pointer(s.image.Data, binding.Root)
		if !valid {
			continue
		}
		for _, row := range chapters.Rows {
			id, err := s.records.Value(records.Chapters, row.ID, binding.Field)
			if err != nil {
				return err
			}
			if id == 0 && binding.Optional {
				continue
			}
			entry := list + int(id)*4
			if binding.Field != field && overlap(entry, 4) {
				return edit.ErrLayout
			}
			address, valid := gba.Pointer(s.image.Data, entry)
			if !valid {
				continue
			}
			if binding.Field == ChangesField &&
				mapChangesOverlap(s.image.Data, binding.Root, id, overlap) {
				return edit.ErrLayout
			}
			key := [2]int{address, binding.CompressedLimit}
			length := binding.Length
			if binding.CompressedLimit > 0 {
				if seen[key] {
					continue
				}
				seen[key] = true
				_, consumed, err := gba.DecompressLZ77Stream(
					s.image.Data[address:],
					binding.CompressedLimit,
				)
				if err != nil {
					continue
				}
				length = consumed
			}
			if overlap(address, max(4, length)) {
				return edit.ErrLayout
			}
		}
	}
	return nil
}

func mapChangesOverlap(
	data []byte,
	root int,
	id int64,
	overlap func(int, int) bool,
) bool {
	table, err := readMapChanges(data, root, id)
	if err != nil {
		return false
	}
	if overlap(table.address, len(table.rows)*12+len(table.terminator)) {
		return true
	}
	for _, change := range table.rows {
		payload := int(gba.Uint32(change.header[:], 8) & 0x01FFFFFF)
		if overlap(payload, len(change.tiles)) {
			return true
		}
	}
	return false
}

func (s *Editor) mapUses(
	format Format,
	draft *Draft,
) ([]Use, error) {
	chapters := s.records.Table(records.Chapters)
	if chapters == nil || chapters.Err != nil {
		return nil, edit.ErrLayout
	}
	list, valid := gba.Pointer(s.image.Data, format.Layout)
	if !valid {
		return nil, ErrMap
	}
	var uses []Use
	found := false
	for _, row := range chapters.Rows {
		id, err := s.records.Value(records.Chapters, row.ID, LayoutField)
		if err != nil {
			return nil, err
		}
		site := list + int(id)*4
		address, valid := gba.Pointer(s.image.Data, site)
		if !valid || address != draft.address {
			continue
		}
		updated := site == draft.pointer
		uses = append(
			uses,
			Use{
				Chapter:    row.ID,
				ResourceID: int(id),
				Pointer:    site,
				Updated:    updated,
			},
		)
		if !updated {
			continue
		}
		if row.ID == draft.chapter {
			found = true
		}
		resources, err := format.resources(s.image.Data, row.Address)
		if err != nil {
			return nil, err
		}
		if _, err = resources.preview(draft.content); err != nil {
			return nil, err
		}
	}
	if !found {
		return nil, rom.ErrStale
	}
	return uses, nil
}

// MapImpactKind identifies a confirmed coordinate contract, never a UI label.
type ImpactKind string

const (
	UnitStart      ImpactKind = "map_impact_unit_start"
	UnitEnd        ImpactKind = "map_impact_unit_end"
	MovementNode   ImpactKind = "map_impact_movement"
	TriggerArea    ImpactKind = "map_impact_area"
	ChangeArea     ImpactKind = "map_impact_change"
	ScriptPosition ImpactKind = "map_impact_script"
	ScriptPath     ImpactKind = "map_impact_script_path"
)

// MapImpact describes unchanged coordinates against the old and new map.
// LostCells counts previously in-bounds cells that this resize excludes, even
// when a region was already partly outside the original map.
type Impact struct {
	Kind                                     ImpactKind
	Chapter, Reference, Address, Index, Step int
	Bounds                                   image.Rectangle
	BeforeOutside, AfterOutside              bool
	LostCells, GainedCells                   int
}

// MapImpactWarning records a part of discovery that could not be completed.
// Err retains the stable domain error and its cause chain.
type ImpactWarning struct {
	Chapter, Address int
	Kind             ImpactKind
	Err              error
}

func (e *Edit) BeforeSize() image.Point { return e.beforeSize }

func (e *Edit) AfterSize() image.Point { return e.afterSize }

func (e *Edit) Resized() bool { return e.beforeSize != e.afterSize }

func (e *Edit) Impacts() []Impact { return slices.Clone(e.impacts) }

func (e *Edit) ImpactWarnings() []ImpactWarning { return slices.Clone(e.warnings) }
