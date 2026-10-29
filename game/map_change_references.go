package game

import (
	"bytes"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

var (
	ErrMapChangeContext = fault.Code{
		ID:   "game.map_change_context",
		Kind: fault.Unsupported,
	}
	ErrMapChangeLimit = fault.Code{
		ID:   "game.map_change_limit",
		Kind: fault.Capacity,
	}
)

// MapChangeLink is a confirmed event target in its current chapter context.
type MapChangeLink struct{ Chapter, ID, Address int }

// MapChangeReference compares the record selected before and after a table edit.
// Before/After are record indexes (-1 means absent), never event IDs.
type MapChangeReference struct {
	Chapter, ScriptChapter, Pointer, Address, Instruction int
	ID, AfterID                                           int
	Position                                              image.Point
	ByPosition, Retargeted                                bool
	Before, After                                         int
}

type (
	MapChangeWarning struct {
		Chapter, Address int
		Err              error
	}
	MapChangeReferences struct {
		References []MapChangeReference
		Warnings   []MapChangeWarning
	}
)

type (
	mapOperandState struct {
		chapter int
		slots   eventSlotState
	}
	mapOperand struct {
		id         int
		point      image.Point
		byPosition bool
	}
)

// Context is tracked through confirmed linear instructions. A label, branch,
// call or unknown effect invalidates it; an explicit map load can establish it
// again. No caller's map is assumed for a separately discovered called script.
func mapOperandStates(
	entry ScriptEntry,
	format deploymentFormat,
	rows []scriptRow,
) []mapOperandState {
	state := mapOperandState{chapter: entry.Chapter}
	if entry.Source == "script_call_entry" {
		state.chapter = -1
	}
	result := make([]mapOperandState, len(rows))
	for index, row := range rows {
		result[index] = state
		scanner := deploymentScanner{format: format, data: row.instruction.Raw}
		command, valid := scanner.instruction(0)
		switch {
		case valid && command.mapID >= 0:
			state.chapter = command.mapID
		case !valid || command.chapterEnd || row.definition == nil || row.call >= 0 || row.branch >= 0 || row.label >= 0:
			state.chapter = -1
		}
		if format.kind == eventFE8 {
			state.slots.observe(row.instruction.Raw, index+1)
		}
	}
	return result
}

func mapOperands(
	row scriptRow,
	state mapOperandState,
	rows []scriptRow,
	format deploymentFormat,
) ([]mapOperand, error) {
	if !mapChangeOperation(row.instruction.Op) {
		return nil, ErrField
	}
	if state.chapter < 0 {
		return nil, ErrMapChangeContext
	}
	values := make(map[string]int64)
	for _, field := range row.definition.fields {
		values[field.ID] = field.value(row.instruction.Raw)
	}
	if row.instruction.Op == ScriptMapChangePosition {
		return []mapOperand{
			{
				byPosition: true,
				point: image.Pt(
					int(values[ScriptX]),
					int(values[ScriptY]),
				),
			},
		}, nil
	}
	value, found := values[ScriptMapChangeID]
	if !found {
		return nil, ErrMapChangeContext
	}
	if value >= 0 {
		if value > 127 {
			return nil, ErrMapChanges
		}
		return []mapOperand{{id: int(value)}}, nil
	}
	if format.kind != eventFE8 {
		return nil, ErrMapChangeContext
	}
	literal := func(source int) (uint32, bool) {
		if source <= 0 || source > len(rows) {
			return 0, false
		}
		raw := rows[source-1].instruction.Raw
		if len(raw) != 8 || raw[0] != 0x40 || raw[1] != 5 {
			return 0, false
		}
		return u32(raw, 4), true
	}
	switch value {
	case -1:
		position, known := literal(state.slots.slots[0xB])
		if !known {
			return nil, ErrMapChangeContext
		}
		return []mapOperand{
			{
				byPosition: true,
				point:      image.Pt(int(position&0xFFFF), int(position>>16)),
			},
		}, nil
	case -3:
		if !state.slots.queueKnown {
			return nil, ErrMapChangeContext
		}
		result := make([]mapOperand, 0, state.slots.count)
		for _, source := range state.slots.queue[:state.slots.count] {
			id, known := literal(source)
			if !known {
				return nil, ErrMapChangeContext
			}
			id &= 0xFFFF // Event27 consumes each queue item as a signed halfword.
			if id > 127 {
				return nil, ErrMapChanges
			}
			result = append(result, mapOperand{id: int(id)})
		}
		return result, nil
	}
	return nil, ErrMapChangeContext
}

func (s *Session) ScriptMapChapter(draft *ScriptDraft, index int) (int, error) {
	if draft == nil || !draft.Current(s) {
		return 0, rom.ErrStale
	}
	if index < 0 || index >= len(draft.rows) ||
		!mapChangeOperation(draft.rows[index].instruction.Op) {
		return 0, ErrField
	}
	state := mapOperandStates(draft.Entry(), draft.original.format, draft.rows)[index]
	if state.chapter < 0 {
		return 0, ErrMapChangeContext
	}
	return state.chapter, nil
}

func (s *Session) ScriptMapChanges(
	draft *ScriptDraft,
	index int,
) ([]MapChangeLink, error) {
	chapter, err := s.ScriptMapChapter(draft, index)
	if err != nil {
		return nil, err
	}
	state := mapOperandStates(draft.Entry(), draft.original.format, draft.rows)[index]
	operands, err := mapOperands(
		draft.rows[index],
		state,
		draft.rows,
		draft.original.format,
	)
	if err != nil {
		return nil, err
	}
	table, err := s.mapChanges(chapter)
	if err != nil {
		return nil, err
	}
	var result []MapChangeLink
	for _, operand := range operands {
		id := operand.id
		if operand.byPosition {
			id = mapChangeAt(table.rows, operand.point)
		}
		found := mapChangeIndex(table.rows, id)
		if found < 0 {
			if operand.byPosition && draft.original.format.kind != eventFE8 {
				return nil, ErrMapChangeContext
			}
			return nil, ErrMapChanges
		}
		result = append(
			result,
			MapChangeLink{
				Chapter: chapter,
				ID:      id,
				Address: table.rows[found].address,
			},
		)
	}
	return result, nil
}

func (s *Session) validateMapChangeScript(draft *ScriptDraft) (bool, error) {
	changed := false
	oldStates := mapOperandStates(
		draft.Entry(),
		draft.original.format,
		draft.original.rows,
	)
	states := mapOperandStates(draft.Entry(), draft.original.format, draft.rows)
	for index, row := range draft.rows {
		if !mapChangeOperation(row.instruction.Op) {
			continue
		}
		oldIndex := slices.IndexFunc(
			draft.original.rows,
			func(old scriptRow) bool { return old.instruction.ID == row.instruction.ID },
		)
		operands, err := mapOperands(
			row,
			states[index],
			draft.rows,
			draft.original.format,
		)
		if oldIndex >= 0 &&
			bytes.Equal(
				row.instruction.Raw,
				draft.original.rows[oldIndex].instruction.Raw,
			) {
			oldOperands, oldErr := mapOperands(
				draft.original.rows[oldIndex],
				oldStates[oldIndex],
				draft.original.rows,
				draft.original.format,
			)
			if states[index].chapter == oldStates[oldIndex].chapter &&
				slices.Equal(operands, oldOperands) &&
				fault.CodeOf(err) == fault.CodeOf(oldErr) {
				continue
			}
		}
		changed = true
		// Runtime selectors remain valid native operands. A direct resource
		// change requires a known chapter and a real target.
		if err != nil {
			if err == ErrMapChangeContext && states[index].chapter >= 0 {
				continue
			}
			return changed, err
		}
		if _, err = s.ScriptMapChanges(draft, index); err != nil {
			if err == ErrMapChangeContext && states[index].chapter >= 0 {
				continue
			}
			return changed, err
		}
	}
	return changed, nil
}

// Parameter edits can be shared by chapters with different change tables.
// A partial script alias cannot establish the context at the edited instruction.
func (s *Session) validateSharedMapChangeScript(
	draft *ScriptDraft,
	uses []ScriptUse,
	structural bool,
) error {
	changed, err := s.validateMapChangeScript(draft)
	if err != nil || !changed {
		return err
	}
	for _, use := range uses {
		if use.Chapter == draft.Entry().Chapter &&
			use.Pointer == draft.Entry().Pointer {
			continue
		}
		if !structural && use.Address != draft.Entry().Address {
			return ErrMapChangeContext
		}
		shared := *draft
		shared.original.Entry.Chapter = use.Chapter
		shared.original.Entry.Pointer = use.Pointer
		shared.original.Entry.Source = use.Source
		if _, err = s.validateMapChangeScript(&shared); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) MapChangeReferences(
	draft *MapChangesDraft,
) (MapChangeReferences, error) {
	if draft == nil || !draft.Current(s) {
		return MapChangeReferences{}, rom.ErrStale
	}
	format := s.Adapter.(gameFormat)
	uses, err := s.mapResourceUses(
		format,
		mapChangesField,
		draft.original.pointer,
		draft.original.address,
	)
	if err != nil {
		return MapChangeReferences{}, err
	}
	report := s.mapChangeReferences(draft, uses)
	if !draft.Current(s) {
		return MapChangeReferences{}, rom.ErrStale
	}
	return report, nil
}

func (d *MapChangesDraft) compareMapOperand(
	operand mapOperand,
) MapChangeReference {
	beforeID, afterID := operand.id, operand.id
	if operand.byPosition {
		beforeID = mapChangeAt(d.original.rows, operand.point)
		afterID = mapChangeAt(d.rows, operand.point)
	}
	before, after := mapChangeIndex(
		d.original.rows,
		beforeID,
	), mapChangeIndex(
		d.rows,
		afterID,
	)
	return MapChangeReference{
		ID:         beforeID,
		AfterID:    afterID,
		Position:   operand.point,
		ByPosition: operand.byPosition,
		Before:     before,
		After:      after,
		Retargeted: before >= 0 && after >= 0 &&
			d.original.rows[before].address != d.rows[after].address,
	}
}

func (s *Session) mapChangeReferences(
	draft *MapChangesDraft,
	uses []MapUse,
) MapChangeReferences {
	var report MapChangeReferences
	chapters := make(map[int]bool)
	for _, use := range uses {
		if use.Updated {
			chapters[use.Chapter] = true
		}
	}
	warn := func(chapter, address int, err error) {
		for _, old := range report.Warnings {
			if old.Chapter == chapter &&
				fault.CodeOf(old.Err) == fault.CodeOf(err) {
				return
			}
		}
		report.Warnings = append(
			report.Warnings,
			MapChangeWarning{Chapter: chapter, Address: address, Err: err},
		)
	}
	remaining := 65536
	for _, use := range uses {
		if !use.Updated {
			continue
		}
		entries, err := s.ScriptEntries(use.Chapter)
		if err != nil {
			warn(use.Chapter, 0, err)
			continue
		}
		seen := make(map[int]bool)
		for _, entry := range entries {
			if entry.Err != nil {
				warn(use.Chapter, entry.Pointer, entry.Err)
				continue
			}
			if entry.Address == 0 || seen[entry.Address] {
				continue
			}
			seen[entry.Address] = true
			program, err := s.Script(use.Chapter, entry.Pointer)
			if err != nil {
				warn(use.Chapter, entry.Pointer, err)
				continue
			}
			if !program.Complete {
				warn(use.Chapter, entry.Pointer, ErrMapChangeContext)
			}
			states := mapOperandStates(entry, program.format, program.rows)
			for index, row := range program.rows {
				remaining--
				if remaining < 0 {
					warn(
						use.Chapter,
						row.instruction.Address,
						ErrMapChangeLimit,
					)
					return report
				}
				if row.definition == nil || row.call >= 0 || row.branch >= 0 {
					warn(
						use.Chapter,
						row.instruction.Address,
						ErrMapChangeContext,
					)
				}
				if !mapChangeOperation(row.instruction.Op) {
					continue
				}
				state := states[index]
				if state.chapter >= 0 && !chapters[state.chapter] {
					continue
				}
				operands, err := mapOperands(
					row,
					state,
					program.rows,
					program.format,
				)
				if err != nil {
					warn(use.Chapter, row.instruction.Address, err)
					continue
				}
				for _, operand := range operands {
					if len(report.References) >= 4096 {
						warn(
							use.Chapter,
							row.instruction.Address,
							ErrMapChangeLimit,
						)
						return report
					}
					ref := draft.compareMapOperand(operand)
					ref.Chapter, ref.ScriptChapter = state.chapter, use.Chapter
					ref.Pointer, ref.Address, ref.Instruction = entry.Pointer, row.instruction.Address, index
					if operand.byPosition && program.format.kind != eventFE8 &&
						(ref.Before < 0 || ref.After < 0) {
						warn(
							use.Chapter,
							row.instruction.Address,
							ErrMapChangeContext,
						)
					}
					report.References = append(report.References, ref)
				}
			}
		}
	}
	return report
}
