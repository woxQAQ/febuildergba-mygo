package game

import (
	"bytes"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// ScriptDraft keeps native rows and stable identities independently of UI state.
// Parameter-only edits keep their addresses; structural edits require a fully
// understood instruction stream and are appended as a separate resource.
type ScriptDraft struct {
	document *rom.Document
	revision uint64
	layouts  map[Kind]Layout
	original Script
	rows     []scriptRow
	nextID   uint64
}

func cloneScriptRows(rows []scriptRow) []scriptRow {
	result := slices.Clone(rows)
	for index := range result {
		result[index].instruction.Raw = bytes.Clone(
			result[index].instruction.Raw,
		)
	}
	return result
}

func (s *Session) NewScriptDraft(chapter, site int) (*ScriptDraft, error) {
	program, err := s.Script(chapter, site)
	if err != nil {
		return nil, err
	}
	return &ScriptDraft{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		original: program,
		rows:     cloneScriptRows(program.rows),
		nextID:   uint64(len(program.rows) + 1),
	}, nil
}

func (d *ScriptDraft) Entry() ScriptEntry { return d.original.Entry }
func (d *ScriptDraft) Current(s *Session) bool {
	return d.document == s.Document &&
		d.revision == s.Document.State().Revision &&
		maps.Equal(d.layouts, s.layouts)
}

func (d *ScriptDraft) Instructions() []ScriptInstruction { return scriptRows(d.rows) }

func (d *ScriptDraft) Complete() bool { return d.original.Complete }
func (d *ScriptDraft) Modified() bool {
	if len(d.rows) != len(d.original.rows) {
		return true
	}
	for index, row := range d.rows {
		if !bytes.Equal(
			row.instruction.Raw,
			d.original.rows[index].instruction.Raw,
		) {
			return true
		}
	}
	return false
}

func (d *ScriptDraft) structural() bool {
	if len(d.rows) != len(d.original.rows) {
		return true
	}
	for index, row := range d.rows {
		if row.instruction.ID != d.original.rows[index].instruction.ID ||
			len(
				row.instruction.Raw,
			) != len(
				d.original.rows[index].instruction.Raw,
			) {
			return true
		}
	}
	return false
}

func (d *ScriptDraft) Relocatable() bool {
	if !d.original.Complete {
		return false
	}
	for _, row := range d.rows {
		if row.definition == nil {
			return false
		}
	}
	return true
}

func (d *ScriptDraft) originalLength() int {
	length := 0
	for _, row := range d.original.rows {
		length += len(row.instruction.Raw)
	}
	return length
}

func (d *ScriptDraft) Fields(index int) []ScriptField {
	if !d.original.Complete || index < 0 || index >= len(d.rows) ||
		d.rows[index].definition == nil {
		return nil
	}
	return slices.Clone(d.rows[index].definition.fields)
}

func (d *ScriptDraft) Value(index int, id string) (int64, error) {
	for _, field := range d.Fields(index) {
		if field.ID == id {
			return field.value(d.rows[index].instruction.Raw), nil
		}
	}
	return 0, ErrField
}

// Reference resolves a declared script or deployment operand in the draft.
// New instructions have no pointer site until the plan has been applied.
func (d *ScriptDraft) Reference(index int, id string) (ScriptEntry, error) {
	if index < 0 || index >= len(d.rows) ||
		d.rows[index].instruction.Address == 0 {
		return ScriptEntry{}, ErrField
	}
	for _, field := range d.Fields(index) {
		if field.ID != id || (id != ScriptPointer && id != ScriptUnitPointer) {
			continue
		}
		value := field.value(d.rows[index].instruction.Raw)
		if value < 0x080000C0 || value >= 0x0A000000 {
			return ScriptEntry{}, ErrValue
		}
		return ScriptEntry{
			Chapter: d.Entry().Chapter,
			Pointer: d.rows[index].instruction.Address + field.offset,
			Address: int(value) - 0x08000000,
			Source:  string(d.rows[index].instruction.Op),
		}, nil
	}
	return ScriptEntry{}, ErrField
}

func (d *ScriptDraft) Set(index int, id string, value int64) (err error) {
	context := fault.Context{
		Resource: scriptResource,
		RecordID: new(index),
		Field:    id,
		Value:    new(value),
	}
	defer func() { err = fault.Wrap(err, "game.change_script_draft", context) }()
	for _, field := range d.Fields(index) {
		if field.ID != id {
			continue
		}
		if value < 0 || value > field.Max {
			context.Bounds = &fault.Bounds{Min: 0, Max: field.Max}
			return ErrValue
		}
		field.put(d.rows[index].instruction.Raw, value)
		if id == ScriptLabelID {
			if d.rows[index].definition.label {
				d.rows[index].label = value
			}
			if d.rows[index].definition.branch {
				d.rows[index].branch = value
			}
		}
		return nil
	}
	return ErrField
}

func (d *ScriptDraft) Insert(index int, op ScriptOp) error {
	if !d.Relocatable() {
		return ErrScriptStructure
	}
	if index < 0 || index > len(d.rows) ||
		len(d.rows) >= maxScriptInstructions {
		return ErrValue
	}
	for _, definition := range d.original.format.scriptDefinitions() {
		if definition.op != op {
			continue
		}
		row := scriptRow{
			instruction: ScriptInstruction{
				ID:       d.nextID,
				Op:       op,
				Raw:      bytes.Clone(definition.raw),
				Editable: true,
			},
			definition: &definition,
			label:      -1,
			branch:     -1,
			call:       -1,
		}
		d.nextID++
		if definition.label {
			row.label = 0
		}
		if definition.branch {
			row.branch = 0
		}
		d.rows = slices.Insert(d.rows, index, row)
		return nil
	}
	return ErrField
}

func (d *ScriptDraft) Remove(index int) error {
	if !d.Relocatable() {
		return ErrScriptStructure
	}
	if index < 0 || index >= len(d.rows) {
		return ErrField
	}
	d.rows = slices.Delete(d.rows, index, index+1)
	return nil
}

func (d *ScriptDraft) Duplicate(index int) error {
	if !d.Relocatable() {
		return ErrScriptStructure
	}
	if index < 0 || index >= len(d.rows) ||
		len(d.rows) >= maxScriptInstructions {
		return ErrField
	}
	row := cloneScriptRows(d.rows[index : index+1])[0]
	row.instruction.ID, row.instruction.Address = d.nextID, 0
	d.nextID++
	d.rows = slices.Insert(d.rows, index+1, row)
	return nil
}

func (d *ScriptDraft) Move(from, to int) error {
	if !d.Relocatable() {
		return ErrScriptStructure
	}
	if from < 0 || to < 0 || from >= len(d.rows) || to >= len(d.rows) {
		return ErrField
	}
	if from == to {
		return nil
	}
	row := d.rows[from]
	if from < to {
		copy(d.rows[from:to], d.rows[from+1:to+1])
	} else {
		copy(d.rows[to+1:from+1], d.rows[to:from])
	}
	d.rows[to] = row
	return nil
}

func (d *ScriptDraft) validateLabels() error {
	labels := make(map[int64]bool)
	for _, row := range d.rows {
		if row.label >= 0 {
			if labels[row.label] {
				return ErrScript
			}
			labels[row.label] = true
		}
	}
	for _, row := range d.rows {
		if row.branch >= 0 && !labels[row.branch] {
			return ErrScript
		}
	}
	return nil
}
