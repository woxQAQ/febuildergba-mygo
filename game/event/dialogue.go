package event

import (
	"bytes"
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// Dialog selects the text for a newly created ordinary talk. Existing scripts
// keep their own operands and are edited through their referenced resources.
func (d *ConditionDraft) Dialog(index int) (uint16, bool) {
	if index < 0 || index >= len(d.records) || d.records[index].message == nil {
		return 0, false
	}
	return *d.records[index].message, true
}

func (d *ConditionDraft) SetDialogue(index int, id uint16) error {
	if _, ok := d.Dialog(index); !ok {
		return edit.ErrField
	}
	if id >= 0x7FFF {
		return edit.ErrValue
	}
	*d.records[index].message = id
	return nil
}

// DialogueText requires an existing, readable ROM text entry from the primary
// table. Native controls may remain read-only in the text editor without
// preventing a script from referencing the original encoded text.
func (s *Editor) DialogueText(id uint16) (result text.Value, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.dialogue_text",
			fault.Context{
				Resource:   text.Resource,
				ResourceID: new(int(id)),
			},
		)
	}()
	s.Refresh()
	table, err := s.text.Table(s.image.Data)
	if err != nil {
		return result, err
	}
	if table.Recovered || id == 0 || int(id) >= table.Count {
		return result, text.ErrText
	}
	return s.text.Read(s.image.Data, id)
}

func (f Format) dialogueScript(id uint16) []byte {
	raw := bytes.Clone(f.scripts.dialog)
	binary.LittleEndian.PutUint16(raw[f.scripts.textOffset:], id)
	return raw
}

// Text discovery uses confirmed direct operands and the same bounded control
// flow as deployment discovery. Slot-based text is included only when its
// immediate value is known; other instructions and script bytes stay untouched.
func (s *deploymentScanner) collectTexts(at int, slots *eventSlotState) {
	if s.texts == nil {
		return
	}
	var ids []uint32
	if s.format.scripts.trackSlots {
		op := s.data[at]
		if (op == 0x20 || op == 0x21) && s.data[at+1] == 0x1B {
			id := uint32(gba.Uint16(s.data, at+2))
			if id >= 0x8000 {
				if site := slots.slots[2]; site > 0 {
					id = gba.Uint32(s.data, site)
				} else {
					s.warn(at)
					return
				}
			}
			ids = append(ids, id)
		}
	} else {
		for _, offset := range s.format.scripts.flow[s.data[at]].texts {
			ids = append(ids, gba.Uint32(s.data, at+offset))
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
