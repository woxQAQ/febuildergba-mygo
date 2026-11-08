// Package terrain edits native class movement costs and terrain bonuses.
package terrain

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

var ErrTable = fault.Code{
	ID:   "game.invalid_terrain",
	Kind: fault.InvalidData,
}

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

// FE6Layout is the FE6 class table's terrain columns.
func FE6Layout() Format {
	return Format{
		51,
		[]Column{
			{Normal, 52},
			{Avoid, 56},
			{Defense, 60},
			{Resistance, 64},
		},
	}
}

// FE7Layout is the terrain column layout shared by FE7 and FE8.
func FE7Layout() Format {
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
}

func (f Format) Count() int        { return f.count }
func (f Format) Columns() []Column { return slices.Clone(f.columns) }

// Class describes the current record layout, including manual table
// locations. Host names the containing record, so the session can tell an
// owner's own pointer from damage.
type Class struct {
	ID, Address, Length int
	Name                string
	Host                romwrite.Owner
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

// Claims declares each class's column pointers (hosted by the class record)
// and their payloads. Slot is the column index, so two columns or two classes
// aliasing one position conflict instead of silently sharing it.
func (f Format) Claims(data []byte, classes []Class) []romwrite.Claim {
	var result []romwrite.Claim
	for _, class := range classes {
		for slot, table := range f.Read(data, class) {
			owner := romwrite.Owner{
				Resource: "terrain",
				ID:       class.ID,
				Slot:     slot,
			}
			if table.Pointer >= 0xC0 && gba.Within(data, table.Pointer, 4) &&
				table.Pointer+4 <= class.Address+class.Length {
				result = append(
					result,
					romwrite.Claim{
						Start: table.Pointer,
						End:   table.Pointer + 4,
						Owner: owner,
						Host:  class.Host,
						Site:  true,
					},
				)
			}
			if table.Address >= 0xC0 && table.Address < len(data) {
				result = append(
					result,
					romwrite.Claim{
						Start: table.Address,
						End:   min(table.Address+f.count, len(data)),
						Owner: owner,
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
