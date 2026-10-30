package event

import (
	"bytes"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

const (
	ScriptLoadUnits2   ScriptOp = "script_load_units_2"
	ScriptLoadUnits3   ScriptOp = "script_load_units_3"
	ScriptEnqueueSlot  ScriptOp = "script_enqueue_slot"
	ScriptEnqueueSlot1 ScriptOp = "script_enqueue_slot_1"
	ScriptDequeueSlot  ScriptOp = "script_dequeue_slot"
	ScriptLoadPolicy            = "script_load_policy"
)

func (f Format) loadDefinitions() []scriptDefinition {
	if f.kind != eventFE8 {
		return nil
	}
	var result []scriptDefinition
	for index, entry := range []struct {
		op   ScriptOp
		help string
	}{
		{ScriptLoadUnits, "script_load1_help"},
		{ScriptLoadUnits2, "script_load2_help"},
		{ScriptLoadUnits3, "script_load3_help"},
	} {
		policy := sceneOperand(ScriptLoadPolicy, 2, 1, 0, 2)
		policy.Help = entry.help
		pointer := scriptOperand(ScriptUnitPointer, 4, 4, 0xFFFFFFFF)
		pointer.Help = "script_load_pointer_help"
		argument := byte(1)
		if index == 2 {
			argument = 0
		}
		result = append(result, scriptDefinition{
			op:     entry.op,
			raw:    []byte{0x40 + byte(index), 0x2C, argument, 0, 0, 0, 0, 0},
			fields: []ScriptField{policy, pointer},
			help:   entry.help,
		})
	}
	return append(
		result,
		scriptDefinition{
			op:  ScriptEnqueueSlot,
			raw: []byte{0x20, 7, 1, 0},
			fields: []ScriptField{
				scriptOperand(ScriptSlot, 2, 2, eventSlotCount-1),
			},
			help: "script_queue_push_help",
		},
		scriptDefinition{
			op:   ScriptEnqueueSlot1,
			raw:  []byte{0x21, 7, 0, 0},
			help: "script_queue_push1_help",
		},
		scriptDefinition{
			op:  ScriptDequeueSlot,
			raw: []byte{0x22, 7, 2, 0},
			fields: []ScriptField{
				scriptOperand(ScriptSlot, 2, 2, eventSlotCount-1),
			},
			help: "script_queue_pop_help",
		},
	)
}

// A changed literal or queue transfer must validate the deployment consumed by
// its LOAD, even though that LOAD's encoded slot marker has not changed.
func (s *Editor) validateSlotUnitLoads(
	d *ScriptDraft,
	data []byte,
	positions []int,
) error {
	if d.original.format.kind != eventFE8 {
		return nil
	}
	original := &ScriptDraft{original: d.original, rows: d.original.rows}
	oldSources, sources := original.unitLoadSources(), d.unitLoadSources()
	structural := d.structural()
	oldRows := make(map[uint64]int)
	for index, row := range original.rows {
		oldRows[row.instruction.ID] = index
	}
	for index, row := range d.rows {
		if row.definition == nil || !slotUnitLoad(row.instruction.Raw) {
			continue
		}
		source, resolved := sources[index]
		oldIndex, exists := oldRows[row.instruction.ID]
		oldSource, oldResolved := oldSources[oldIndex]
		if exists && !structural && resolved == oldResolved &&
			bytes.Equal(
				row.instruction.Raw,
				original.rows[oldIndex].instruction.Raw,
			) &&
			(!resolved || gba.Uint32(d.rows[source].instruction.Raw, 4) == gba.Uint32(original.rows[oldSource].instruction.Raw, 4)) {
			continue
		}
		if !resolved {
			return ErrScriptStructure
		}
		group := deployment.NewReader(data, d.original.format.units).Group(
			positions[source]+4,
			"script_units",
			0,
		)
		if group.Err != nil {
			return group.Err
		}
	}
	return nil
}
