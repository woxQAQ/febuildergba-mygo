// Package terrain edits native class movement costs and terrain bonuses.
package terrain

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var (
	ErrTable = fault.Code{
		ID:   "game.invalid_terrain",
		Kind: fault.InvalidData,
	}
	ErrReferences = fault.Code{
		ID:   "game.terrain_reference_conflict",
		Kind: fault.Conflict,
	}
)

type Kind string

const (
	Normal     Kind = "move_normal"
	Rain       Kind = "move_rain"
	Snow       Kind = "move_snow"
	Avoid      Kind = "terrain_avoid"
	Defense    Kind = "terrain_defense"
	Resistance Kind = "terrain_resistance"
)

func (k Kind) Movement() bool { return k == Normal || k == Rain || k == Snow }

type Column struct {
	Kind   Kind
	Offset int
}

type Format struct {
	count   int
	columns []Column
}

func ForVersion(id string) Format {
	switch id {
	case version.FE6:
		return Format{
			51,
			[]Column{
				{Normal, 52},
				{Avoid, 56},
				{Defense, 60},
				{Resistance, 64},
			},
		}
	case version.FE7J, version.FE7U, version.FE8J, version.FE8U:
		return Format{
			65,
			[]Column{
				{Normal, 56},
				{Rain, 60},
				{Snow, 64},
				{Avoid, 68},
				{Defense, 72},
				{Resistance, 76},
			},
		}
	default:
		return Format{}
	}
}

func (f Format) Count() int        { return f.count }
func (f Format) Columns() []Column { return slices.Clone(f.columns) }

// Class describes the current record layout, including manual table locations.
type Class struct {
	ID, Address, Length int
	Name                string
}

type Reference struct {
	Class            Class
	Kind             Kind
	Pointer, Address int
}

type Table struct {
	Reference
	Values []int
	Empty  bool
	Err    error
}

func (f Format) Read(data []byte, class Class) []Table {
	result := make([]Table, len(f.columns))
	for i, column := range f.columns {
		table := &result[i]
		table.Reference = Reference{
			Class:   class,
			Kind:    column.Kind,
			Pointer: class.Address + column.Offset,
		}
		site := table.Pointer
		switch {
		case class.Address < 0xC0 || column.Offset+4 > class.Length || site%4 != 0 || !gba.Within(data, site, 4):
			table.Err = ErrTable
		case gba.Uint32(data, site) == 0:
			table.Empty = true
		default:
			address, valid := gba.Pointer(data, site)
			table.Address = address
			if !valid || address < 0xC0 || !gba.Within(data, address, f.count) {
				table.Err = ErrTable
				break
			}
			table.Values = make([]int, f.count)
			for id := range table.Values {
				table.Values[id] = int(
					int8(data[address+id]),
				) // #nosec G115 -- native signed-byte interpretation.
			}
		}
		if table.Err != nil {
			table.Err = fault.Wrap(table.Err, "terrain.read", fault.Context{
				Resource: "terrain",
				RecordID: &class.ID,
				Field:    string(column.Kind),
			})
		}
	}
	return result
}

func (f Format) Ranges(data []byte, classes []Class) []space.Range {
	var result []space.Range
	for _, class := range classes {
		for _, table := range f.Read(data, class) {
			if table.Pointer >= 0xC0 && gba.Within(data, table.Pointer, 4) &&
				table.Pointer+4 <= class.Address+class.Length {
				result = append(
					result,
					space.Range{Start: table.Pointer, End: table.Pointer + 4},
				)
			}
			if table.Address >= 0xC0 && table.Address < len(data) {
				result = append(
					result,
					space.Range{
						Start: table.Address,
						End:   min(table.Address+f.count, len(data)),
					},
				)
			}
		}
	}
	return result
}

// NameKey uses native terrain identities; unknown IDs remain visible by number.
// These labels do not claim to decode a modified ROM's terrain name table.
func (f Format) NameKey(id int) string {
	if id < 0 || id >= f.count {
		return "terrain_unknown"
	}
	return "terrain_" + terrainNames[id]
}

var terrainNames = [...]string{
	"unknown",
	"plains",
	"road",
	"village",
	"village_closed",
	"house",
	"armory",
	"vendor",
	"arena",
	"unknown",
	"fort",
	"castle_gate",
	"forest",
	"thicket",
	"sand",
	"desert",
	"river",
	"mountain",
	"peak",
	"bridge",
	"drawbridge",
	"sea",
	"lake",
	"floor",
	"magic_floor",
	"fence",
	"wall",
	"damaged_wall",
	"rubble",
	"pillar",
	"door",
	"throne",
	"empty_chest",
	"chest",
	"roof",
	"gate",
	"church",
	"ruins",
	"cliff",
	"ballista",
	"long_ballista",
	"killer_ballista",
	"ship",
	"shipwreck",
	"unknown",
	"stairs",
	"unknown",
	"glacier",
	"arena",
	"valley",
	"fence",
	"snag",
	"snag_bridge",
	"sky",
	"deeps",
	"village_ruins",
	"inn",
	"barrel",
	"bone",
	"dark",
	"water",
	"gunnels",
	"deck",
	"brace",
	"mast",
}
