package event

import (
	"bytes"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

type ScriptUse struct {
	Chapter, Pointer, Address int
	Source                    string
	WorldMap                  bool
	EventID                   int
}

// ScriptPlan is one immutable parameter patch or append-and-pointer operation.
type ScriptPlan struct {
	commit                      *edit.Commit
	source, destination, length int
	relocated                   bool
	before, after               int
	uses                        []ScriptUse
	movements                   int
}

func (plan *ScriptPlan) MovementCount() int { return plan.movements }

func (plan *ScriptPlan) Source() int { return plan.source }

func (plan *ScriptPlan) Destination() int { return plan.destination }

func (plan *ScriptPlan) Length() int { return plan.length }

func (plan *ScriptPlan) Relocated() bool { return plan.relocated }

func (plan *ScriptPlan) Before() int { return plan.before }

func (plan *ScriptPlan) After() int { return plan.after }

func (plan *ScriptPlan) Uses() []ScriptUse { return slices.Clone(plan.uses) }

func (s *Editor) PlanScriptEdit(
	draft *ScriptDraft,
) (result *ScriptPlan, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_script", fault.Context{Resource: scriptResource}) }()
	s.Refresh()
	if draft == nil {
		return nil, rom.ErrStale
	}
	if err = draft.binding.Check(s.context); err != nil {
		return nil, err
	}
	if draft.ReadOnly() {
		return nil, ErrScriptStructure
	}
	if !draft.Modified() {
		return nil, edit.ErrValue
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
	builder := romwrite.NewBuilder(s.image.Data, romwrite.Protection{
		Pointer: s.source.ProtectPointer,
		Range: func(_ []byte, _ romwrite.Owner, region romwrite.Range) error {
			return s.scriptWriteRange(region.Start, region.End)
		},
	})
	draft, movements, err := draft.prepareMovements(builder)
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
	var content []byte
	var positions []int
	if structural {
		destination, err = builder.ReplaceEncoded(func(at int) ([]byte, error) {
			content, positions, err = draft.scriptContent(at, true)
			return content, err
		}, romwrite.PointerSite{
			Owner: ScriptOwner(entry.Pointer), Address: entry.Pointer,
			ExpectedWord: gba.Uint32(s.image.Data, entry.Pointer),
		})
	} else {
		content, positions, err = draft.scriptContent(destination, false)
		if err == nil {
			err = s.patchScript(draft, builder, content)
		}
	}
	if err != nil {
		return nil, err
	}
	write := builder.Plan()
	prospective := bytes.Repeat([]byte{0xFF}, write.Edit.Size)
	copy(prospective, s.image.Data)
	for _, change := range write.Edit.Changes {
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
	result = &ScriptPlan{
		commit:      edit.NewCommit(draft.binding, scriptResource, write.Edit),
		source:      entry.Address,
		destination: destination,
		length:      len(content),
		movements:   movements,
		relocated:   structural,
		before:      len(draft.original.rows),
		after:       len(draft.rows),
	}
	result.uses = s.scriptUses(entry, structural, draft.originalLength())
	if err = s.validateSharedMapChangeScript(
		draft,
		result.uses,
		structural,
	); err != nil {
		return nil, err
	}
	if err = draft.binding.Check(s.context); err != nil {
		return nil, err
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

func (s *Editor) patchScript(
	draft *ScriptDraft,
	builder *romwrite.Builder,
	content []byte,
) error {
	entry := draft.Entry()
	if err := s.scriptWriteRange(
		entry.Address,
		entry.Address+len(content),
	); err != nil {
		return err
	}
	for index, row := range draft.rows {
		if !bytes.Equal(
			row.instruction.Raw,
			draft.original.rows[index].instruction.Raw,
		) {
			if err := builder.Patch(
				ScriptCodeOwner(),
				row.instruction.Address,
				row.instruction.Raw,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Editor) validateScriptEdit(
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
		old, found := originals[row.instruction.ID]
		if err := s.validateWorldMapMovement(row, old, found); err != nil {
			return err
		}
		for _, field := range row.definition.fields {
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

func (s *Editor) validateScriptOperand(
	data []byte,
	address int,
	field ScriptField,
	format Format,
) error {
	value := field.value(data[address:])
	if err := s.validateWorldMapOperand(field, value); err != nil {
		return err
	}
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
	case ScriptPortraitTarget:
		return s.validateScriptPortrait(value)
	case ScriptTextTarget:
		if field.isIndirect(value) {
			return nil
		}
		_, err := s.DialogueText(
			uint16(value),
		) // #nosec G115 -- Text operands are at most 16 bits.
		return err
	case ScriptEventTarget:
		if field.isIndirect(value) {
			return nil
		}
		target, valid := gba.Pointer(data, address+field.offset)
		if !valid || target < 0xC0 || target%4 != 0 {
			return ErrScript
		}
		if !format.readScript(data, ScriptEntry{Address: target}).Complete {
			return ErrScript
		}
	case ScriptDeploymentTarget:
		if field.isIndirect(value) {
			return nil
		}
		if value == 0 {
			return deployment.ErrDeployment
		}
		group := deployment.NewReader(data, format.units).Group(
			address+field.offset,
			"script_units",
			0,
		)
		return group.Err
	}
	return nil
}

func (s *Editor) scriptUses(
	entry ScriptEntry,
	relocated bool,
	length int,
) []ScriptUse {
	var result []ScriptUse
	seen := make(map[int]bool)
	consider := func(index scriptIndex, global bool) {
		for _, candidate := range index.entries {
			if candidate.Err != nil || global && seen[candidate.Pointer] {
				continue
			}
			if !global {
				seen[candidate.Pointer] = true
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
						Chapter:  candidate.Chapter,
						Pointer:  candidate.Pointer,
						Address:  candidate.Address,
						Source:   candidate.Source,
						WorldMap: candidate.WorldMap,
						EventID:  candidate.EventID,
					},
				)
			}
		}
	}
	table := s.records.Table(records.Chapters)
	if table != nil && table.Err == nil {
		for _, chapter := range table.Rows {
			index, err := s.scriptIndex(chapter.ID)
			if err == nil {
				consider(index, false)
			}
		}
	}
	if s.WorldMapEventsAvailable() {
		index, err := s.scriptIndex(WorldMapScripts)
		if err == nil {
			consider(index, true)
		}
	}
	return result
}

func (s *Editor) ApplyScriptEdit(plan *ScriptPlan) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_script", fault.Context{Resource: scriptResource}) }()
	if plan == nil {
		return rom.ErrStale
	}
	if err = plan.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

// In-place edits share the session's claim guard with pointer updates, but
// authorize script payloads and their hosted operand sites as one resource.
func (s *Editor) scriptWriteRange(start, end int) error {
	if s.source.ProtectRange == nil {
		return edit.ErrLayout
	}
	if err := s.source.ProtectRange(
		s.image.Data,
		ScriptCodeOwner(),
		romwrite.Range{
			Start: start, End: end,
		},
	); err != nil {
		return fault.New(
			edit.ErrLayout,
			"game.protect_script_range",
			fault.Context{
				Resource: scriptResource,
				Range: &fault.Range{
					Offset: start,
					Length: end - start,
					Size:   len(s.image.Data),
				},
			},
			err,
		)
	}
	return nil
}

// scriptContent packs known rows and retargets declared calls into the same
// resource. Generic slot values cannot establish an internal pointer contract.
func (d *ScriptDraft) scriptContent(
	destination int,
	structural bool,
) ([]byte, []int, error) {
	var content []byte
	positions := make([]int, len(d.rows))
	addresses := make(map[int]int)
	for index, row := range d.rows {
		positions[index] = destination + len(content)
		if row.instruction.Address > 0 {
			addresses[row.instruction.Address] = positions[index]
		}
		content = append(content, row.instruction.Raw...)
	}
	if len(content) == 0 {
		return nil, nil, ErrScript
	}
	if !structural {
		return content, positions, nil
	}
	entry := d.Entry()
	end := entry.Address + d.originalLength()
	loadSources := d.unitLoadSources()
	for index, row := range d.rows {
		if row.definition == nil {
			return nil, nil, ErrScriptStructure
		}
		for _, field := range row.definition.fields {
			value := field.value(row.instruction.Raw)
			if field.Target == ScriptDeploymentTarget &&
				d.original.format.scripts.trackSlots &&
				slotUnitLoad(row.instruction.Raw) {
				source, ok := loadSources[index]
				if !ok {
					return nil, nil, ErrScriptStructure
				}
				value = int64(gba.Uint32(d.rows[source].instruction.Raw, 4))
			}
			if field.Optional && value == 0 {
				continue
			}
			if (field.Target == ScriptEventTarget || field.Target == ScriptDeploymentTarget || field.Target == ScriptMovementTarget) &&
				(value < 0x080000C0 || value >= 0x0A000000) {
				return nil, nil, ErrScriptStructure
			}
			if field.ID == ScriptSlotValue &&
				value >= int64(0x08000000+entry.Address) &&
				value < int64(0x08000000+end) {
				return nil, nil, ErrScriptStructure
			}
			if field.Target != ScriptEventTarget {
				continue
			}
			target := int(value) - 0x08000000
			if target < entry.Address || target >= end {
				continue
			}
			relocated, exists := addresses[target]
			if !exists {
				return nil, nil, ErrScript
			}
			field.put(
				content[positions[index]-destination:],
				int64(0x08000000+relocated),
			)
		}
	}
	return content, positions, nil
}
