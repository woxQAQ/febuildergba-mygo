package game

import (
	"bytes"
	"encoding/binary"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type ScriptUse struct {
	Chapter, Pointer, Address int
	Source                    string
}

// ScriptEdit is one immutable parameter patch or append-and-pointer operation.
type ScriptEdit struct {
	document                    *rom.Document
	revision                    uint64
	layouts                     map[Kind]Layout
	source, destination, length int
	relocated                   bool
	before, after               int
	uses                        []ScriptUse
	movements                   int
	edit                        rom.Edit
}

func (edit *ScriptEdit) MovementCount() int { return edit.movements }
func (edit *ScriptEdit) Source() int        { return edit.source }
func (edit *ScriptEdit) Destination() int   { return edit.destination }
func (edit *ScriptEdit) Length() int        { return edit.length }
func (edit *ScriptEdit) Relocated() bool    { return edit.relocated }
func (edit *ScriptEdit) Before() int        { return edit.before }
func (edit *ScriptEdit) After() int         { return edit.after }
func (edit *ScriptEdit) Uses() []ScriptUse  { return slices.Clone(edit.uses) }

func (s *Session) PlanScriptEdit(
	draft *ScriptDraft,
) (result *ScriptEdit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_script", fault.Context{Resource: scriptResource}) }()
	s.Refresh()
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if !draft.Modified() {
		return nil, ErrValue
	}
	if !draft.Complete() {
		return nil, ErrScriptStructure
	}
	entry := draft.Entry()
	current, err := s.Script(entry.Chapter, entry.Pointer)
	if err != nil {
		return nil, err
	}
	if current.Entry.Address != entry.Address {
		return nil, rom.ErrStale
	}
	draft, resources, err := draft.prepareMovements(len(s.image.Data))
	if err != nil {
		return nil, err
	}
	structural := draft.structural()
	if structural && !draft.Relocatable() {
		return nil, ErrScriptStructure
	}
	if draft.labelsChanged(structural) {
		if err = draft.validateLabels(); err != nil {
			return nil, err
		}
	}
	destination := entry.Address
	if structural {
		destination = (resources.Size + 3) &^ 3
	}
	content, positions, err := draft.scriptContent(destination, structural)
	if err != nil {
		return nil, err
	}
	edit, err := s.scriptWritePlan(
		draft,
		destination,
		content,
		structural,
		resources.Size,
	)
	if err != nil {
		return nil, err
	}
	edit.Changes = append(resources.Changes, edit.Changes...)
	prospective := bytes.Repeat([]byte{0xFF}, edit.Size)
	copy(prospective, s.image.Data)
	for _, change := range edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	if err = s.validateScriptEdit(
		draft,
		prospective,
		positions,
		destination,
	); err != nil {
		return nil, err
	}
	result = &ScriptEdit{
		document:    s.Document,
		revision:    s.image.Revision,
		layouts:     maps.Clone(s.layouts),
		source:      entry.Address,
		destination: destination,
		length:      len(content),
		movements:   len(resources.Changes),
		relocated:   structural,
		before:      len(draft.original.rows),
		after:       len(draft.rows),
		edit:        edit,
	}
	result.uses = s.scriptUses(entry, structural, draft.originalLength())
	if err = s.validateSharedMapChangeScript(
		draft,
		result.uses,
		structural,
	); err != nil {
		return nil, err
	}
	if !draft.Current(s) {
		return nil, rom.ErrStale
	}
	return result, nil
}

func (d *ScriptDraft) labelsChanged(structural bool) bool {
	if structural {
		return true
	}
	for index, row := range d.rows {
		if row.definition != nil &&
			(row.definition.label || row.definition.branch) &&
			!bytes.Equal(
				row.instruction.Raw,
				d.original.rows[index].instruction.Raw,
			) {
			return true
		}
	}
	return false
}

func (s *Session) scriptWritePlan(
	draft *ScriptDraft,
	destination int,
	content []byte,
	structural bool,
	baseSize int,
) (rom.Edit, error) {
	entry := draft.Entry()
	if structural {
		if err := s.resourcePointer(entry.Pointer); err != nil {
			return rom.Edit{}, err
		}
		plan, err := space.PlanAppend(baseSize, content)
		if err != nil {
			return rom.Edit{}, err
		}
		plan.Edit.Changes = append(
			plan.Edit.Changes,
			rom.Change{
				Offset: entry.Pointer,
				Data: binary.LittleEndian.AppendUint32(
					nil,
					uint32(0x08000000+destination),
				),
			},
		) // #nosec G115 -- PlanAppend bounds the destination.
		return plan.Edit, nil
	}
	if err := s.scriptWriteRange(
		entry.Address,
		entry.Address+len(content),
	); err != nil {
		return rom.Edit{}, err
	}
	edit := rom.Edit{Size: baseSize}
	for index, row := range draft.rows {
		if !bytes.Equal(
			row.instruction.Raw,
			draft.original.rows[index].instruction.Raw,
		) {
			edit.Changes = append(
				edit.Changes,
				rom.Change{
					Offset: row.instruction.Address,
					Data:   bytes.Clone(row.instruction.Raw),
				},
			)
		}
	}
	return edit, nil
}

func (s *Session) validateScriptEdit(
	draft *ScriptDraft,
	data []byte,
	positions []int,
	destination int,
) error {
	decoded := draft.original.format.readScript(
		data,
		ScriptEntry{Address: destination},
	)
	if !decoded.Complete || len(decoded.rows) != len(draft.rows) {
		return ErrScript
	}
	structural := draft.structural()
	originals := make(map[uint64]scriptRow)
	for _, row := range draft.original.rows {
		originals[row.instruction.ID] = row
	}
	for index, row := range draft.rows {
		if row.definition == nil {
			continue
		}
		for _, field := range row.definition.fields {
			old, found := originals[row.instruction.ID]
			if found && (!structural || field.Target != ScriptMovementTarget) &&
				field.value(
					old.instruction.Raw,
				) == field.value(
					row.instruction.Raw,
				) {
				continue
			}
			if err := s.validateScriptOperand(
				data,
				positions[index],
				field,
				draft.original.format,
			); err != nil {
				return err
			}
		}
	}
	if err := s.validateSlotUnitLoads(draft, data, positions); err != nil {
		return err
	}
	return nil
}

func (s *Session) validateScriptOperand(
	data []byte,
	address int,
	field ScriptField,
	format deploymentFormat,
) error {
	value := field.value(data[address:])
	if value == 0 && field.Optional {
		return nil
	}
	switch field.Target {
	case ScriptMovementTarget:
		if value < 0x080000C0 || value >= 0x0A000000 {
			return ErrScriptStructure
		}
		_, err := readScriptMovement(data, int(value)-0x08000000)
		return err
	case ScriptTextTarget:
		if format.kind == eventFE8 && (value < 0 || value >= 0x8000) {
			return nil
		}
		_, err := s.DialogueText(
			uint16(value),
		) // #nosec G115 -- Text operands are at most 16 bits.
		return err
	case ScriptEventTarget:
		if format.kind == eventFE8 && value == 0xFFFFFFFF {
			return nil
		}
		target, valid := pointer(data, address+field.offset)
		if !valid || target < 0xC0 || target%4 != 0 {
			return ErrScript
		}
		if !format.readScript(data, ScriptEntry{Address: target}).Complete {
			return ErrScript
		}
	case ScriptDeploymentTarget:
		if format.kind == eventFE8 && value >= 0x80000000 {
			return nil
		}
		if value == 0 {
			return ErrDeployment
		}
		group := format.group(
			data,
			address+field.offset,
			"script_units",
			0,
			make(map[int]DeploymentGroup),
		)
		return group.Err
	}
	return nil
}

func (s *Session) scriptUses(
	entry ScriptEntry,
	relocated bool,
	length int,
) []ScriptUse {
	var result []ScriptUse
	table := s.Table(Chapters)
	if table == nil || table.Err != nil {
		return result
	}
	for _, chapter := range table.Rows {
		index, err := s.scriptIndex(chapter.ID)
		if err != nil {
			continue
		}
		for _, candidate := range index.entries {
			if candidate.Err != nil {
				continue
			}
			affected := candidate.Pointer == entry.Pointer
			if !relocated {
				program := index.programs[candidate.Address]
				programLength := 0
				for _, row := range program.rows {
					programLength += len(row.instruction.Raw)
				}
				affected = candidate.Address < entry.Address+length &&
					entry.Address < candidate.Address+programLength
			}
			if affected {
				result = append(
					result,
					ScriptUse{
						Chapter: chapter.ID,
						Pointer: candidate.Pointer,
						Address: candidate.Address,
						Source:  candidate.Source,
					},
				)
			}
		}
	}
	return result
}

func (s *Session) ApplyScriptEdit(edit *ScriptEdit) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_script", fault.Context{Resource: scriptResource}) }()
	if edit == nil || edit.document != s.Document ||
		!maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err = s.Document.ApplyEdit(
		edit.revision,
		scriptResource,
		edit.edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

// In-place code edits cannot claim bytes already described as record data,
// condition lists, event headers, deployment records or movement paths.
func (s *Session) scriptWriteRange(start, end int) error {
	format, err := s.scriptFormat()
	if err != nil {
		return err
	}
	overlap := func(at, length int) bool { return length > 0 && start < at+length && at < end }
	for _, table := range s.Tables {
		if overlap(table.Pointer, 4) {
			return ErrLayout
		}
		if table.Err == nil &&
			overlap(table.Address, table.Count*table.Stride) {
			return ErrLayout
		}
	}
	for _, at := range []int{format.text.pointer, format.text.treePointer, format.text.rootPointer, format.deployments.root} {
		if overlap(at, 4) {
			return ErrLayout
		}
	}
	if text, readErr := s.Adapter.TextTable(
		s.image.Data,
	); readErr == nil &&
		overlap(text.Address, text.Count*4) {
		return ErrLayout
	}
	chapters := s.Table(Chapters)
	if chapters == nil || chapters.Err != nil {
		return ErrLayout
	}
	list, valid := pointer(s.image.Data, format.deployments.root)
	if !valid {
		return ErrLayout
	}
	for _, chapter := range chapters.Rows {
		value, valueErr := s.Value(Chapters, chapter.ID, chapterEventsField)
		if valueErr != nil {
			continue
		}
		site := list + int(value)*4
		if overlap(site, 4) {
			return ErrLayout
		}
		header, valid := pointer(s.image.Data, site)
		if !valid {
			continue
		}
		if overlap(header, format.deployments.ending+4) {
			return ErrLayout
		}
		for _, table := range eventTables {
			conditions, readErr := format.deployments.readEvents(
				s.image.Data,
				header+table.offset(),
				table,
			)
			if readErr != nil {
				continue
			}
			for _, region := range conditions.ranges {
				if overlap(region.Start, region.End-region.Start) {
					return ErrLayout
				}
			}
		}
		groups, readErr := s.ChapterDeployments(chapter.ID)
		if readErr != nil {
			continue
		}
		if scriptDeploymentOverlap(groups.Groups, overlap) {
			return ErrLayout
		}
	}
	return nil
}
