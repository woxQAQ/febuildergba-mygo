package maps

import (
	"bytes"
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

// MapChangesEdit appends a frozen table and only changed tile payloads. The
// selected PLIST entry is the sole existing write site; scripts stay intact.
type ChangesEdit struct {
	binding                                  edit.Stamp
	plan                                     space.Plan
	source, pointer, before, after, payloads int
	uses                                     []Use
	references                               ChangeReferences
}

func (e *ChangesEdit) Source() int { return e.source }

func (e *ChangesEdit) Destination() int { return e.plan.Destination.Start }

func (e *ChangesEdit) Pointer() int { return e.pointer }

func (e *ChangesEdit) Length() int { return e.plan.Destination.End - e.plan.Destination.Start }

func (e *ChangesEdit) Before() int { return e.before }

func (e *ChangesEdit) After() int { return e.after }

func (e *ChangesEdit) Payloads() int { return e.payloads }

func (e *ChangesEdit) Uses() []Use { return slices.Clone(e.uses) }

func (e *ChangesEdit) References() ChangeReferences {
	return ChangeReferences{
		References: slices.Clone(e.references.References),
		Warnings:   slices.Clone(e.references.Warnings),
	}
}

func (e *ChangesEdit) Applicable() bool {
	return !slices.ContainsFunc(
		e.references.References,
		func(ref ChangeReference) bool { return ref.Before >= 0 && ref.After < 0 },
	)
}

func (s *Editor) mapResourceUses(
	format Format,
	field string,
	site, source int,
) ([]Use, error) {
	chapters := s.records.Table(records.Chapters)
	if chapters == nil || chapters.Err != nil {
		return nil, edit.ErrLayout
	}
	root := 0
	for _, binding := range format.bindings(s.source.Bindings) {
		if binding.Field == field {
			root = binding.Root
			break
		}
	}
	list, valid := gba.Pointer(s.image.Data, root)
	if !valid {
		return nil, ErrMapChanges
	}
	var result []Use
	for _, row := range chapters.Rows {
		id, err := s.records.Value(records.Chapters, row.ID, field)
		if err != nil {
			return nil, err
		}
		entry := list + int(id)*4
		address, valid := gba.Pointer(s.image.Data, entry)
		updated := entry == site
		if !updated && (!valid || source == 0 || address != source) {
			continue
		}
		result = append(
			result,
			Use{
				Chapter:    row.ID,
				ResourceID: int(id),
				Pointer:    entry,
				Updated:    updated,
			},
		)
	}
	return result, nil
}

func (s *Editor) PlanChanges(
	draft *ChangesDraft,
) (result *ChangesEdit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_map_changes", fault.Context{Resource: ChangesField}) }()
	s.Refresh()
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if !draft.Modified() {
		return nil, edit.ErrValue
	}
	draft = draft.snapshot()
	format := s.format
	if s.context.Validate(s.image.Data) != nil {
		return nil, edit.ErrVersion
	}
	if err = s.mapWriteSite(
		format,
		draft.original.pointer,
		ChangesField,
	); err != nil {
		return nil, err
	}
	uses, err := s.mapResourceUses(
		format,
		ChangesField,
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
	references := s.source.References(draft, uses)
	if !draft.Current(s) {
		return nil, rom.ErrStale
	}
	return &ChangesEdit{
		binding:    draft.base.binding,
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

func (s *Editor) validateMapChanges(
	format Format,
	draft *ChangesDraft,
	uses []Use,
) error {
	found := false
	for _, use := range uses {
		if !use.Updated {
			continue
		}
		if use.Chapter == draft.Chapter() {
			found = true
		}
		_, chapter, _, err := s.records.Resolve(
			records.Chapters,
			use.Chapter,
			ChangesField,
		)
		if err != nil {
			return err
		}
		resources, err := format.resources(s.image.Data, chapter.Address)
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
				tile := gba.Uint16(row.tiles, offset)
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

func (s *Editor) ApplyChanges(edit *ChangesEdit) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_map_changes", fault.Context{Resource: ChangesField}) }()
	if edit == nil {
		return rom.ErrStale
	}
	if err = edit.binding.Check(s.context); err != nil {
		return err
	}
	if !edit.Applicable() {
		return ErrMapChangeReference
	}
	if err = s.context.Document().ApplyEdit(
		edit.binding.Revision(),
		ChangesField,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
