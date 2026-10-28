package game

import (
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

// Dialog selects the text for a newly created ordinary talk. Existing scripts
// keep their own operands and are edited through their referenced resources.
func (d *EventDraft) Dialog(index int) (uint16, bool) {
	if index < 0 || index >= len(d.records) || d.records[index].message == nil {
		return 0, false
	}
	return *d.records[index].message, true
}

func (d *EventDraft) SetDialogue(index int, id uint16) error {
	if _, ok := d.Dialog(index); !ok {
		return ErrField
	}
	if id >= 0x7FFF {
		return ErrValue
	}
	*d.records[index].message = id
	return nil
}

// DialogueText requires an existing, readable ROM text entry from the primary
// table. Native controls may remain read-only in the text editor without
// preventing a script from referencing the original encoded text.
func (s *Session) DialogueText(id uint16) (result TextValue, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.dialogue_text",
			fault.Context{Resource: textResource, ResourceID: new(int(id))},
		)
	}()
	s.Refresh()
	table, err := s.Adapter.TextTable(s.image.Data)
	if err != nil {
		return result, err
	}
	if table.Recovered || id == 0 || int(id) >= table.Count {
		return result, ErrText
	}
	return s.Adapter.ReadText(s.image.Data, id)
}

func (f deploymentFormat) dialogueScript(id uint16) []byte {
	if f.kind == eventFE8 {
		return []byte{
			0x20,
			0x1A,
			0,
			0,
			0x20,
			0x1B,
			byte(id & 0xFF),
			byte(id >> 8),
			0x20,
			0x1D,
			0,
			0,
			0x22,
			0x1B,
			0,
			0,
			0x20,
			1,
			0,
			0,
		}
	}
	words := []uint32{0x0D, uint32(id), 0x09, 0x0A, 0}
	if f.kind == eventFE6 {
		words = []uint32{0x07, uint32(id), 0x05, 0x06, 0}
	}
	var result []byte
	for _, word := range words {
		result = binary.LittleEndian.AppendUint32(result, word)
	}
	return result
}

// Text discovery uses confirmed direct operands and the same bounded control
// flow as deployment discovery. Slot-based text is included only when its
// immediate value is known; other instructions and script bytes stay untouched.
func (s *deploymentScanner) collectTexts(at int, slots *[16]int) {
	if s.texts == nil {
		return
	}
	var ids []uint32
	op := s.data[at]
	switch s.format.kind {
	case eventFE8:
		if (op == 0x20 || op == 0x21) && s.data[at+1] == 0x1B {
			id := uint32(u16(s.data, at+2))
			if id >= 0x8000 {
				if site := slots[2]; site > 0 {
					id = u32(s.data, site)
				} else {
					s.warn(at)
					return
				}
			}
			ids = append(ids, id)
		}
	case eventFE6:
		if op == 0x07 || op == 0x08 {
			ids = append(ids, u32(s.data, at+4))
		}
	default:
		switch op {
		case 0x0D, 0x0E, 0x11:
			ids = append(ids, u32(s.data, at+4))
		case 0x0F, 0x12:
			ids = append(ids, u32(s.data, at+4), u32(s.data, at+8))
		}
	}
	for _, id := range ids {
		if id > 0 && id < 0x7FFF && !slices.Contains(*s.texts, uint16(id)) {
			*s.texts = append(
				*s.texts,
				uint16(id),
			) // #nosec G115 -- Native text IDs are bounded above.
		}
	}
}
