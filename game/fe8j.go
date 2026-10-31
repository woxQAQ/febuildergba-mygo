package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/chapter"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/itemicon"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
)

// Locations follow ROMFE8JP; field layouts are shared with FE8U.
func fe8jFormat() format {
	return format{
		id:      version.FE8J,
		header:  "BE8J01",
		minimum: 16 << 20,
		tables: []records.Rule{
			{
				Kind: records.Units,
				Pointers: []int{
					0x10650,
					0x10268,
					0x1069C,
					0xA3B64,
					0xA3CE0,
					0xA4928,
					0xA49A0,
					0xA4C3C,
					0xA5458,
					0xA56A8,
					0xA5860,
					0xA595C,
					0xA5BB0,
					0xA5D28,
					0xA61F8,
				},
				Stride:  52,
				FirstID: 1,
				Count:   255,
				Probe:   0x2C,
				Fields:  records.UnitFields(version.FE8J),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x17860, 0x17ADC, 0x18AC0, 0x19130},
				Stride:   84,
				Count:    256,
				Probe:    0x88,
				Fields:   records.ClassFields(),
			},
			{
				Kind: records.Items,
				Pointers: []int{
					0x17568,
					0x17554,
					0x1753C,
					0x17510,
					0x172BC,
				},
				Stride:        36,
				Count:         256,
				Probe:         -1,
				PointerFields: []int{12, 16},
				Fields:        records.ItemFields(),
			},
		},
		quotes:   quote.FE8(true),
		chapters: chapter.FE8(0xBAB1C, 0x34534, 0xBA9EC, 0xBAD44, 0xBAEAC),
		events:   event.FE8(0x345E4, 0xA60380, 0x5B9EE8),
		shops: shop.Format{
			DefaultPointer:     0xB8E38,
			PreparationPointer: 0x9C144,
		},
		worldmap: worldmap.Format{Pointer: 0xC8DC0},
		maps: maps.Format{
			Graphics:    0x19640,
			Palette:     0x19674,
			Config:      0x195D8,
			Layout:      0x34588,
			Animation:   0x30084,
			Animation2:  0x30BC8,
			Changes:     0x345B4,
			WorldEvents: 0,
		},
		text: text.Format{
			Pointer:     0xA000,
			Recovery:    0x14D08C,
			TreePointer: 0x6E0,
			RootPointer: 0x6DC,
			ShiftJIS:    true,
			Names:       text.FE8JNames,
		},
		portraits: portrait.Format{
			Pointer:     0x542C,
			Stride:      28,
			CardMarker:  0,
			CardPointer: 16,
			StateOffset: 24,
			MouthOffset: 22,
		},
		icons: itemicon.Format{
			Pointer:  0x36D4,
			Palette:  0x351C,
			Original: 0x5BA470,
			Maximum:  0xDF,
		},
	}
}
