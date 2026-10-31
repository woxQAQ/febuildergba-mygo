// Package quote edits native battle and defeat dialog tables.
package quote

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type Kind string

const (
	Battle     Kind = "quote_battle"
	General    Kind = "quote_general"
	Triangle   Kind = "quote_triangle"
	Death      Kind = "quote_death"
	DeathLyn   Kind = "quote_death_lyn"
	DeathOther Kind = "quote_death_other"
)

const (
	Unit       = "quote_unit"
	Opponent   = "quote_opponent"
	Chapter    = "quote_chapter"
	Route      = "quote_route"
	Flag       = "quote_flag"
	Text       = "quote_text"
	FinalText  = "quote_final_text"
	Script     = "quote_script"
	MaxEntries = 2048
)

var (
	ErrTable = fault.Code{
		ID:   "game.invalid_quote_table",
		Kind: fault.InvalidData,
	}
	ErrReferences = fault.Code{
		ID:   "game.quote_reference_conflict",
		Kind: fault.Conflict,
	}
)

type Field struct {
	ID            string
	Offset, Width int
	Min, Max      uint32
}

func (f Field) Value(raw []byte) uint32 {
	switch f.Width {
	case 1:
		return uint32(raw[f.Offset])
	case 2:
		return uint32(gba.Uint16(raw, f.Offset))
	default:
		return gba.Uint32(raw, f.Offset)
	}
}

// Editable keeps extended character words intact until their layout is known.
func (f Field) Editable(raw []byte) bool {
	return (f.ID != Unit && f.ID != Opponent) || f.Value(raw) <= 255
}

func (f Field) put(raw []byte, value uint32) {
	for i := range f.Width {
		raw[f.Offset+i] = byte(value >> (8 * i))
	}
}

func field(id string, offset, width int, minimum, maximum uint32) Field {
	return Field{id, offset, width, minimum, maximum}
}

type rule struct {
	kind                        Kind
	pointers                    []int
	stride, endOffset, endWidth int
	endValue                    uint32
	fields                      []Field
}

func (r rule) ended(raw []byte) bool {
	return (Field{Offset: r.endOffset, Width: r.endWidth}).Value(
		raw,
	) == r.endValue
}

type Format struct {
	rules      []rule
	anyChapter uint32
	version    int
}

func FE6() Format {
	pair := []Field{
		field(Unit, 0, 1, 1, 255),
		field(Opponent, 1, 1, 1, 255),
		field(Chapter, 2, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Flag, 8, 4, 0, 65535),
	}
	single := []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Flag, 8, 4, 0, 65535),
	}
	return Format{version: 6, anyChapter: 0x2D, rules: []rule{
		{Battle, []int{0x6B660}, 12, 1, 1, 0, pair},
		{General, []int{0x6B744, 0x6B7A4}, 16, 0, 1, 0, single},
		{Triangle, []int{0x6B748, 0x6B7DC}, 16, 0, 1, 0, single},
		{
			Death,
			[]int{0x6B7FC, 0x6B88C, 0x6D2BC},
			16,
			0,
			1,
			0,
			append(slices.Clone(single), field(FinalText, 12, 4, 0, 65535)),
		},
	}}
}

func FE7(japanese bool) Format {
	delta, anyChapter := 0, uint32(0x43)
	if japanese {
		delta, anyChapter = 0x7D0, 0x45
	}
	pair := []Field{
		field(Unit, 0, 1, 1, 255),
		field(Opponent, 1, 1, 1, 255),
		field(Chapter, 2, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Script, 8, 4, 0, 0x09FFFFFF),
		field(Flag, 12, 4, 0, 65535),
	}
	single := []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Flag, 8, 4, 0, 65535),
	}
	death := []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Script, 8, 4, 0, 0x09FFFFFF),
		field(Flag, 12, 4, 0, 65535),
	}
	fallback := []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Script, 4, 4, 0, 0x09FFFFFF),
		field(Flag, 8, 4, 0, 65535),
	}
	rules := []rule{
		{Battle, []int{0x792E0}, 16, 1, 1, 0, pair},
		{General, []int{0x79454, 0x794DC}, 12, 0, 1, 0, single},
		{Triangle, []int{0x79458, 0x79510}, 12, 0, 1, 0, single},
		{Death, []int{0x79550, 0x796B4}, 16, 0, 1, 0, death},
		{DeathLyn, []int{0x7955C, 0x7967C}, 12, 0, 1, 0, fallback},
		{DeathOther, []int{0x79558, 0x79678}, 12, 0, 1, 0, fallback},
	}
	for i := range rules {
		for j := range rules[i].pointers {
			rules[i].pointers[j] += delta
		}
	}
	return Format{version: 7, anyChapter: anyChapter, rules: rules}
}

func FE8(japanese bool) Format {
	battle, death := 0x846AC, 0x8472C
	if japanese {
		battle, death = 0x86978, 0x869F8
	}
	return Format{version: 8, anyChapter: 0xFF, rules: []rule{
		{
			Battle,
			[]int{battle},
			16,
			0,
			2,
			0xFFFF,
			[]Field{
				field(Unit, 0, 2, 0, 255),
				field(Opponent, 2, 2, 0, 255),
				field(Chapter, 4, 2, 0, 255),
				field(Flag, 6, 2, 0, 65535),
				field(Text, 8, 2, 0, 65535),
				field(Script, 12, 4, 0, 0x09FFFFFF),
			},
		},
		{
			Death,
			[]int{death},
			12,
			0,
			2,
			0xFFFF,
			[]Field{
				field(Unit, 0, 2, 1, 255),
				field(Route, 2, 1, 0, 255),
				field(Chapter, 3, 1, 0, 255),
				field(Flag, 4, 2, 0, 65535),
				field(Text, 6, 2, 0, 65535),
				field(Script, 8, 4, 0, 0x09FFFFFF),
			},
		},
	}}
}

// Table retains a complete native terminator. Divergent known consumers are
// reported instead of guessing which entry should drive checks and execution.
type Table struct {
	Kind            Kind
	Address, Stride int
	Pointers        []int
	Entries         []Entry
	Err             error
	terminator      []byte
	rule            rule
}
type Entry struct {
	ID      uint64
	Address int
	Raw     []byte
}

func (f Format) Tables(data []byte) []Table {
	result := make([]Table, 0, len(f.rules))
	for _, rule := range f.rules {
		table := readTable(data, rule, rule.pointers[0])
		if table.Err == nil {
			for _, site := range rule.pointers {
				if at, ok := gba.Pointer(
					data,
					site,
				); !ok ||
					at != table.Address {
					table.Err = fault.Wrap(
						ErrReferences,
						"quote.read_table",
						fault.Context{
							Table: string(rule.kind),
							Range: &fault.Range{
								Offset: site,
								Length: 4,
								Size:   len(data),
							},
						},
					)
					break
				}
			}
		}
		result = append(result, table)
	}
	return result
}

func readTable(data []byte, r rule, site int) (result Table) {
	result = Table{
		Kind:     r.kind,
		Stride:   r.stride,
		Pointers: slices.Clone(r.pointers),
		rule:     r,
	}
	defer func() {
		result.Err = fault.Wrap(
			result.Err,
			"quote.read_table",
			fault.Context{
				Table: string(r.kind),
				Range: &fault.Range{Offset: site, Length: 4, Size: len(data)},
			},
		)
	}()
	at, valid := gba.Pointer(data, site)
	if !valid || at < 0xC0 || at%4 != 0 {
		result.Err = ErrTable
		return
	}
	result.Address = at
	for i := 0; i <= MaxEntries; i++ {
		address := at + i*r.stride
		if !gba.Within(data, address, r.stride) {
			break
		}
		raw := data[address : address+r.stride]
		if r.ended(raw) {
			result.terminator = slices.Clone(raw)
			return
		}
		if i == MaxEntries {
			break
		}
		result.Entries = append(
			result.Entries,
			Entry{ID: uint64(i + 1), Address: address, Raw: slices.Clone(raw)},
		)
	}
	result.Err = ErrTable
	return
}

func (f Format) Ranges(data []byte) []space.Range {
	ranges := f.resourceRanges(data)
	for _, r := range f.rules {
		for _, site := range r.pointers {
			ranges = append(ranges, space.Range{Start: site, End: site + 4})
		}
	}
	return ranges
}

// Declared script prefixes are protected without claiming their full extent.
func (f Format) resourceRanges(data []byte) []space.Range {
	var ranges []space.Range
	for _, r := range f.rules {
		for _, site := range r.pointers {
			table := readTable(data, r, site)
			for _, entry := range table.Entries {
				for _, field := range r.fields {
					if field.ID != Script {
						continue
					}
					if at, valid := gba.Pointer(
						data,
						entry.Address+field.Offset,
					); valid && at >= 0xC0 &&
						gba.Within(data, at, 4) {
						ranges = append(
							ranges,
							space.Range{Start: at, End: at + 4},
						)
					}
				}
			}
			if table.Address != 0 && len(table.Entries) > 0 {
				ranges = append(
					ranges,
					space.Range{
						Start: table.Address,
						End:   table.Address + len(table.Entries)*r.stride,
					},
				)
			}
			if table.Err == nil {
				ranges = append(
					ranges,
					space.Range{
						Start: table.Address + len(table.Entries)*r.stride,
						End:   table.Address + (len(table.Entries)+1)*r.stride,
					},
				)
			}
		}
	}
	return ranges
}

func (f Format) TextUses(data []byte, id uint16) []text.Use {
	var result []text.Use
	for _, table := range f.Tables(data) {
		for i, entry := range table.Entries {
			for _, field := range table.rule.fields {
				if (field.ID == Text || field.ID == FinalText) &&
					field.Value(entry.Raw) == uint32(id) {
					result = append(
						result,
						text.Use{
							Kind:  string(table.Kind),
							ID:    i,
							Field: field.ID,
						},
					)
				}
			}
		}
	}
	return result
}
