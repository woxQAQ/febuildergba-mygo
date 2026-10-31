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

// Locations and the 253-character limit follow ROMFE7U and UnitFE7Form.
func fe7uFormat() format {
	return format{
		id:      version.FE7U,
		header:  "AE7E01",
		minimum: 16 << 20,
		tables: []records.Rule{
			{
				Kind:     records.Units,
				Pointers: []int{0x9A274},
				Stride:   52,
				FirstID:  1,
				Count:    253,
				Probe:    0x2C,
				Fields:   records.UnitFields(version.FE7U),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x178F0},
				Stride:   84,
				Count:    256,
				Probe:    0x88,
				Fields:   records.ClassFields(),
			},
			{
				Kind:          records.Items,
				Pointers:      []int{0x174D4},
				Stride:        36,
				Count:         256,
				Probe:         -1,
				PointerFields: []int{12, 16},
				Fields:        records.ItemFields(),
			},
		},
		chapters: chapter.FE7(0x31580, true),
		events:   event.FE7(0x315D4, true),
		shops:    shop.Format{DefaultPointer: 0xB04D4},
		maps: maps.Format{
			Graphics:    0x19230,
			Palette:     0x19264,
			Config:      0x191C8,
			Layout:      0x3159C,
			Animation:   0x2D364,
			Animation2:  0x2DEA4,
			Changes:     0x315B8,
			WorldEvents: 0,
		},
		text: text.Format{
			Pointer:     0x12CB8,
			Recovery:    0xB808AC,
			TreePointer: 0x6BC,
			RootPointer: 0x6B8,
		},
		portraits: portrait.Format{
			Pointer:     0x6B30,
			Stride:      28,
			CardMarker:  0,
			CardPointer: 16,
			StateOffset: 24,
			MouthOffset: 22,
		},
		icons: itemicon.Format{
			Pointer:  0x4E20,
			Palette:  0x4D40,
			Original: 0xC5EA4,
			Maximum:  0xAC,
		},
	}
}
