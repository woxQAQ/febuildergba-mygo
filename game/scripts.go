package game

import (
	"bytes"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
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
	Raw      []byte
	Editable bool
}

type scriptRow struct {
	instruction   ScriptInstruction
	definition    *scriptDefinition
	label, branch int64
	call          int
}

// Script is a bounded linear instruction listing, including forward labels past
// an early end. Unknown boundaries leave a partial, read-only listing.
type Script struct {
	Entry        ScriptEntry
	Complete     bool
	Instructions []ScriptInstruction
	rows         []scriptRow
	format       deploymentFormat
}

type scriptIndex struct {
	revision uint64
	layouts  map[Kind]Layout
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

func (f deploymentFormat) readScript(data []byte, entry ScriptEntry) Script {
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
			if within(data, at, 4) {
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
		if terminal && len(pending) == 0 {
			result.Complete = true
			break
		}
		at += command.length
	}
	result.Instructions = scriptRows(result.rows)
	return result
}

func (s *Session) scriptFormat() (gameFormat, error) {
	s.Refresh()
	format, ok := s.Adapter.(gameFormat)
	if !ok || !format.Match(s.image.Data) {
		return gameFormat{}, ErrVersion
	}
	return format, nil
}

func (s *Session) scriptIndex(chapter int) (scriptIndex, error) {
	format, err := s.scriptFormat()
	if err != nil {
		return scriptIndex{}, err
	}
	if cached, ok := s.scriptIndexes[chapter]; ok &&
		cached.revision == s.image.Revision &&
		maps.Equal(cached.layouts, s.layouts) {
		return cached, nil
	}
	_, row, field, err := s.locate(Chapters, chapter, chapterEventsField)
	if err != nil {
		return scriptIndex{}, err
	}
	header, err := format.deployments.eventHeader(
		s.image.Data,
		s.image.Data[row.Address+field.Offset],
	)
	if err != nil {
		return scriptIndex{}, err
	}
	result := scriptIndex{
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		programs: make(map[int]Script),
	}
	seen := make(map[int]bool)
	add := func(site int, source string) {
		if seen[site] || len(result.entries) >= 512 ||
			!within(s.image.Data, site, 4) {
			return
		}
		seen[site] = true
		word := u32(s.image.Data, site)
		if word == 1 {
			return
		}
		entry := ScriptEntry{Chapter: chapter, Pointer: site, Source: source}
		if word != 0 {
			address, valid := pointer(s.image.Data, site)
			if !valid || address < 0xC0 || address%4 != 0 {
				entry.Err = ErrScript
			} else {
				entry.Address = address
			}
		}
		result.entries = append(result.entries, entry)
	}
	if format.deployments.opening >= 0 {
		add(header+format.deployments.opening, "script_opening")
	}
	add(header+format.deployments.ending, "script_ending")
	for _, table := range eventTables {
		conditions, readErr := format.deployments.readEvents(
			s.image.Data,
			header+table.offset(),
			table,
		)
		if readErr != nil {
			continue
		}
		for _, condition := range conditions.Events {
			add(condition.Address+4, condition.source(table))
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
		program := format.deployments.readScript(s.image.Data, entry)
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

func (s *Session) ScriptEntries(chapter int) ([]ScriptEntry, error) {
	index, err := s.scriptIndex(chapter)
	return slices.Clone(index.entries), err
}

func (s *Session) Script(chapter, site int) (Script, error) {
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
			program = format.deployments.readScript(s.image.Data, entry)
		}
		program.Entry = entry
		program.Instructions = scriptRows(program.rows)
		return program, nil
	}
	return Script{}, ErrField
}

func (s *Session) ScriptCommands() []ScriptOp {
	format, err := s.scriptFormat()
	if err != nil {
		return nil
	}
	var result []ScriptOp
	for _, definition := range format.deployments.scriptDefinitions() {
		result = append(result, definition.op)
	}
	return result
}
