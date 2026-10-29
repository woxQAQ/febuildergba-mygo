package game

import (
	"bytes"
	"encoding/binary"
	"image"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// MapChangesEdit appends a frozen table and only changed tile payloads. The
// selected PLIST entry is the sole existing write site; scripts stay intact.
type MapChangesEdit struct {
	document                                 *rom.Document
	revision                                 uint64
	layouts                                  map[Kind]Layout
	plan                                     space.Plan
	source, pointer, before, after, payloads int
	uses                                     []MapUse
	references                               MapChangeReferences
}

func (e *MapChangesEdit) Source() int      { return e.source }
func (e *MapChangesEdit) Destination() int { return e.plan.Destination.Start }
func (e *MapChangesEdit) Pointer() int     { return e.pointer }

func (e *MapChangesEdit) Length() int    { return e.plan.Destination.End - e.plan.Destination.Start }
func (e *MapChangesEdit) Before() int    { return e.before }
func (e *MapChangesEdit) After() int     { return e.after }
func (e *MapChangesEdit) Payloads() int  { return e.payloads }
func (e *MapChangesEdit) Uses() []MapUse { return slices.Clone(e.uses) }
func (e *MapChangesEdit) References() MapChangeReferences {
	return MapChangeReferences{
		References: slices.Clone(e.references.References),
		Warnings:   slices.Clone(e.references.Warnings),
	}
}

func (e *MapChangesEdit) Applicable() bool {
	return !slices.ContainsFunc(
		e.references.References,
		func(ref MapChangeReference) bool { return ref.Before >= 0 && ref.After < 0 },
	)
}

func (s *Session) mapResourceUses(
	format gameFormat,
	field string,
	site, source int,
) ([]MapUse, error) {
	chapters := s.Table(Chapters)
	if chapters == nil || chapters.Err != nil {
		return nil, ErrLayout
	}
	root := 0
	for _, binding := range format.mapBindings() {
		if binding.field == field {
			root = binding.root
			break
		}
	}
	list, valid := pointer(s.image.Data, root)
	if !valid {
		return nil, ErrMapChanges
	}
	var result []MapUse
	for _, row := range chapters.Rows {
		id, err := s.Value(Chapters, row.ID, field)
		if err != nil {
			return nil, err
		}
		entry := list + int(id)*4
		address, valid := pointer(s.image.Data, entry)
		updated := entry == site
		if !updated && (!valid || source == 0 || address != source) {
			continue
		}
		result = append(
			result,
			MapUse{
				Chapter:    row.ID,
				ResourceID: int(id),
				Pointer:    entry,
				Updated:    updated,
			},
		)
	}
	return result, nil
}

func (s *Session) PlanMapChangesEdit(
	draft *MapChangesDraft,
) (result *MapChangesEdit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_map_changes", fault.Context{Resource: mapChangesField}) }()
	s.Refresh()
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if !draft.Modified() {
		return nil, ErrValue
	}
	draft = draft.snapshot()
	format, ok := s.Adapter.(gameFormat)
	if !ok || !format.Match(s.image.Data) {
		return nil, ErrVersion
	}
	if err = s.mapWriteSite(
		format,
		draft.original.pointer,
		mapChangesField,
	); err != nil {
		return nil, err
	}
	uses, err := s.mapResourceUses(
		format,
		mapChangesField,
		draft.original.pointer,
		draft.original.address,
	)
	if err != nil {
		return nil, err
	}
	if err = s.validateMapChanges(format, draft, uses); err != nil {
		return nil, err
	}
	content := make([]byte, len(draft.rows)*12)
	content = append(content, draft.original.terminator...)
	destination := (len(s.image.Data) + 3) &^ 3
	payloads := 0
	for index, row := range draft.rows {
		header := row.header
		if !bytes.Equal(row.tiles, row.originalTiles) {
			for len(content)%4 != 0 {
				content = append(content, 0)
			}
			address := destination + len(content)
			if address > rom.MaxSize {
				return nil, space.ErrCapacity
			}
			binary.LittleEndian.PutUint32(
				header[8:],
				uint32(0x08000000+address),
			) // #nosec G115 -- Bounded by the native ROM capacity.
			content = append(content, row.tiles...)
			payloads++
		}
		copy(content[index*12:], header[:])
	}
	plan, err := space.PlanAppend(len(s.image.Data), content)
	if err != nil {
		return nil, err
	}
	updated := binary.LittleEndian.AppendUint32(
		nil,
		uint32(0x08000000+plan.Destination.Start),
	) // #nosec G115 -- PlanAppend bounds the ROM address.
	plan.Edit.Changes = append(
		plan.Edit.Changes,
		rom.Change{Offset: draft.original.pointer, Data: updated},
	)
	references := s.mapChangeReferences(draft, uses)
	if !draft.Current(s) {
		return nil, rom.ErrStale
	}
	return &MapChangesEdit{
		document:   draft.base.document,
		revision:   draft.base.revision,
		layouts:    maps.Clone(draft.base.layouts),
		plan:       plan,
		source:     draft.original.address,
		pointer:    draft.original.pointer,
		before:     len(draft.original.rows),
		after:      len(draft.rows),
		payloads:   payloads,
		uses:       uses,
		references: references,
	}, nil
}

func (s *Session) validateMapChanges(
	format gameFormat,
	draft *MapChangesDraft,
	uses []MapUse,
) error {
	found := false
	for _, use := range uses {
		if !use.Updated {
			continue
		}
		if use.Chapter == draft.Chapter() {
			found = true
		}
		_, chapter, _, err := s.locate(Chapters, use.Chapter, mapChangesField)
		if err != nil {
			return err
		}
		resources, err := format.maps.resources(s.image.Data, chapter.Address)
		if err != nil {
			return err
		}
		base, err := resources.preview(resources.layout)
		if err != nil {
			return err
		}
		for _, row := range draft.rows {
			oldIndex := slices.IndexFunc(
				draft.original.rows,
				func(old mapChangeRecord) bool { return row.address != 0 && old.address == row.address },
			)
			if oldIndex >= 0 {
				old := draft.original.rows[oldIndex]
				if old.header == row.header &&
					bytes.Equal(old.tiles, row.tiles) {
					continue
				}
			}
			bounds := row.info().Bounds
			if bounds.Empty() ||
				!bounds.In(image.Rect(0, 0, base.Width, base.Height)) {
				return ErrMapChanges
			}
			for offset := 0; offset < len(row.tiles); offset += 2 {
				tile := u16(row.tiles, offset)
				if tile == 0 {
					continue
				}
				if _, _, valid := resources.tile(tile); !valid {
					return ErrMapChanges
				}
			}
		}
	}
	if !found {
		return rom.ErrStale
	}
	return nil
}

func (s *Session) ApplyMapChangesEdit(edit *MapChangesEdit) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_map_changes", fault.Context{Resource: mapChangesField}) }()
	if edit == nil || edit.document != s.Document ||
		!maps.Equal(edit.layouts, s.layouts) ||
		edit.revision != s.Document.State().Revision {
		return rom.ErrStale
	}
	if !edit.Applicable() {
		return ErrMapChangeReference
	}
	if err = s.Document.ApplyEdit(
		edit.revision,
		mapChangesField,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
