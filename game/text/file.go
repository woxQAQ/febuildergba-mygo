package text

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

const MaxFileSize = 16 << 20

var (
	ErrFileSyntax = fault.Code{
		ID:   "game.text_file_syntax",
		Kind: fault.InvalidInput,
	}
	ErrFileID = fault.Code{
		ID:   "game.text_file_id",
		Kind: fault.InvalidInput,
	}
	ErrFileDuplicate = fault.Code{
		ID:   "game.text_file_duplicate",
		Kind: fault.InvalidInput,
	}
	ErrFileSize = fault.Code{
		ID:   "game.text_file_size",
		Kind: fault.InvalidInput,
	}
	ErrFileEntries = fault.Code{
		ID:   "game.text_file_entries",
		Kind: fault.InvalidInput,
	}
)

// File owns a text-table snapshot for background file conversion. It never
// retains an editor or calls session reference providers on a worker.
type File struct {
	binding edit.Stamp
	format  Format
	data    []byte
	table   Table
}

func (s *Editor) File() (*File, error) {
	if err := s.refresh(); err != nil {
		return nil, err
	}
	table, err := s.format.Table(s.image.Data)
	if err != nil {
		return nil, err
	}
	return &File{
		binding: s.stamp,
		format:  s.format,
		data:    s.image.Data,
		table:   table,
	}, nil
}

func (f *File) Current(s *Editor) bool {
	return f != nil && f.binding.Current(s.context)
}

// Export uses FEBuilder's [hex ID] sections and raw native markers. Explicit
// markers keep newlines, literal brackets, comment delimiters and empty text
// intact when an older importer discards physical empty lines and comments.
func (f *File) Export(ids []uint16) ([]byte, error) {
	if len(ids) == 0 {
		return nil, edit.ErrValue
	}
	var output strings.Builder
	seen := make(map[uint16]bool, len(ids))
	for _, id := range ids {
		if id == 0 || int(id) >= f.table.Count {
			return nil, fileError(ErrFileID, 0, id, nil)
		}
		if seen[id] {
			return nil, fileError(ErrFileDuplicate, 0, id, nil)
		}
		seen[id] = true
		value, err := f.format.Read(f.data, id)
		if err != nil {
			return nil, fileError(ErrFileEntries, 0, id, err)
		}
		fmt.Fprintf(&output, "[%04X]\r\n%s\r\n", id, fileBody(value.Raw))
		if output.Len() > MaxFileSize {
			return nil, ErrFileSize
		}
	}
	return []byte(output.String()), nil
}

func fileBody(raw string) string {
	if raw == "" {
		return "@0000"
	}
	var result strings.Builder
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			i++
			fmt.Fprintf(&result, "@%04X", raw[i])
		case '[':
			// Raw rendering emits only fixed-width control markers here;
			// literal opening brackets were escaped above.
			result.WriteByte('@')
			result.WriteString(raw[i+3 : i+7])
			i += 7
		case '\n':
			result.WriteString("@0001")
		case '@', '/', '{', ']':
			fmt.Fprintf(&result, "@%04X", raw[i])
		default:
			result.WriteByte(raw[i])
		}
	}
	return result.String()
}

// FileEntry is a review row. Err blocks the entire import; Changed refers to
// normalized game content, rather than alternate spellings of control codes.
type FileEntry struct {
	ID            uint16
	Line          int
	Before, After string
	Changed       bool
	Err           error
	Uses          []Use
}

type Import struct {
	source       *File
	entries      []FileEntry
	replacements []replacement
}

func cloneFileEntries(entries []FileEntry) []FileEntry {
	result := slices.Clone(entries)
	for i := range result {
		result[i].Uses = slices.Clone(result[i].Uses)
	}
	return result
}

func (d *Import) Current(
	s *Editor,
) bool {
	return d != nil && d.source.Current(s)
}
func (d *Import) Entries() []FileEntry { return cloneFileEntries(d.entries) }

func fileError(code fault.Code, line int, id uint16, cause error) error {
	return fault.New(code, "game.text_file", fault.Context{
		Resource: Resource, ResourceID: new(int(id)), RecordID: new(line),
	}, cause)
}

// Import follows the legacy physical-line rules, with strict duplicate and ID
// validation. UTF-8 may have a BOM; both LF and CRLF are accepted.
func (f *File) Import(content []byte) (*Import, error) {
	if len(content) == 0 || len(content) > MaxFileSize {
		return nil, ErrFileSize
	}
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(content) {
		return nil, ErrFileSyntax
	}
	lines := strings.Split(
		strings.ReplaceAll(string(content), "\r\n", "\n"),
		"\n",
	)
	result := &Import{source: f}
	seen := make(map[uint16]bool)
	var body []string
	var id uint16
	lineNumber := 0
	finish := func() {
		if lineNumber == 0 {
			return
		}
		entry := FileEntry{
			ID:    id,
			Line:  lineNumber,
			After: strings.Join(body, "\n"),
		}
		original, err := f.format.Read(f.data, id)
		entry.Before = original.Content
		if err != nil {
			entry.Err = err
		} else {
			value, parseErr := f.format.fileValue(entry.After)
			entry.Err = parseErr
			if parseErr == nil {
				entry.After = value.Content
				entry.Changed = entry.Before != entry.After
				if entry.Changed {
					if f.table.Recovered {
						entry.Err = ErrText
					} else {
						at := f.table.Address + int(id)*4
						encoded, expected, encodeErr := f.format.encodeText(
							f.data,
							at,
							id,
							entry.After,
							Description,
						)
						entry.Err = encodeErr
						if encodeErr == nil {
							result.replacements = append(
								result.replacements,
								replacement{
									id:       id,
									role:     Description,
									entry:    at,
									encoded:  encoded,
									expected: expected,
								},
							)
						}
					}
				}
			}
		}
		if entry.Err != nil {
			entry.Err = fileError(
				fault.CodeOf(entry.Err),
				lineNumber,
				id,
				entry.Err,
			)
		}
		result.entries = append(result.entries, entry)
	}
	for index, raw := range lines {
		if strings.ContainsRune(raw, '\r') {
			return nil, fileError(ErrFileSyntax, index+1, id, nil)
		}
		if f.format.ShiftJIS && strings.Contains(raw, "\t{U}") ||
			!f.format.ShiftJIS && strings.Contains(raw, "\t{J}") {
			continue
		}
		line := raw
		for _, marker := range []string{"{J}", "{U}", "//"} {
			at := strings.Index(line, marker)
			if at == 0 {
				line = ""
			} else if at > 0 && (line[at-1] == ' ' || line[at-1] == '\t') {
				line = line[:at-1]
			}
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") &&
			len(line) >= 4 {
			value, parseErr := parseFileHex(line[1:len(line)-1], 32)
			if parseErr == nil || line == "[XXXX]" {
				if parseErr != nil || value == 0 || value > 0x7FFE {
					return nil, fileError(ErrFileID, index+1, 0, nil)
				}
				if int(value) >= f.table.Count {
					return nil, fileError(ErrFileID, index+1, 0, nil)
				}
				next := uint16(value)
				if seen[next] {
					return nil, fileError(ErrFileDuplicate, index+1, next, nil)
				}
				finish()
				id, lineNumber, body = next, index+1, nil
				seen[id] = true
				continue
			}
		}
		if lineNumber == 0 {
			return nil, fileError(ErrFileSyntax, index+1, 0, nil)
		}
		body = append(body, line)
	}
	finish()
	if len(result.entries) == 0 {
		return nil, ErrFileSyntax
	}
	return result, nil
}

func parseFileHex(value string, bits int) (uint64, error) {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'A' || char > 'F') &&
			(char < 'a' || char > 'f') {
			return 0, ErrFileSyntax
		}
	}
	return strconv.ParseUint(value, 16, bits)
}

var fileCommandAliases = map[string]string{
	"FarLeft":     "OpenFarLeft",
	"MidLeft":     "OpenMidLeft",
	"Left":        "OpenLeft",
	"Right":       "OpenRight",
	"MidRight":    "OpenMidRight",
	"FarRight":    "OpenFarRight",
	"FarFarLeft":  "OpenFarFarLeft",
	"FarFarRight": "OpenFarFarRight",
	"Tact2":       "Tact 2",
}

func (f Format) fileValue(content string) (Value, error) {
	var words []uint16
	var span strings.Builder
	var pending uint16
	ended := false
	flush := func() error {
		if span.Len() == 0 {
			return nil
		}
		if pending != 0 || ended {
			return ErrTextArgument
		}
		units, err := f.characterUnits(span.String())
		if err != nil {
			return err
		}
		for _, unit := range units {
			words = append(words, unit.symbol)
		}
		span.Reset()
		return nil
	}
	appendCode := func(code uint16) error {
		if ended {
			return ErrTextControl
		}
		if pending != 0 {
			words = append(words, code)
			pending = 0
			return nil
		}
		if code == 0 {
			ended = true
			return nil
		}
		words = append(words, code)
		if code == 0x10 || code == 0x80 {
			pending = code
		}
		return nil
	}
	for i := 0; i < len(content); {
		if len(words) > 4096 || span.Len() > 16384 {
			return Value{}, ErrTextEncoding
		}
		switch content[i] {
		case '@':
			if i+5 <= len(content) {
				code, err := parseFileHex(content[i+1:i+5], 16)
				if err == nil && code <= 0xFFFF {
					if err := flush(); err != nil {
						return Value{}, err
					}
					if err := appendCode(uint16(code)); err != nil {
						return Value{}, err
					}
					i += 5
					continue
				}
			}
			span.WriteByte('@')
			i++
		case '[':
			if err := flush(); err != nil {
				return Value{}, err
			}
			end := strings.IndexByte(content[i:], ']')
			if end < 0 {
				return Value{}, ErrTextControl
			}
			name := content[i+1 : i+end]
			if name == "X" || name == "N" {
				i += end + 1
				continue
			}
			if strings.HasPrefix(name, "0x") {
				if len(name) < 3 || len(name) > 6 {
					return Value{}, ErrTextControl
				}
				code, err := parseFileHex(name[2:], 16)
				if err != nil || code > 0xFFFF {
					return Value{}, ErrTextControl
				}
				if err := appendCode(uint16(code)); err != nil {
					return Value{}, err
				}
			} else {
				if alias, ok := fileCommandAliases[name]; ok {
					name = alias
				}
				units, next, _, err := f.marker("["+name+"]", pending)
				if err != nil || ended {
					return Value{}, ErrTextControl
				}
				for _, unit := range units {
					words = append(words, unit.symbol)
				}
				pending = next
			}
			i += end + 1
		case '\n':
			if err := flush(); err != nil {
				return Value{}, err
			}
			if pending != 0 || ended {
				return Value{}, ErrTextArgument
			}
			words = append(words, 1)
			i++
		default:
			span.WriteByte(content[i])
			i++
		}
	}
	if err := flush(); err != nil {
		return Value{}, err
	}
	if pending != 0 || len(words) > 4096 {
		return Value{}, ErrTextArgument
	}
	value := f.descriptionValue(textStream{words: words})
	return value, nil
}

type ImportPlan struct {
	commit     *edit.Commit
	entries    []FileEntry
	addedBytes int
}

func (p *ImportPlan) Current(
	s *Editor,
) bool {
	return p != nil && p.commit.Current(s.context)
}

func (p *ImportPlan) Entries() []FileEntry { return cloneFileEntries(p.entries) }
func (p *ImportPlan) AddedBytes() int      { return p.addedBytes }

func (s *Editor) PlanImport(draft *Import) (*ImportPlan, error) {
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if err := s.refresh(); err != nil {
		return nil, err
	}
	var invalid []error
	for _, entry := range draft.entries {
		if entry.Err != nil {
			invalid = append(invalid, entry.Err)
		}
	}
	if len(invalid) > 0 {
		return nil, fault.New(
			ErrFileEntries,
			"game.plan_text_import",
			fault.Context{Resource: Resource},
			errors.Join(invalid...),
		)
	}
	if len(draft.replacements) == 0 {
		return nil, edit.ErrValue
	}
	replacements := slices.Clone(draft.replacements)
	entries := cloneFileEntries(draft.entries)
	rows := make(map[uint16]int, len(entries))
	for index, entry := range entries {
		rows[entry.ID] = index
	}
	for i := range replacements {
		item := &replacements[i]
		item.references = s.references(s.image.Data, item.id)
		entries[rows[item.id]].Uses = slices.Clone(item.references.Uses)
	}
	plan, err := s.format.planReplacements(s.image.Data, replacements)
	if err != nil {
		return nil, fault.Wrap(
			err,
			"game.plan_text_import",
			fault.Context{Resource: Resource},
		)
	}
	return &ImportPlan{
		commit: edit.NewCommit(
			draft.source.binding,
			"text.import",
			plan.Edit,
		),
		entries:    entries,
		addedBytes: plan.Edit.Size - len(s.image.Data),
	}, nil
}

func (s *Editor) ApplyImport(plan *ImportPlan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return fault.Wrap(
		plan.commit.Apply(s.context),
		"game.apply_text_import",
		fault.Context{Resource: Resource},
	)
}
