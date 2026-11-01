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
				Fields:   records.FE7UnitFields(),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x17CE0},
				Stride:   84,
				Count:    256,
				Probe:    0x88,
				Fields:   records.FE7ClassFields(),
			},
			{
				Kind:          records.Items,
				Pointers:      []int{0x164DC},
				Stride:        36,
				Count:         256,
				Probe:         -1,
				PointerFields: []int{12, 16},
				Fields:        records.FE7ItemFields(),
			},
		},
		sprites: sprite.Format{
			Standing: []int{0x2522C}, Palette: 0x1900E8,
			Weapons: 0x96D78, WeaponPalette: 0x90E70, WeaponX: 3, WeaponY: 0,
		},
		traits:               records.FE7Traits(),
		weaponThresholds:     records.FE7WeaponThresholds(),
		classAnimationOffset: 52,
		terrain:              terrain.FE7Layout(),
		supports: support.Format{
			Layout: support.FE7Layout(),
			TalkPointers: []int{
				0x79264, 0x792D4, 0x79334, 0x9BB38, 0x9BB60,
				0x9F668, 0x9F730, 0x9F83C, 0x9F8AC,
			},
		},
		// Addresses follow ROMFE7JP, not a derived delta.
		quotes: quote.Format{
			AnyChapter: 0x45,
			Rules: []quote.Rule{
				{
					Kind:      quote.Battle,
					Pointers:  []int{0x79AB0},
					Stride:    16,
					EndOffset: 1,
					EndWidth:  1,
					Fields:    quote.FE7BattleFields(),
				},
				{
					Kind: quote.General, Pointers: []int{0x79C24, 0x79CAC},
					Stride: 12, EndWidth: 1, Fields: quote.FE7GeneralFields(),
				},
				{
					Kind: quote.Triangle, Pointers: []int{0x79C28, 0x79CE0},
					Stride: 12, EndWidth: 1, Fields: quote.FE7GeneralFields(),
				},
				{
					Kind: quote.Death, Pointers: []int{0x79D20, 0x79E84},
					Stride: 16, EndWidth: 1, Fields: quote.FE7DeathFields(),
				},
				{
					Kind: quote.DeathLyn, Pointers: []int{0x79D2C, 0x79E4C},
					Stride: 12, EndWidth: 1, Fields: quote.FE7FallbackFields(),
				},
				{
					Kind: quote.DeathOther, Pointers: []int{0x79D28, 0x79E48},
					Stride: 12, EndWidth: 1, Fields: quote.FE7FallbackFields(),
				},
			},
		},
		chapters: chapter.Format{
			Layout:   chapter.FE7Layout(),
			Pointers: []int{0x31A6C},
		},
		events: event.Format{
			Layout: event.FE7JLayout(),
			Root:   0x31B18, Defeat: 0xD6F71C, Rout: 0x0807AA5D,
		},
		shops: shop.Format{DefaultPointer: 0xB12FC},
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
			Layout:  portrait.FE7Layout(),
			Pointer: 0x69C0,
		},
		genericPortraits: portrait.GenericFormat{
			Pointer: 0x7450,
			Hook:    0x7428,
			Count:   6,
		},
		icons: itemicon.Format{
			Pointer:  0x4CFC,
			Palette:  0x4C1C,
			Original: 0xC12F4,
			Maximum:  0xAC,
		},
	}
}
