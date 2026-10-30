package event

import (
	"bytes"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/gba"
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

type mapOperandState struct {
	chapter int
	slots   eventSlotState
}

// Context is tracked through confirmed linear instructions. A label, branch,
// call or unknown effect invalidates it; an explicit map load can establish it
// again. No caller's map is assumed for a separately discovered called script.
func mapOperandStates(
	entry ScriptEntry,
	format Format,
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
	format Format,
) ([]maps.Target, error) {
	if !mapChangeOperation(row.instruction.Op) {
		return nil, edit.ErrField
	}
	if state.chapter < 0 {
		return nil, ErrMapChangeContext
	}
	values := make(map[string]int64)
	for _, field := range row.definition.fields {
		values[field.ID] = field.value(row.instruction.Raw)
	}
	if row.instruction.Op == ScriptMapChangePosition {
		return []maps.Target{
			{
				ByPosition: true,
				Position: image.Pt(
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
			return nil, maps.ErrMapChanges
		}
		return []maps.Target{{ID: int(value)}}, nil
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
		return gba.Uint32(raw, 4), true
	}
	switch value {
	case -1:
		position, known := literal(state.slots.slots[0xB])
		if !known {
			return nil, ErrMapChangeContext
		}
		return []maps.Target{
			{
				ByPosition: true,
				Position:   image.Pt(int(position&0xFFFF), int(position>>16)),
			},
		}, nil
	case -3:
		if !state.slots.queueKnown {
			return nil, ErrMapChangeContext
		}
		result := make([]maps.Target, 0, state.slots.count)
		for _, source := range state.slots.queue[:state.slots.count] {
			id, known := literal(source)
			if !known {
				return nil, ErrMapChangeContext
			}
			id &= 0xFFFF // Event27 consumes each queue item as a signed halfword.
			if id > 127 {
				return nil, maps.ErrMapChanges
			}
			result = append(result, maps.Target{ID: int(id)})
		}
		return result, nil
	}
	return nil, ErrMapChangeContext
}

func (s *Editor) MapChapter(draft *ScriptDraft, index int) (int, error) {
	if draft == nil || !draft.Current(s) {
		return 0, rom.ErrStale
	}
	if index < 0 || index >= len(draft.rows) ||
		!mapChangeOperation(draft.rows[index].instruction.Op) {
		return 0, edit.ErrField
	}
	state := mapOperandStates(draft.Entry(), draft.original.format, draft.rows)[index]
	if state.chapter < 0 {
		return 0, ErrMapChangeContext
	}
	return state.chapter, nil
}

func (s *Editor) MapChanges(
	draft *ScriptDraft,
	index int,
) ([]maps.Link, error) {
	chapter, err := s.MapChapter(draft, index)
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
	var result []maps.Link
	for _, operand := range operands {
		change, err := s.source.ResolveMap(chapter, operand)
		if err != nil {
			if operand.ByPosition && draft.original.format.kind != eventFE8 {
				return nil, ErrMapChangeContext
			}
			return nil, err
		}
		result = append(
			result,
			maps.Link{
				Chapter: chapter,
				ID:      change.ID,
				Address: change.Address,
			},
		)
	}
	return result, nil
}

func (s *Editor) validateMapChangeScript(draft *ScriptDraft) (bool, error) {
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
		if _, err = s.MapChanges(draft, index); err != nil {
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
func (s *Editor) validateSharedMapChangeScript(
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

const (
	ScriptMapChange           ScriptOp = "script_map_change"
	ScriptMapRevert           ScriptOp = "script_map_revert"
	ScriptMapChangePosition   ScriptOp = "script_map_change_position"
	ScriptMapChangeInstant    ScriptOp = "script_map_change_instant"
	ScriptMapChangeQuiet      ScriptOp = "script_map_change_quiet"
	ScriptMapChangeWater      ScriptOp = "script_map_change_water"
	ScriptMapChangeParameter  ScriptOp = "script_map_change_parameter"
	ScriptMapInstantParameter ScriptOp = "script_map_instant_parameter"
	ScriptMapQuietParameter   ScriptOp = "script_map_quiet_parameter"
	ScriptMapWaterParameter   ScriptOp = "script_map_water_parameter"
	ScriptMapRefresh          ScriptOp = "script_map_refresh"
)

const (
	ScriptMapChangeID       = "script_map_change_id"
	ScriptMapRemovePrevious = "script_map_remove_previous"
	ScriptMapWaterShadows   = "script_map_water_shadows"
)

func (f Format) mapChangeDefinitions() []scriptDefinition {
	var result []scriptDefinition
	add := func(op ScriptOp, raw []byte, help string, fields ...ScriptField) {
		result = append(
			result,
			scriptDefinition{op: op, raw: raw, help: help, fields: fields},
		)
	}
	id := func(offset, width int, minimum int64) ScriptField {
		field := sceneOperand(ScriptMapChangeID, offset, width, minimum, 127)
		field.Target = ScriptMapChangeTarget
		return field
	}
	if f.kind == eventFE8 {
		add(
			ScriptMapChange,
			[]byte{0x20, 0x27, 0, 0},
			"script_fe8_map_change_help",
			id(2, 2, -3),
		)
		add(
			ScriptMapRevert,
			[]byte{0x21, 0x27, 0, 0},
			"script_fe8_map_revert_help",
			id(2, 2, -3),
		)
		return result
	}
	if f.kind == eventFE6 {
		add(
			ScriptMapChange,
			scriptWords(0x29, 0),
			"script_fe6_map_change_help",
			id(4, 4, -1),
		)
		add(
			ScriptMapChangePosition,
			scriptWords(0x2A, 0),
			"script_word_map_position_help",
			sceneOperand(ScriptX, 4, 1, 0, 255),
			sceneOperand(ScriptY, 5, 1, 0, 255),
		)
		return result
	}
	shift := uint32(0)
	if f.kind == eventFE7U {
		shift = 2
	}
	for _, command := range []struct {
		op, parameter ScriptOp
		code          uint32
		flag          bool
		help          string
	}{
		{ScriptMapChange, ScriptMapChangeParameter, 0x5D, true, "script_fe7_map_change_help"},
		{ScriptMapChangeInstant, ScriptMapInstantParameter, 0x5F, false, "script_fe7_map_instant_help"},
		{ScriptMapChangeQuiet, ScriptMapQuietParameter, 0x60, true, "script_fe7_map_quiet_help"},
		{ScriptMapChangeWater, ScriptMapWaterParameter, 0x62, true, "script_fe7_map_water_help"},
	} {
		code := command.code + shift
		add(
			command.parameter,
			scriptWords(code|0xFFFF0000),
			"script_map_parameter_help",
		)
		field := id(2, 2, 0)
		// IDs occupy 0..127. Other low-half bits are not a new writable contract.
		field.bits = 7
		fields := []ScriptField{field}
		if command.flag {
			remove := sceneOperand(ScriptMapRemovePrevious, 2, 2, 0, 1)
			remove.bits, remove.shift = 1, 15
			fields = append(fields, remove)
		}
		add(command.op, scriptWords(code), command.help, fields...)
	}
	add(
		ScriptMapChangePosition,
		scriptWords(0x5E+shift),
		"script_word_map_position_help",
		sceneOperand(ScriptX, 2, 1, 0, 255),
		sceneOperand(ScriptY, 3, 1, 0, 255),
	)
	add(
		ScriptMapRefresh,
		scriptWords(0x61+shift),
		"script_map_refresh_help",
		sceneOperand(ScriptMapWaterShadows, 2, 2, 0, 1),
	)
	return result
}

func (d *ScriptDraft) IsMapChange(index int) bool {
	return index >= 0 && index < len(d.rows) &&
		mapChangeOperation(d.rows[index].instruction.Op)
}

func mapChangeOperation(op ScriptOp) bool {
	switch op {
	case ScriptMapChange,
		ScriptMapRevert,
		ScriptMapChangePosition,
		ScriptMapChangeInstant,
		ScriptMapChangeQuiet,
		ScriptMapChangeWater,
		ScriptMapChangeParameter,
		ScriptMapInstantParameter,
		ScriptMapQuietParameter,
		ScriptMapWaterParameter:
		return true
	}
	return false
}
