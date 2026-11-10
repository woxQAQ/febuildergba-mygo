package event

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
)

// TextUse names the script operand source in the shared text usage list.
const TextUse = "event_script"

// TextUses indexes confirmed direct text operands from chapter entries, static
// calls and the independent world-map directory. Aliases retain their entry
// context; repeated visits to the same operand within that context collapse.
// Dynamic operands, opaque instructions, damaged entries and scan limits leave
// useful partial results. No script or ASM callback is executed.
func (s *Editor) TextUses(id uint16) ([]text.Use, bool) {
	s.Refresh()
	if s.context.Validate(s.image.Data) != nil {
		return nil, false
	}
	if s.textUses == nil {
		s.indexTextUses()
	}
	return slices.Clone(s.textUses[id]), s.textUsesComplete
}

// TextOperand is a confirmed direct text ID at one named instruction field.
type TextOperand struct {
	Field string
	ID    uint16
}

func scriptTextOperands(row scriptRow) ([]TextOperand, bool) {
	if row.definition == nil || row.instruction.Op == ScriptASM {
		return nil, false
	}
	var result []TextOperand
	complete := true
	for _, field := range row.definition.fields {
		if field.Target != ScriptTextTarget {
			continue
		}
		value := field.value(row.instruction.Raw)
		if field.isIndirect(value) || value < 0 || value > 0xFFFF {
			complete = false
			continue
		}
		if value != 0 {
			// #nosec G115 -- Direct text operands were bounded above.
			result = append(
				result,
				TextOperand{Field: field.ID, ID: uint16(value)},
			)
		}
	}
	return result, complete
}

// TextOperands reads confirmed fields even in a partial, read-only script.
// Reading a reference never grants permission to modify the instruction.
func (d *ScriptDraft) TextOperands(index int) []TextOperand {
	if index < 0 || index >= len(d.rows) {
		return nil
	}
	result, _ := scriptTextOperands(d.rows[index])
	return result
}

func (s *Editor) indexTextUses() {
	s.textUses = make(map[uint16][]text.Use)
	s.textUsesComplete = true
	chapters := s.records.Table(records.Chapters)
	var scopes []int
	if chapters == nil || chapters.Err != nil {
		s.textUsesComplete = false
	} else {
		for _, row := range chapters.Rows {
			scopes = append(scopes, row.ID)
		}
	}
	if s.WorldMapEventsAvailable() {
		scopes = append(scopes, WorldMapScripts)
	}
	seen := make(map[text.Use]bool)
	// Bound the combined query as well as each existing script graph walk.
	remaining := 131072
	for _, chapter := range scopes {
		if remaining <= 0 {
			s.textUsesComplete = false
			break
		}
		index, err := s.scriptIndex(chapter)
		if err != nil {
			s.textUsesComplete = false
			continue
		}
		s.textUsesComplete = s.textUsesComplete && !index.partial
		for _, entry := range index.entries {
			// The directory owns world-map contexts independently of chapters.
			if chapter != WorldMapScripts && entry.WorldMap {
				continue
			}
			if entry.Err != nil {
				s.textUsesComplete = false
				continue
			}
			if entry.Address == 0 {
				continue
			}
			program, ok := index.programs[entry.Address]
			if !ok {
				s.textUsesComplete = false
				continue
			}
			s.textUsesComplete = s.textUsesComplete && program.Complete
			for _, row := range program.rows {
				remaining--
				if remaining < 0 {
					s.textUsesComplete = false
					return
				}
				operands, complete := scriptTextOperands(row)
				s.textUsesComplete = s.textUsesComplete && complete
				for _, operand := range operands {
					use := text.Use{
						Kind:    TextUse,
						ID:      chapter,
						Field:   operand.Field,
						Pointer: entry.Pointer,
						Address: row.instruction.Address,
						Source:  entry.Source,
					}
					if !seen[use] {
						seen[use] = true
						s.textUses[operand.ID] = append(
							s.textUses[operand.ID],
							use,
						)
					}
				}
			}
		}
	}
}
