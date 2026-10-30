package event

import (
	"bytes"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

var (
	ErrScript = fault.Code{
		ID:   "game.invalid_script",
		Kind: fault.InvalidData,
	}
	ErrScriptStructure = fault.Code{
		ID:   "game.unsupported_script_structure",
		Kind: fault.Unsupported,
	}
)

const maxScriptInstructions = 8192

const scriptResource = "event_script"

// ScriptEntry identifies a declared pointer site. Equal addresses may have
// independent owners; calls are listed as separate entries without execution.
type ScriptEntry struct {
	Chapter, Pointer, Address int
	Source                    string
	Err                       error
}

type ScriptInstruction struct {
	ID       uint64
	Address  int
	Op       ScriptOp
	Help     string
	Raw      []byte
	Editable bool
}

type scriptRow struct {
	instruction   ScriptInstruction
	definition    *scriptDefinition
	label, branch int64
	call          int
	movements     map[string][]ScriptMovementStep
}

// Script is a bounded linear instruction listing, including forward labels past
// an early end. Unknown boundaries leave a partial, read-only listing.
type Script struct {
	Entry        ScriptEntry
	Complete     bool
	Instructions []ScriptInstruction
	rows         []scriptRow
	format       Format
}

type scriptIndex struct {
	binding  edit.Stamp
	entries  []ScriptEntry
	programs map[int]Script
}

func scriptRows(rows []scriptRow) []ScriptInstruction {
	result := make([]ScriptInstruction, len(rows))
	for index, row := range rows {
		result[index] = row.instruction
		result[index].Raw = bytes.Clone(row.instruction.Raw)
	}
	return result
}

func (f Format) readScript(data []byte, entry ScriptEntry) Script {
	result := Script{Entry: entry, format: f}
	if entry.Address == 0 {
		result.Complete = true
		return result
	}
	if entry.Address < 0xC0 || entry.Address%4 != 0 {
		return result
	}
	scanner := deploymentScanner{format: f, data: data}
	definitions := f.scriptDefinitions()
	labels, pending := make(map[int64]bool), make(map[int64]bool)
	at := entry.Address
	for range maxScriptInstructions {
		command, valid := scanner.instruction(at)
		if !valid {
			if gba.Within(data, at, 4) {
				result.rows = append(
					result.rows,
					scriptRow{
						instruction: ScriptInstruction{
							ID:      uint64(len(result.rows) + 1),
							Address: at,
							Op:      ScriptUnknown,
							Raw:     bytes.Clone(data[at : at+4]),
						},
						label:  -1,
						branch: -1,
						call:   -1,
					},
				)
			}
			break
		}
		raw := bytes.Clone(data[at : at+command.length])
		row := scriptRow{
			instruction: ScriptInstruction{
				ID:      uint64(len(result.rows) + 1),
				Address: at,
				Op:      ScriptUnknown,
				Raw:     raw,
			},
			label:  int64(command.label),
			branch: int64(command.branch),
			call:   command.call,
		}
		if f.scriptASM(raw) {
			row.instruction.Op = ScriptASM
		}
		row.describe(definitions, at)
		result.rows = append(result.rows, row)
		if row.label >= 0 {
			labels[row.label] = true
			delete(pending, row.label)
		}
		if row.branch >= 0 && !labels[row.branch] {
			pending[row.branch] = true
		}
		terminal := command.end || command.chapterEnd || command.jump ||
			(row.definition != nil && row.definition.terminal)
		if terminal && (len(pending) == 0 || f.labelSearchEnd(raw)) {
			result.Complete = true
			break
		}
		at += command.length
	}
	result.Instructions = scriptRows(result.rows)
	return result
}

func (s *Editor) scriptFormat() (Format, error) {
	s.Refresh()
	format := s.format
	if s.context.Validate(s.image.Data) != nil {
		return Format{}, edit.ErrVersion
	}
	return format, nil
}

func (s *Editor) scriptIndex(chapter int) (scriptIndex, error) {
	format, err := s.scriptFormat()
	if err != nil {
		return scriptIndex{}, err
	}
	if cached, ok := s.scriptIndexes[chapter]; ok &&
		cached.binding.Current(s.context) {
		return cached, nil
	}
	_, row, field, err := s.records.Resolve(
		records.Chapters,
		chapter,
		HeaderField,
	)
	if err != nil {
		return scriptIndex{}, err
	}
	header, err := format.eventHeader(
		s.image.Data,
		s.image.Data[row.Address+field.Offset],
	)
	if err != nil {
		return scriptIndex{}, err
	}
	result := scriptIndex{
		binding:  s.context.Bind(s.image.Revision),
		programs: make(map[int]Script),
	}
	seen := make(map[int]bool)
	add := func(site int, source string) {
		if seen[site] || len(result.entries) >= 512 ||
			!gba.Within(s.image.Data, site, 4) {
			return
		}
		seen[site] = true
		word := gba.Uint32(s.image.Data, site)
		if word == 1 {
			return
		}
		entry := ScriptEntry{Chapter: chapter, Pointer: site, Source: source}
		if word != 0 {
			address, valid := gba.Pointer(s.image.Data, site)
			if !valid || address < 0xC0 || address%4 != 0 {
				entry.Err = ErrScript
			} else {
				entry.Address = address
			}
		}
		result.entries = append(result.entries, entry)
	}
	if format.opening >= 0 {
		add(header+format.opening, "script_opening")
	}
	add(header+format.ending, "script_ending")
	for _, table := range eventTables {
		conditions, readErr := format.readEvents(
			s.image.Data,
			header+table.offset(),
			table,
		)
		if readErr != nil {
			continue
		}
		for _, condition := range conditions.Events {
			if condition.OperandKind == ConditionScript {
				add(condition.Address+4, condition.source(table))
			}
		}
	}
	// Decode each address once; aliases remain individual pointer entries.
	for cursor, budget := 0, 32768; cursor < len(result.entries) && budget > 0; cursor++ {
		entry := result.entries[cursor]
		if entry.Err != nil || entry.Address == 0 {
			continue
		}
		if _, exists := result.programs[entry.Address]; exists {
			continue
		}
		program := format.readScript(s.image.Data, entry)
		result.programs[entry.Address] = program
		budget -= len(program.rows)
		for _, row := range program.rows {
			if row.call >= 0 {
				add(row.call, "script_call_entry")
			}
		}
	}
	if s.scriptIndexes == nil {
		s.scriptIndexes = make(map[int]scriptIndex)
	}
	s.scriptIndexes[chapter] = result
	return result, nil
}

func (s *Editor) ScriptEntries(chapter int) ([]ScriptEntry, error) {
	index, err := s.scriptIndex(chapter)
	return slices.Clone(index.entries), err
}

func (s *Editor) Script(chapter, site int) (Script, error) {
	index, err := s.scriptIndex(chapter)
	if err != nil {
		return Script{}, err
	}
	for _, entry := range index.entries {
		if entry.Pointer != site {
			continue
		}
		if entry.Err != nil {
			return Script{}, entry.Err
		}
		program, exists := index.programs[entry.Address]
		if !exists {
			format, readErr := s.scriptFormat()
			if readErr != nil {
				return Script{}, readErr
			}
			program = format.readScript(s.image.Data, entry)
		}
		program.Entry = entry
		program.Instructions = scriptRows(program.rows)
		return program, nil
	}
	return Script{}, edit.ErrField
}

func (s *Editor) ScriptCommands() []ScriptOp {
	format, err := s.scriptFormat()
	if err != nil {
		return nil
	}
	var result []ScriptOp
	for _, definition := range format.scriptDefinitions() {
		result = append(result, definition.op)
	}
	return result
}

// FE6/FE7 label lookup stops at native END (0), but continues past the
// equivalent early return (1). FE8 searches by command length until its label.
func (f Format) labelSearchEnd(raw []byte) bool {
	return f.kind != eventFE8 && len(raw) == 4 && gba.Uint32(raw, 0) == 0
}

func (f Format) scriptASM(raw []byte) bool {
	switch f.kind {
	case eventFE8:
		return len(raw) == 8 && raw[1] == 0x0D
	case eventFE7U:
		return len(raw) == 8 && raw[0] == 0x3E
	case eventFE7J:
		return len(raw) == 8 && raw[0] == 0x3C
	case eventFE6:
		return len(raw) == 8 && raw[0] == 0x17
	}
	return false
}

func (row *scriptRow) describe(definitions []scriptDefinition, at int) {
	for index := range definitions {
		if definitions[index].matches(row.instruction.Raw) {
			row.definition = &definitions[index]
			row.instruction.Op = row.definition.op
			row.instruction.Help = row.definition.help
			row.instruction.Editable = true
			break
		}
	}
	if row.definition == nil {
		return
	}
	for _, field := range row.definition.fields {
		if field.ID == ScriptLabelID && row.definition.label {
			row.label = field.value(row.instruction.Raw)
		}
		if field.ID == ScriptLabelID && row.definition.branch {
			row.branch = field.value(row.instruction.Raw)
		}
		if field.Target == ScriptEventTarget {
			row.call = at + field.offset
		}
	}
}

func scriptDeploymentOverlap(
	groups []deployment.Group,
	overlap func(int, int) bool,
) bool {
	for _, group := range groups {
		for _, region := range group.Regions() {
			if overlap(region.Start, region.End-region.Start) {
				return true
			}
		}
	}
	return false
}
