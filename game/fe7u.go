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
	"github.com/woxQAQ/febuildergba-mygo/game/sprite"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
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
				Fields:   records.FE7UnitFields(),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x178F0},
				Stride:   84,
				Count:    256,
				Probe:    0x88,
				Fields:   records.FE7ClassFields(),
			},
			{
				Kind:          records.Items,
				Pointers:      []int{0x174D4},
				Stride:        36,
				Count:         256,
				Probe:         -1,
				PointerFields: []int{12, 16},
				Fields:        records.FE7ItemFields(),
			},
		},
		sprites: sprite.Format{
			Standing: []int{0x24DA0}, Palette: 0x194594,
			Weapons: 0x965F4, WeaponPalette: 0x90530, WeaponX: 3, WeaponY: 0,
		},
		traits:               records.FE7Traits(),
		weaponThresholds:     records.FE7WeaponThresholds(),
		classAnimationOffset: 52,
		terrain:              terrain.FE7Layout(),
		supports: support.Format{
			Layout: support.FE7Layout(),
			TalkPointers: []int{
				0x78A94, 0x78B04, 0x78B64, 0x9B164, 0x9B18C,
				0x9EC2C, 0x9ECF4, 0x9EE00, 0x9EE70,
			},
		},
		quotes: quote.Format{
			AnyChapter: 0x43,
			Rules: []quote.Rule{
				{
					Kind:      quote.Battle,
					Pointers:  []int{0x792E0},
					Stride:    16,
					EndOffset: 1,
					EndWidth:  1,
					Fields:    quote.FE7BattleFields(),
				},
				{
					Kind: quote.General, Pointers: []int{0x79454, 0x794DC},
					Stride: 12, EndWidth: 1, Fields: quote.FE7GeneralFields(),
				},
				{
					Kind: quote.Triangle, Pointers: []int{0x79458, 0x79510},
					Stride: 12, EndWidth: 1, Fields: quote.FE7GeneralFields(),
				},
				{
					Kind: quote.Death, Pointers: []int{0x79550, 0x796B4},
					Stride: 16, EndWidth: 1, Fields: quote.FE7DeathFields(),
				},
				{
					Kind: quote.DeathLyn, Pointers: []int{0x7955C, 0x7967C},
					Stride: 12, EndWidth: 1, Fields: quote.FE7FallbackFields(),
				},
				{
					Kind: quote.DeathOther, Pointers: []int{0x79558, 0x79678},
					Stride: 12, EndWidth: 1, Fields: quote.FE7FallbackFields(),
				},
			},
		},
		chapters: chapter.Format{
			Layout:   chapter.FE7ULayout(),
			Pointers: []int{0x31580},
		},
		events: event.Format{
			Layout: event.FE7ULayout(),
			Root:   0x315D4, Defeat: 0xCA749C, Rout: 0x08079FE9,
		},
		shops: shop.Format{DefaultPointer: 0xB04D4},
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
			Layout:  portrait.FE7Layout(),
			Pointer: 0x6B30,
		},
		genericPortraits: portrait.GenericFormat{
			Pointer: 0x75C0,
			Hook:    0x7598,
			Count:   6,
		},
		icons: itemicon.Format{
			Pointer:  0x4E20,
			Palette:  0x4D40,
			Original: 0xC5EA4,
			Maximum:  0xAC,
		},
	}
}
