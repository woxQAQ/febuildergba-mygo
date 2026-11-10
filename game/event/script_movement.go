package event

import (
	"bytes"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

const (
	ScriptMoveUnitPath            ScriptOp = "script_move_unit_path"
	ScriptMovePositionPath        ScriptOp = "script_move_position_path"
	ScriptMoveFactionPaths        ScriptOp = "script_move_faction_paths"
	ScriptMoveFactionPositionPath ScriptOp = "script_move_faction_position_path"
	ScriptMovementPointer                  = "script_movement_pointer"
	ScriptMovementPlayer                   = "script_movement_player"
	ScriptMovementOther                    = "script_movement_other"
	// MaxScriptMovementBytes is the native MU command buffer, including operands
	// and its final HALT. FE8 deployment REDA records use a different format.
	MaxScriptMovementBytes = 64
)

// ScriptMovementStep describes one native FE6/FE7 MU command. Value is used by
// sleep (9) and speed (12); other commands have no operand.
type ScriptMovementStep struct {
	Command byte
	Value   int
}

func wordMovementDefinitions(
	unit, position, faction uint32,
) []scriptDefinition {
	path := func(id string, offset int) ScriptField {
		field := scriptOperand(id, offset, 4, 0xFFFFFFFF)
		field.Target = ScriptMovementTarget
		return field
	}
	character := scriptOperand(ScriptCharacter, 4, 2, 0xFF)
	character.Reference = records.Units
	result := []scriptDefinition{
		{
			op:     ScriptMoveUnitPath,
			raw:    scriptWords(unit, 1, 0),
			fields: []ScriptField{character, path(ScriptMovementPointer, 8)},
			help:   "script_movement_help",
		},
		{
			op:  ScriptMovePositionPath,
			raw: scriptWords(position, 0, 0),
			fields: []ScriptField{
				sceneOperand(ScriptX, 4, 2, 0, 0x7FFF),
				sceneOperand(ScriptY, 6, 2, 0, 0x7FFF),
				path(ScriptMovementPointer, 8),
			},
			help: "script_movement_help",
		},
	}
	if faction == 0 {
		return result
	}
	code := faction
	return append(
		result,
		scriptDefinition{
			op:  ScriptMoveFactionPositionPath,
			raw: scriptWords(code, 1, 0, 0),
			fields: []ScriptField{
				character,
				scriptOperand(ScriptSpeed, 6, 2, 0xFFFF),
				sceneOperand(ScriptX, 8, 2, -0x8000, 0x7FFF),
				sceneOperand(ScriptY, 10, 2, -0x8000, 0x7FFF),
				path(ScriptMovementOther, 12),
			},
			help: "script_movement_faction_position_help",
		},
		scriptDefinition{
			op:  ScriptMoveFactionPaths,
			raw: scriptWords(code+1, 1, 0, 0),
			fields: []ScriptField{
				character,
				path(ScriptMovementPlayer, 8),
				path(ScriptMovementOther, 12),
			},
			help: "script_movement_faction_help",
		},
	)
}

func readScriptMovement(
	data []byte,
	address int,
) ([]ScriptMovementStep, error) {
	if address < 0xC0 || address > len(data)-MaxScriptMovementBytes {
		return nil, ErrScriptStructure
	}
	raw := data[address : address+MaxScriptMovementBytes]
	var steps []ScriptMovementStep
	for at := 0; at < len(raw); at++ {
		step := ScriptMovementStep{Command: raw[at]}
		if step.Command > 14 || step.Command == 11 {
			return nil, ErrScriptStructure
		}
		if step.Command == 9 || step.Command == 12 {
			at++
			if at == len(raw) {
				return nil, ErrScriptStructure
			}
			step.Value = int(raw[at])
		}
		steps = append(steps, step)
		if step.Command == 4 {
			return steps, nil
		}
	}
	return nil, ErrScriptStructure
}

func packScriptMovement(steps []ScriptMovementStep) ([]byte, error) {
	if len(steps) == 0 || len(steps) > MaxScriptMovementBytes {
		return nil, edit.ErrValue
	}
	var raw []byte
	for index, step := range steps {
		if step.Command > 14 || step.Command == 11 ||
			((step.Command == 4) != (index == len(steps)-1)) {
			return nil, edit.ErrValue
		}
		raw = append(raw, step.Command)
		switch step.Command {
		case 9, 12:
			// The coordinate scanner treats a speed byte as another opcode. Values
			// above its command range preserve the destination calculated by movement.
			if step.Value < 1 || step.Value > 127 ||
				(step.Command == 12 && step.Value < 16) {
				return nil, edit.ErrValue
			}
			raw = append(raw, byte(step.Value))
		default:
			if step.Value != 0 {
				return nil, edit.ErrValue
			}
		}
	}
	if len(raw) > MaxScriptMovementBytes {
		return nil, edit.ErrValue
	}
	return append(
		raw,
		bytes.Repeat([]byte{4}, MaxScriptMovementBytes-len(raw))...), nil
}

func (d *ScriptDraft) Movement(
	index int,
	id string,
) ([]ScriptMovementStep, error) {
	for _, field := range d.Fields(index) {
		if field.ID != id || field.Target != ScriptMovementTarget {
			continue
		}
		if steps, ok := d.rows[index].movements[id]; ok {
			return slices.Clone(steps), nil
		}
		value := field.value(d.rows[index].instruction.Raw)
		if value < 0x080000C0 || value >= 0x0A000000 {
			return nil, ErrScriptStructure
		}
		return readScriptMovement(d.source, int(value)-0x08000000)
	}
	return nil, edit.ErrField
}

// SetMovement stages a private replacement for this operand. Shared old paths
// are retained; path allocation and the instruction update use one script edit.
func (d *ScriptDraft) SetMovement(
	index int,
	id string,
	steps []ScriptMovementStep,
) error {
	if d.ReadOnly() {
		return ErrScriptStructure
	}
	for _, field := range d.Fields(index) {
		if field.ID != id || field.Target != ScriptMovementTarget {
			continue
		}
		if _, err := packScriptMovement(steps); err != nil {
			return err
		}
		row := &d.rows[index]
		value := field.value(row.instruction.Raw)
		if value >= 0x080000C0 && value < 0x0A000000 {
			original, err := readScriptMovement(d.source, int(value)-0x08000000)
			if err == nil && slices.Equal(original, steps) {
				delete(row.movements, id)
				return nil
			}
		}
		if row.movements == nil {
			row.movements = make(map[string][]ScriptMovementStep)
		}
		row.movements[id] = slices.Clone(steps)
		return nil
	}
	return edit.ErrField
}

func (d *ScriptDraft) prepareMovements(
	builder *romwrite.Builder,
) (*ScriptDraft, int, error) {
	prepared := *d
	prepared.rows = cloneScriptRows(d.rows)
	movements := 0
	for index := range prepared.rows {
		row := &prepared.rows[index]
		for _, field := range prepared.Fields(index) {
			steps, ok := row.movements[field.ID]
			if !ok {
				continue
			}
			raw, err := packScriptMovement(steps)
			if err != nil {
				return nil, 0, err
			}
			destination, err := builder.Append(raw)
			if err != nil {
				return nil, 0, err
			}
			field.put(
				row.instruction.Raw,
				int64(0x08000000+destination),
			)
			movements++
		}
		row.movements = nil
	}
	return &prepared, movements, nil
}
