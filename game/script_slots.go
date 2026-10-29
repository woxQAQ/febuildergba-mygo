package game

const (
	ScriptSlotAdd        ScriptOp = "script_slot_add"
	ScriptSlotSubtract   ScriptOp = "script_slot_subtract"
	ScriptSlotMultiply   ScriptOp = "script_slot_multiply"
	ScriptSlotDivide     ScriptOp = "script_slot_divide"
	ScriptSlotRemainder  ScriptOp = "script_slot_remainder"
	ScriptSlotAnd        ScriptOp = "script_slot_and"
	ScriptSlotOr         ScriptOp = "script_slot_or"
	ScriptSlotXor        ScriptOp = "script_slot_xor"
	ScriptSlotShiftLeft  ScriptOp = "script_slot_shift_left"
	ScriptSlotShiftRight ScriptOp = "script_slot_shift_right"
)

func (f deploymentFormat) slotDefinitions() []scriptDefinition {
	if f.kind != eventFE8 {
		return nil
	}
	var result []scriptDefinition
	for index, op := range []ScriptOp{
		ScriptSlotAdd, ScriptSlotSubtract, ScriptSlotMultiply, ScriptSlotDivide,
		ScriptSlotRemainder, ScriptSlotAnd, ScriptSlotOr, ScriptSlotXor,
		ScriptSlotShiftLeft, ScriptSlotShiftRight,
	} {
		fields := []ScriptField{
			scriptOperand(ScriptSlot, 2, 2, eventSlotCount-1),
			scriptOperand(ScriptSlotA, 2, 2, eventSlotCount-1),
			scriptOperand(ScriptSlotB, 2, 2, eventSlotCount-1),
		}
		for i := range fields {
			fields[i].bits = 4
			fields[i].shift = uint(i * 4)
		}
		result = append(
			result,
			scriptDefinition{
				op:     op,
				raw:    []byte{0x20 + byte(index), 6, 0x1C, 2},
				fields: fields,
				help:   "script_slot_operation_help",
			},
		)
	}
	return result
}

// Arithmetic may copy a literal's value without changing its ownership. A
// computed, pointer-shaped result never becomes a rewritable literal operand.
func (s *eventSlotState) observeArithmetic(raw []byte) {
	packed := u16(raw, 2)
	destination, left, right := int(
		packed&15,
	), int(
		(packed>>4)&15,
	), int(
		(packed>>8)&15,
	)
	if destination >= eventSlotCount || left >= eventSlotCount ||
		right >= eventSlotCount {
		*s = eventSlotState{}
		return
	}
	source := 0
	switch raw[0] {
	case 0x20,
		0x26,
		0x27: // Addition, OR and XOR with the interpreter's zero slot.
		if left == 0 {
			source = s.slots[right]
		} else if right == 0 {
			source = s.slots[left]
		}
		if raw[0] == 0x26 && left == right {
			source = s.slots[left]
		}
	case 0x21,
		0x28,
		0x29: // Subtracting or shifting by zero preserves the value.
		if right == 0 {
			source = s.slots[left]
		}
	case 0x25: // AND with itself preserves the value.
		if left == right {
			source = s.slots[left]
		}
	}
	s.slots[destination] = source
	if destination == 0xD {
		s.slots[destination] = 0
		s.queueKnown = false
	}
}
