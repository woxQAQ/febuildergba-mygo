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

// Rule describes one native table's structure. Pointers are its confirmed
// consumers, listed by the version file.
type Rule struct {
	Kind                        Kind
	Pointers                    []int
	Stride, EndOffset, EndWidth int
	EndValue                    uint32
	Fields                      []Field
}

func (r Rule) ended(raw []byte) bool {
	return (Field{Offset: r.EndOffset, Width: r.EndWidth}).Value(
		raw,
	) == r.EndValue
}

// Format declares the native tables and their version-independent semantics.
// BattleConditions marks FE8 battle chapter conditions, including the triangle
// marker; SilentDeath marks FE8 death entries staying silent without script or
// text.
type Format struct {
	Rules            []Rule
	AnyChapter       uint32
	BattleConditions bool
	SilentDeath      bool
}

func FE6BattleFields() []Field {
	return []Field{
		field(Unit, 0, 1, 1, 255),
		field(Opponent, 1, 1, 1, 255),
		field(Chapter, 2, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Flag, 8, 4, 0, 65535),
	}
}

func FE6GeneralFields() []Field {
	return []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Flag, 8, 4, 0, 65535),
	}
}

func FE6DeathFields() []Field {
	return append(FE6GeneralFields(), field(FinalText, 12, 4, 0, 65535))
}

func FE7BattleFields() []Field {
	return []Field{
		field(Unit, 0, 1, 1, 255),
		field(Opponent, 1, 1, 1, 255),
		field(Chapter, 2, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Script, 8, 4, 0, 0x09FFFFFF),
		field(Flag, 12, 4, 0, 65535),
	}
}

func FE7GeneralFields() []Field {
	return []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Flag, 8, 4, 0, 65535),
	}
}

func FE7DeathFields() []Field {
	return []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Text, 4, 4, 0, 65535),
		field(Script, 8, 4, 0, 0x09FFFFFF),
		field(Flag, 12, 4, 0, 65535),
	}
}

func FE7FallbackFields() []Field {
	return []Field{
		field(Unit, 0, 1, 1, 255),
		field(Chapter, 1, 1, 0, 255),
		field(Script, 4, 4, 0, 0x09FFFFFF),
		field(Flag, 8, 4, 0, 65535),
	}
}

func FE8BattleFields() []Field {
	return []Field{
		field(Unit, 0, 2, 0, 255),
		field(Opponent, 2, 2, 0, 255),
		field(Chapter, 4, 2, 0, 255),
		field(Flag, 6, 2, 0, 65535),
		field(Text, 8, 2, 0, 65535),
		field(Script, 12, 4, 0, 0x09FFFFFF),
	}
}

func FE8DeathFields() []Field {
	return []Field{
		field(Unit, 0, 2, 1, 255),
		field(Route, 2, 1, 0, 255),
		field(Chapter, 3, 1, 0, 255),
		field(Flag, 4, 2, 0, 65535),
		field(Text, 6, 2, 0, 65535),
		field(Script, 8, 4, 0, 0x09FFFFFF),
	}
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
	rule            Rule
}
type Entry struct {
	ID      uint64
	Address int
	Raw     []byte
}

func (f Format) Tables(data []byte) []Table {
	result := make([]Table, 0, len(f.Rules))
	for _, r := range f.Rules {
		table := readTable(data, r, r.Pointers[0])
		if table.Err == nil {
			for _, site := range r.Pointers {
				if at, ok := gba.Pointer(
					data,
					site,
				); !ok ||
					at != table.Address {
					table.Err = fault.Wrap(
						ErrReferences,
						"quote.read_table",
						fault.Context{
							Table: string(r.Kind),
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

func readTable(data []byte, r Rule, site int) (result Table) {
	result = Table{
		Kind:     r.Kind,
		Stride:   r.Stride,
		Pointers: slices.Clone(r.Pointers),
		rule:     r,
	}
	defer func() {
		result.Err = fault.Wrap(
			result.Err,
			"quote.read_table",
			fault.Context{
				Table: string(r.Kind),
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
		address := at + i*r.Stride
		if !gba.Within(data, address, r.Stride) {
			break
		}
		raw := data[address : address+r.Stride]
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

// Claims declares each table's consumer pointers, payload and terminator,
// plus the declared script prefixes. Script prefixes are protected without
// claiming their full extent.
func (f Format) Claims(data []byte) []space.Claim {
	var result []space.Claim
	for _, r := range f.Rules {
		owner := space.Owner{Resource: string(r.Kind)}
		for _, site := range r.Pointers {
			result = append(
				result,
				space.Claim{
					Range: space.Range{Start: site, End: site + 4},
					Owner: owner,
					Site:  true,
				},
			)
			table := readTable(data, r, site)
			for _, entry := range table.Entries {
				for _, field := range r.Fields {
					if field.ID != Script {
						continue
					}
					if at, valid := gba.Pointer(
						data,
						entry.Address+field.Offset,
					); valid && at >= 0xC0 &&
						gba.Within(data, at, 4) {
						result = append(
							result,
							space.Claim{
								Range: space.Range{Start: at, End: at + 4},
								Owner: owner,
							},
						)
					}
				}
			}
			if table.Address != 0 && len(table.Entries) > 0 {
				result = append(
					result,
					space.Claim{
						Range: space.Range{
							Start: table.Address,
							End:   table.Address + len(table.Entries)*r.Stride,
						},
						Owner: owner,
					},
				)
			}
			if table.Err == nil {
				result = append(
					result,
					space.Claim{
						Range: space.Range{
							Start: table.Address + len(table.Entries)*r.Stride,
							End:   table.Address + (len(table.Entries)+1)*r.Stride,
						},
						Owner: owner,
					},
				)
			}
		}
	}
	return result
}

func (f Format) TextUses(data []byte, id uint16) []text.Use {
	var result []text.Use
	for _, table := range f.Tables(data) {
		for i, entry := range table.Entries {
			for _, field := range table.rule.Fields {
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
