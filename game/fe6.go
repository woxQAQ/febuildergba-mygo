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
				Fields:   records.FE6UnitFields(),
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
		sprites: sprite.Format{
			Standing: []int{0x21C74}, Palette: 0x100968,
			Weapons: 0x750DC, WeaponPalette: 0x750E4, WeaponX: 6, WeaponY: 8,
		},
		traits:               records.FE6Traits(),
		weaponThresholds:     records.FE6WeaponThresholds(),
		classAnimationOffset: 48,
		terrain:              terrain.FE6Layout(),
		supports: support.Format{
			Layout:       support.FE6Layout(),
			TalkPointers: []int{0x6AFE0},
		},
		quotes: quote.Format{
			AnyChapter: 0x2D,
			Rules: []quote.Rule{
				{
					Kind:      quote.Battle,
					Pointers:  []int{0x6B660},
					Stride:    12,
					EndOffset: 1,
					EndWidth:  1,
					Fields:    quote.FE6BattleFields(),
				},
				{
					Kind: quote.General, Pointers: []int{0x6B744, 0x6B7A4},
					Stride: 16, EndWidth: 1, Fields: quote.FE6GeneralFields(),
				},
				{
					Kind: quote.Triangle, Pointers: []int{0x6B748, 0x6B7DC},
					Stride: 16, EndWidth: 1, Fields: quote.FE6GeneralFields(),
				},
				{
					Kind:     quote.Death,
					Pointers: []int{0x6B7FC, 0x6B88C, 0x6D2BC},
					Stride:   16,
					EndWidth: 1,
					Fields:   quote.FE6DeathFields(),
				},
			},
		},
		chapters: chapter.Format{
			Layout:            chapter.FE6Layout(),
			Pointers:          []int{0x2BB20},
			StrideInstruction: 0x2BB12,
		},
		events: event.Format{
			Layout: event.FE6Layout(),
			Root:   0x2BBCC, Defeat: 0x66AC6C, Rout: 0x0806C2F9,
		},
		shops: shop.Format{DefaultPointer: 0x95F10},
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
			Layout:  portrait.FE6Layout(),
			Pointer: 0x7FD8,
		},
		genericPortraits: portrait.GenericFormat{
			Pointer: 0x8DF0,
			Hook:    0x8DB8,
			Count:   7,
		},
		icons: itemicon.Format{
			Pointer:  0x4CEC,
			Palette:  0x4AC4,
			Original: 0xF9D80,
			Maximum:  0x9F,
		},
	}
}
