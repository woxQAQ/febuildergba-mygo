package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/chapter"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/itemicon"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
)

// FE6's character pointer addresses ID 0, while the editor starts at ID 1.
// Its class records have no promotion bonuses, and items have no weapon EXP
// byte.
func fe6Format() format {
	return format{
		id:      version.FE6,
		header:  "AFEJ01",
		minimum: 8 << 20,
		tables: []records.Rule{
			{
				Kind:     records.Units,
				Pointers: []int{0x17680},
				Stride:   48,
				FirstID:  1,
				Count:    226,
				Probe:    0x2C,
				Skip:     48,
				Fields:   records.UnitFields(version.FE6),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x176E0},
				Stride:   72,
				Count:    256,
				Probe:    -1,
				Fields:   records.FE6ClassFields(),
			},
			{
				Kind:          records.Items,
				Pointers:      []int{0x16410},
				Stride:        32,
				Count:         256,
				Probe:         -1,
				PointerFields: []int{12, 16},
				Fields:        records.FE6ItemFields(),
			},
		},
		chapters: chapter.FE6(),
		events:   event.FE6(0x2BBCC),
		maps: maps.Format{
			Graphics:    0x18AE4,
			Palette:     0x18B18,
			Config:      0x18A7C,
			Layout:      0x2BB70,
			Animation:   0x28344,
			Animation2:  0x28E7C,
			Changes:     0x2BB9C,
			WorldEvents: 0x9310C,
		},
		text: text.Format{
			Pointer:     0x13B10,
			Recovery:    0xF635C,
			TreePointer: 0x6E0,
			RootPointer: 0x6DC,
			ShiftJIS:    true,
			Names:       text.FE6Names,
		},
		portraits: portrait.Format{
			Pointer:     0x7FD8,
			Stride:      16,
			CardMarker:  4,
			CardPointer: 0,
			StateOffset: -1,
			MouthOffset: -1,
		},
		icons: itemicon.Format{
			Pointer:  0x4CEC,
			Palette:  0x4AC4,
			Original: 0xF9D80,
			Maximum:  0x9F,
		},
	}
}
