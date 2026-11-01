package event

import (
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

type ScriptOp string

const (
	ScriptUnknown       ScriptOp = "script_unknown"
	ScriptASM           ScriptOp = "script_asm"
	ScriptEnd           ScriptOp = "script_end"
	ScriptWait          ScriptOp = "script_wait"
	ScriptText          ScriptOp = "script_text"
	ScriptMoreText      ScriptOp = "script_more_text"
	ScriptTextStart     ScriptOp = "script_text_start"
	ScriptTextWait      ScriptOp = "script_text_wait"
	ScriptTextClear     ScriptOp = "script_text_clear"
	ScriptLoadUnits     ScriptOp = "script_load_units"
	ScriptWaitUnits     ScriptOp = "script_wait_units"
	ScriptFlagOn        ScriptOp = "script_flag_on"
	ScriptFlagOff       ScriptOp = "script_flag_off"
	ScriptCheckFlag     ScriptOp = "script_check_flag"
	ScriptLabel         ScriptOp = "script_label"
	ScriptGoto          ScriptOp = "script_goto"
	ScriptFlagBranchOn  ScriptOp = "script_branch_flag_on"
	ScriptFlagBranchOff ScriptOp = "script_branch_flag_off"
	ScriptCall          ScriptOp = "script_call"
	ScriptJump          ScriptOp = "script_jump"
	ScriptSetSlot       ScriptOp = "script_set_slot"
	ScriptEqual         ScriptOp = "script_branch_eq"
	ScriptNotEqual      ScriptOp = "script_branch_ne"
	ScriptGreaterEqual  ScriptOp = "script_branch_ge"
	ScriptGreater       ScriptOp = "script_branch_gt"
	ScriptLessEqual     ScriptOp = "script_branch_le"
	ScriptLess          ScriptOp = "script_branch_lt"
)

const (
	ScriptTextID      = "script_text_id"
	ScriptFlagID      = "script_flag_id"
	ScriptFrames      = "script_frames"
	ScriptLabelID     = "script_label_id"
	ScriptPointer     = "script_pointer"
	ScriptUnitPointer = "script_unit_pointer"
	ScriptSlot        = "script_slot"
	ScriptSlotValue   = "script_slot_value"
	ScriptSlotA       = "script_slot_a"
	ScriptSlotB       = "script_slot_b"
)

// ScriptTarget is the resource contract of an operand, independent of its
// display name. One instruction may refer to several resources of the same kind.
type ScriptTarget uint8

const (
	ScriptNoTarget ScriptTarget = iota
	ScriptTextTarget
	ScriptEventTarget
	ScriptDeploymentTarget
	ScriptMovementTarget
	ScriptMapChangeTarget
)

// ScriptField describes a confirmed operand; the opcode and unrelated bytes
// remain outside its writable range. Pointer operands use native GBA addresses.
// A negative Min denotes a signed native operand. Reference supplies optional
// record choices; Help identifies the operand's version-specific explanation.
type ScriptField struct {
	ID            string
	Min, Max      int64
	Hex           bool
	Reference     records.Kind
	Target        ScriptTarget
	Optional      bool
	Help          string
	offset, width int
	bits, shift   uint
	indirect      *operandRange
}

type operandRange struct{ min, max int64 }

func (field ScriptField) isIndirect(value int64) bool {
	return field.indirect != nil && value >= field.indirect.min &&
		value <= field.indirect.max
}

func (field ScriptField) mask() int64 {
	bits := field.bits
	if bits == 0 {
		bits = uint(field.width * 8)
	}
	return (int64(1)<<bits - 1) << field.shift
}

func (field ScriptField) value(raw []byte) int64 {
	var result int64
	for index := range field.width {
		result |= int64(raw[field.offset+index]) << (8 * index)
	}
	result = (result & field.mask()) >> field.shift
	sign := ((field.mask() >> field.shift) + 1) >> 1
	if field.Min < 0 && result&sign != 0 {
		result -= sign << 1
	}
	return result
}

func (field ScriptField) put(raw []byte, value int64) {
	mask := field.mask()
	for index := range field.width {
		shift := uint(8 * index)
		part := byte((mask >> shift) & 0xFF)
		raw[field.offset+index] = raw[field.offset+index]&^part |
			byte(((value<<field.shift)>>shift)&0xFF)&part
	}
}

type scriptDefinition struct {
	op            ScriptOp
	raw           []byte
	fields        []ScriptField
	label, branch bool
	terminal      bool
	help          string
}

func scriptOperand(id string, offset, width int, maximum int64) ScriptField {
	var target ScriptTarget
	switch id {
	case ScriptTextID:
		target = ScriptTextTarget
	case ScriptPointer:
		target = ScriptEventTarget
	case ScriptUnitPointer:
		target = ScriptDeploymentTarget
	}
	return ScriptField{
		ID:     id,
		Target: target,
		offset: offset,
		width:  width,
		Max:    maximum,
		Hex:    id != ScriptFrames,
	}
}

func (f Format) scriptDefinitions() []scriptDefinition { return f.scripts.definitions }

type coreDefinitions []scriptDefinition

func (result *coreDefinitions) add(
	op ScriptOp,
	raw []byte,
	fields ...ScriptField,
) {
	*result = append(
		*result,
		scriptDefinition{
			op:     op,
			raw:    raw,
			fields: fields,
			label:  op == ScriptLabel,
			branch: op == ScriptGoto || op == ScriptFlagBranchOn ||
				op == ScriptFlagBranchOff ||
				op == ScriptEqual ||
				op == ScriptNotEqual ||
				op == ScriptGreaterEqual ||
				op == ScriptGreater ||
				op == ScriptLessEqual ||
				op == ScriptLess,
			terminal: op == ScriptEnd || op == ScriptJump,
		},
	)
}

func fe8CoreDefinitions() []scriptDefinition {
	var result coreDefinitions
	add := result.add
	add(ScriptEnd, []byte{0x20, 1, 0, 0})
	add(
		ScriptWait,
		[]byte{0x20, 0x0E, 1, 0},
		sceneOperand(ScriptFrames, 2, 2, 1, 0x7FFF),
	)
	add(ScriptTextStart, []byte{0x20, 0x1A, 0, 0})
	add(
		ScriptText,
		[]byte{0x20, 0x1B, 1, 0},
		scriptOperand(ScriptTextID, 2, 2, 0xFFFF),
	)
	add(
		ScriptMoreText,
		[]byte{0x21, 0x1B, 1, 0},
		scriptOperand(ScriptTextID, 2, 2, 0xFFFF),
	)
	add(ScriptTextWait, []byte{0x20, 0x1D, 0, 0})
	add(ScriptTextClear, []byte{0x22, 0x1B, 0, 0})
	add(ScriptWaitUnits, []byte{0x20, 0x30, 0, 0})
	add(
		ScriptFlagOn,
		[]byte{0x29, 2, 0, 0},
		scriptOperand(ScriptFlagID, 2, 2, 0xFFFF),
	)
	add(
		ScriptFlagOff,
		[]byte{0x21, 2, 0, 0},
		scriptOperand(ScriptFlagID, 2, 2, 0xFFFF),
	)
	add(
		ScriptCheckFlag,
		[]byte{0x21, 3, 0, 0},
		scriptOperand(ScriptFlagID, 2, 2, 0xFFFF),
	)
	add(
		ScriptLabel,
		[]byte{0x20, 8, 0, 0},
		scriptOperand(ScriptLabelID, 2, 2, 0xFFFF),
	)
	add(
		ScriptGoto,
		[]byte{0x20, 9, 0, 0},
		scriptOperand(ScriptLabelID, 2, 2, 0xFFFF),
	)
	add(
		ScriptCall,
		[]byte{0x40, 0x0A, 0, 0, 0, 0, 0, 0},
		scriptOperand(ScriptPointer, 4, 4, 0xFFFFFFFF),
	)
	add(
		ScriptSetSlot,
		[]byte{0x40, 5, 0, 0, 0, 0, 0, 0},
		scriptOperand(ScriptSlot, 2, 2, eventSlotCount-1),
		scriptOperand(ScriptSlotValue, 4, 4, 0xFFFFFFFF),
	)
	result[len(result)-1].help = "script_slot_assignment_help"
	for sub, op := range []ScriptOp{ScriptEqual, ScriptNotEqual, ScriptGreaterEqual, ScriptGreater, ScriptLessEqual, ScriptLess} {
		add(
			op,
			[]byte{0x40 + byte(sub), 0x0C, 0, 0, 0, 0, 0, 0},
			scriptOperand(ScriptLabelID, 2, 2, 0xFFFF),
			scriptOperand(ScriptSlotA, 4, 2, eventSlotCount-1),
			scriptOperand(ScriptSlotB, 6, 2, eventSlotCount-1),
		)
	}
	return result
}

func fe6CoreDefinitions() []scriptDefinition {
	var result coreDefinitions
	add := result.add
	words := scriptWords
	add(ScriptEnd, words(0))
	add(ScriptWait, words(2, 0), scriptOperand(ScriptFrames, 4, 4, 0xFFFF))
	add(ScriptText, words(7, 1), scriptOperand(ScriptTextID, 4, 2, 0xFFFF))
	add(
		ScriptMoreText,
		words(8, 1),
		scriptOperand(ScriptTextID, 4, 2, 0xFFFF),
	)
	add(ScriptTextClear, words(5))
	add(
		ScriptFlagOn,
		words(0x34, 0),
		scriptOperand(ScriptFlagID, 4, 4, 0xFFFF),
	)
	add(
		ScriptFlagOff,
		words(0x35, 0),
		scriptOperand(ScriptFlagID, 4, 4, 0xFFFF),
	)
	add(
		ScriptJump,
		words(0x25, 0),
		scriptOperand(ScriptPointer, 4, 4, 0xFFFFFFFF),
	)
	return append(
		result,
		wordFlowDefinitions(0x1B, 0x1C, 0x22, 0x23, 0x12, 0x14)...)
}

func fe7CoreDefinitions(shift, call uint32) []scriptDefinition {
	var result coreDefinitions
	add := result.add
	words := scriptWords
	flagOn, flagOff := uint32(0x74)+shift, uint32(0x75)+shift
	add(ScriptEnd, words(0))
	add(ScriptWait, words(2), scriptOperand(ScriptFrames, 2, 2, 0xFFFF))
	add(
		ScriptText,
		words(0x0D, 1),
		scriptOperand(ScriptTextID, 4, 2, 0xFFFF),
	)
	add(
		ScriptMoreText,
		words(0x11, 1),
		scriptOperand(ScriptTextID, 4, 2, 0xFFFF),
	)
	add(ScriptTextClear, words(9))
	add(
		ScriptFlagOn,
		words(flagOn),
		scriptOperand(ScriptFlagID, 2, 2, 0xFFFF),
	)
	add(
		ScriptFlagOff,
		words(flagOff),
		scriptOperand(ScriptFlagID, 2, 2, 0xFFFF),
	)
	add(
		ScriptCall,
		words(call, 0),
		scriptOperand(ScriptPointer, 4, 4, 0xFFFFFFFF),
	)
	return append(
		result,
		wordFlowDefinitions(
			0x42+shift,
			0x43+shift,
			0x4A+shift,
			0x4B+shift,
			0x30+shift,
			0x39+shift,
		)...)
}

func wordFlowDefinitions(
	label, goTo, branchOn, branchOff, load, wait uint32,
) []scriptDefinition {
	var result coreDefinitions
	add := result.add
	words := scriptWords
	add(
		ScriptLoadUnits,
		words(load, 0),
		scriptOperand(ScriptUnitPointer, 4, 4, 0xFFFFFFFF),
	)
	add(ScriptWaitUnits, words(wait))
	add(
		ScriptLabel,
		words(label, 0),
		scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF),
	)
	add(
		ScriptGoto,
		words(goTo, 0),
		scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF),
	)
	add(
		ScriptFlagBranchOn,
		words(branchOn, 0, 0),
		scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF),
		scriptOperand(ScriptFlagID, 8, 4, 0xFFFF),
	)
	add(
		ScriptFlagBranchOff,
		words(branchOff, 0, 0),
		scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF),
		scriptOperand(ScriptFlagID, 8, 4, 0xFFFF),
	)
	return result
}

func (definition scriptDefinition) matches(raw []byte) bool {
	if len(raw) != len(definition.raw) {
		return false
	}
	for index, value := range raw {
		var mask byte
		for _, field := range definition.fields {
			if index >= field.offset && index < field.offset+field.width {
				mask |= byte(
					(field.mask() >> uint(8*(index-field.offset))) & 0xFF,
				)
			}
		}
		if value&^mask != definition.raw[index]&^mask {
			return false
		}
	}
	return true
}
