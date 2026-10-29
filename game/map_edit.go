package game

import (
	"encoding/binary"
	"image"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// MapUse distinguishes a shared pointer entry from an independent entry which
// still points to the original compressed map.
type MapUse struct {
	Chapter, ResourceID, Pointer int
	Updated                      bool
}

// MapEdit owns an immutable compressed layout and one pointer update. All
// chapters using that pointer entry change together; independent entries stay.
type MapEdit struct {
	document                              *rom.Document
	revision                              uint64
	layouts                               map[Kind]Layout
	source, destination, pointer, changed int
	uses                                  []MapUse
	plan                                  space.Plan
	beforeSize, afterSize                 image.Point
	impacts                               []MapImpact
	warnings                              []MapImpactWarning
}

func (e *MapEdit) Source() int       { return e.source }
func (e *MapEdit) Destination() int  { return e.destination }
func (e *MapEdit) Pointer() int      { return e.pointer }
func (e *MapEdit) ChangedCells() int { return e.changed }

func (e *MapEdit) Length() int    { return e.plan.Destination.End - e.plan.Destination.Start }
func (e *MapEdit) Uses() []MapUse { return slices.Clone(e.uses) }

func (s *Session) PlanMapEdit(draft *MapDraft) (result *MapEdit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_map", fault.Context{Resource: chapterMapField}) }()
	s.Refresh()
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if !draft.Modified() {
		return nil, ErrValue
	}
	format, ok := s.Adapter.(gameFormat)
	if !ok || !format.Match(s.image.Data) {
		return nil, ErrVersion
	}
	if err = s.mapWriteSite(format, draft.pointer); err != nil {
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
	impacts := s.mapResizeImpacts(format, draft, uses)
	if !draft.Current(s) {
		return nil, rom.ErrStale
	}
	return &MapEdit{
		document:    draft.document,
		revision:    draft.revision,
		layouts:     maps.Clone(draft.layouts),
		source:      draft.address,
		destination: plan.Destination.Start,
		pointer:     draft.pointer,
		changed:     draft.changedCells(),
		beforeSize:  draft.originalSize(),
		afterSize:   image.Pt(draft.width, draft.height),
		impacts:     impacts.items,
		warnings:    impacts.warnings,
		uses:        uses,
		plan:        plan,
	}, nil
}

func (s *Session) ApplyMapEdit(edit *MapEdit) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_map", fault.Context{Resource: chapterMapField}) }()
	if edit == nil || edit.document != s.Document ||
		!maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err = s.Document.ApplyEdit(
		edit.revision,
		chapterMapField,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

type mapBinding struct {
	field                         string
	root, compressedLimit, length int
	optional                      bool
}

// These are the chapter's concrete PLIST consumers, including the separate
// roots used by split lists. No ownership is inferred from pointer-like bytes.
func (g gameFormat) mapBindings() []mapBinding {
	f := g.maps
	bindings := []mapBinding{
		{field: mapGraphicsField, root: f.graphics, compressedLimit: 0x8000},
		{
			field:           "map_object_alt_id",
			root:            f.graphics,
			compressedLimit: 0x8000,
			optional:        true,
		},
		{field: "map_palette_id", root: f.palette, length: 512},
		{field: "map_tileset_id", root: f.config, compressedLimit: 0x2400},
		{field: chapterMapField, root: f.layout, compressedLimit: maxMapBytes},
		{field: "tile_animation_1_id", root: f.animation, optional: true},
		{field: "tile_animation_2_id", root: f.animation2, optional: true},
		{field: "map_changes_id", root: f.changes},
		{
			field:    chapterEventsField,
			root:     g.deployments.root,
			length:   g.deployments.ending + 4,
			optional: true,
		},
	}
	if f.worldEvents != 0 {
		bindings = append(
			bindings,
			mapBinding{
				field:    "world_map_events_id",
				root:     f.worldEvents,
				optional: true,
			},
		)
	}
	return bindings
}

func (s *Session) mapWriteSite(format gameFormat, site int) error {
	if err := s.resourcePointer(site); err != nil {
		return fault.New(
			ErrLayout,
			"game.map_pointer_write",
			fault.Context{},
			err,
		)
	}
	overlap := func(at, length int) bool { return at >= 0xC0 && length > 0 && site < at+length && at < site+4 }
	for _, at := range []int{format.text.pointer, format.text.rootPointer, format.text.treePointer, format.icons.pointer, format.icons.palette, format.portraits.pointer} {
		if overlap(at, 4) {
			return ErrLayout
		}
	}
	if text, err := s.Adapter.TextTable(
		s.image.Data,
	); err == nil &&
		overlap(text.Address, text.Count*4) {
		return ErrLayout
	}
	for _, region := range format.deploymentRanges(s.image.Data) {
		if overlap(region.Start, region.End-region.Start) {
			return ErrLayout
		}
	}
	chapters := s.Table(Chapters)
	if chapters == nil || chapters.Err != nil {
		return ErrLayout
	}
	seen := make(map[[2]int]bool)
	for _, binding := range format.mapBindings() {
		if overlap(binding.root, 4) {
			return ErrLayout
		}
		list, valid := pointer(s.image.Data, binding.root)
		if !valid {
			continue
		}
		for _, row := range chapters.Rows {
			id, err := s.Value(Chapters, row.ID, binding.field)
			if err != nil {
				return err
			}
			if id == 0 && binding.optional {
				continue
			}
			entry := list + int(id)*4
			if binding.field != chapterMapField && overlap(entry, 4) {
				return ErrLayout
			}
			address, valid := pointer(s.image.Data, entry)
			if !valid {
				continue
			}
			key := [2]int{address, binding.compressedLimit}
			length := binding.length
			if binding.compressedLimit > 0 {
				if seen[key] {
					continue
				}
				seen[key] = true
				_, consumed, err := gba.DecompressLZ77Stream(
					s.image.Data[address:],
					binding.compressedLimit,
				)
				if err != nil {
					continue
				}
				length = consumed
			}
			if overlap(address, max(4, length)) {
				return ErrLayout
			}
		}
	}
	return nil
}

func (s *Session) mapUses(
	format gameFormat,
	draft *MapDraft,
) ([]MapUse, error) {
	chapters := s.Table(Chapters)
	if chapters == nil || chapters.Err != nil {
		return nil, ErrLayout
	}
	list, valid := pointer(s.image.Data, format.maps.layout)
	if !valid {
		return nil, ErrMap
	}
	var uses []MapUse
	found := false
	for _, row := range chapters.Rows {
		id, err := s.Value(Chapters, row.ID, chapterMapField)
		if err != nil {
			return nil, err
		}
		site := list + int(id)*4
		address, valid := pointer(s.image.Data, site)
		if !valid || address != draft.address {
			continue
		}
		updated := site == draft.pointer
		uses = append(
			uses,
			MapUse{
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
		resources, err := format.maps.resources(s.image.Data, row.Address)
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
