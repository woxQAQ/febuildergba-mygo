package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/chapter"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/itemicon"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
)

// Locations follow ROMFE7JP; FE7J uses Shift-JIS and its own compact name
// glyphs.
func fe7jFormat() format {
	return format{
		id:      version.FE7J,
		header:  "AE7J01",
		minimum: 16 << 20,
		tables: []records.Rule{
			{
				Kind:     records.Units,
				Pointers: []int{0x9AC50},
				Stride:   52,
				FirstID:  1,
				Count:    253,
				Probe:    0x2C,
				Fields:   records.UnitFields(version.FE7J),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x17CE0},
				Stride:   84,
				Count:    256,
				Probe:    0x88,
				Fields:   records.ClassFields(),
			},
			{
				Kind:          records.Items,
				Pointers:      []int{0x164DC},
				Stride:        36,
				Count:         256,
				Probe:         -1,
				PointerFields: []int{12, 16},
				Fields:        records.ItemFields(),
			},
		},
		chapters: chapter.FE7(0x31A6C, false),
		events:   event.FE7(0x31B18, false),
		shops:    shop.Format{DefaultPointer: 0xB12FC},
		maps: maps.Format{
			Graphics:    0x19618,
			Palette:     0x1964C,
			Config:      0x195B0,
			Layout:      0x31ABC,
			Animation:   0x2D824,
			Animation2:  0x2E364,
			Changes:     0x31AE8,
			WorldEvents: 0,
		},
		text: text.Format{
			Pointer:     0x13370,
			Recovery:    0xBBB370,
			TreePointer: 0x6E0,
			RootPointer: 0x6DC,
			ShiftJIS:    true,
			Names:       text.FE7JNames,
		},
		portraits: portrait.Format{
			Pointer:     0x69C0,
			Stride:      28,
			CardMarker:  0,
			CardPointer: 16,
			StateOffset: 24,
			MouthOffset: 22,
		},
		icons: itemicon.Format{
			Pointer:  0x4CFC,
			Palette:  0x4C1C,
			Original: 0xC12F4,
			Maximum:  0xAC,
		},
	}
}
