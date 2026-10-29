package game

// Native slots occupy 0 through D; the adjacent FIFO has 30 words.
const (
	eventSlotCount  = 14
	eventQueueLimit = 30
)

// Origins identify literal operands, not pointer-looking numbers. The scanner
// supplies ROM offsets; drafts supply row indices plus one. Zero is unknown.
type eventSlotState struct {
	slots      [eventSlotCount]int
	queue      [eventQueueLimit]int
	count      int
	queueKnown bool
}

// observe follows only literal assignment and FIFO transfer. Calls, labels,
// branches and unmodelled effects invalidate provenance instead of evaluating
// game state or merging control-flow paths. Slot 0 is reset by the interpreter
// before every instruction; slot D is the queue length, never a pointer source.
func (s *eventSlotState) observe(raw []byte, source int) {
	s.slots[0] = 0
	if len(raw) == 8 && raw[0] == 0x40 && raw[1] == 5 {
		slot := int(u16(raw, 2))
		if slot >= len(s.slots) {
			*s = eventSlotState{}
			return
		}
		s.slots[slot] = source
		if slot == 0xD {
			s.queueKnown, s.count = u32(raw, 4) == 0, 0
			s.slots[slot] = 0
		}
		return
	}
	if len(raw) == 4 && raw[1] == 6 && raw[0] >= 0x20 && raw[0] <= 0x29 {
		s.observeArithmetic(raw)
		return
	}
	if len(raw) == 4 && raw[1] == 7 && raw[0] >= 0x20 && raw[0] <= 0x22 {
		s.observeQueue(raw)
		return
	}
	if len(raw) == 8 && raw[1] == 0x2C && raw[0] >= 0x40 && raw[0] <= 0x42 {
		return
	}
	if s.observeKnownEffects(raw) {
		return
	}
	*s = eventSlotState{}
}

func (s *eventSlotState) observeKnownEffects(raw []byte) bool {
	if len(raw) == 4 {
		if (raw[1] == 0x19 && raw[0] >= 0x20 && raw[0] <= 0x2A) ||
			(raw[1] == 0x33 && raw[0] >= 0x20 && raw[0] <= 0x28) {
			s.slots[0xC] = 0
			return true
		}
		if ((raw[1] == 0x34 || raw[1] == 0x13) && raw[0] >= 0x20 && raw[0] <= 0x2F) ||
			(raw[1] == 0x1A && raw[0] >= 0x20 && raw[0] <= 0x25) {
			return true
		}
		switch u16(raw, 0) {
		case 0x0320, 0x0321, 0x0420, 0x0F20:
			s.slots[0xC] = 0
			return true
		case 0x0220, 0x0221, 0x0228, 0x0229,
			0x0E20, 0x0E21, 0x0E22, 0x0E23, 0x0F21, 0x0F22, 0x0F23,
			0x1020, 0x1120, 0x1A20,
			0x1220, 0x1420, 0x1421, 0x1520, 0x1521, 0x1620,
			0x1720, 0x1721, 0x1722, 0x1723,
			0x2620, 0x2621, 0x2628, 0x2629, 0x2720, 0x2721,
			0x3020, 0x3B20, 0x3B21, 0x3B22, 0x3B24, 0x3B25:
			return true
		}
	}
	return false
}

func (s *eventSlotState) observeQueue(raw []byte) {
	slot := int(u16(raw, 2))
	if raw[0] == 0x21 {
		slot = 1
	}
	if slot >= len(s.slots) {
		*s = eventSlotState{}
		return
	}
	s.slots[0xD] = 0
	if raw[0] != 0x22 {
		if s.queueKnown && s.count < len(s.queue) {
			s.queue[s.count] = s.slots[slot]
			s.count++
		} else {
			// An unproven index or overflow can overwrite adjacent slot state.
			*s = eventSlotState{}
		}
		return
	}
	value := 0
	if s.queueKnown && s.count > 0 {
		value = s.queue[0]
		s.count--
		copy(s.queue[:], s.queue[1:])
	} else {
		// An unknown/empty pop can underflow the native queue counter.
		*s = eventSlotState{}
	}
	s.slots[slot] = value
	if slot == 0xD {
		s.queueKnown = false
		s.slots[slot] = 0
	}
}

func slotUnitLoad(raw []byte) bool {
	return len(raw) == 8 && raw[1] == 0x2C && raw[0] >= 0x40 &&
		raw[0] <= 0x42 &&
		u32(raw, 4)&0x80000000 != 0
}

// unitLoadSources resolves each indirect LOAD to a local SVAL row. No source
// survives a label or call, even if the linear byte listing suggests a value.
func (d *ScriptDraft) unitLoadSources() map[int]int {
	result := make(map[int]int)
	if d.original.format.kind != eventFE8 {
		return result
	}
	var state eventSlotState
	for index, row := range d.rows {
		if row.definition == nil {
			state = eventSlotState{}
			continue
		}
		if slotUnitLoad(row.instruction.Raw) && state.slots[2] > 0 {
			result[index] = state.slots[2] - 1
		}
		state.observe(row.instruction.Raw, index+1)
	}
	return result
}
