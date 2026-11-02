package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/arena"
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
				Fields:  records.FE8UnitFields(),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x17860, 0x17ADC, 0x18AC0, 0x19130},
				Stride:   84,
				Count:    256,
				Probe:    0x88,
				Fields:   records.FE7ClassFields(),
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
				Fields:        records.FE7ItemFields(),
			},
		},
		sprites: sprite.Format{
			Standing: []int{
				0x266D4, 0x26754, 0x267DC, 0x26C2C,
				0x27274, 0x27B44, 0x27C30, 0x27FF8,
			},
			Palette: 0x5C7340,
			Weapons: 0x9FEFC, WeaponPalette: 0x93450, WeaponX: 3, WeaponY: 0,
		},
		traits:               records.FE8Traits(),
		weaponThresholds:     records.FE7WeaponThresholds(),
		classAnimationOffset: 52,
		terrain:              terrain.FE7Layout(),
		arena: arena.Format{
			Classes:     [3]int{0x31898, 0x318A4, 0x318F4},
			BaseWeapons: []int{0x31B04, 0x31F00},
			Upgrades:    0x31B24,
			LinkUnits:   0x9A164,
		},
		supports: support.Format{
			BonusPointer: 0x284C8,
			Layout:       support.FE8Layout(),
			TalkPointers: []int{0x86A50, 0x86AC8},
		},
		quotes: quote.Format{
			AnyChapter:       0xFF,
			BattleConditions: true,
			SilentDeath:      true,
			Rules: []quote.Rule{
				{
					Kind:     quote.Battle,
					Pointers: []int{0x86978},
					Stride:   16,
					EndWidth: 2,
					EndValue: 0xFFFF,
					Fields:   quote.FE8BattleFields(),
				},
				{
					Kind:     quote.Death,
					Pointers: []int{0x869F8},
					Stride:   12,
					EndWidth: 2,
					EndValue: 0xFFFF,
					Fields:   quote.FE8DeathFields(),
				},
			},
		},
		chapters: chapter.Format{
			Layout:   chapter.FE8Layout(),
			Pointers: []int{0xBAB1C, 0x34534, 0xBA9EC, 0xBAD44, 0xBAEAC},
		},
		events: event.Format{
			WorldMapEvents: event.WorldMapEventsFormat{
				Beginning:     0xBF264,
				Intro:         0xBF2B0,
				EirikaEnding:  0x9E20,
				EphraimEnding: 0x9E38,
			},
			Layout:     event.FE8Layout(),
			Root:       0x345E4,
			LoadHelper: 0xA60380,
			Defeat:     0x5B9EE8,
		},
		shops: shop.Format{
			DefaultPointer:     0xB8E38,
			PreparationPointer: 0x9C144,
		},
		worldmap: worldmap.Format{
			Pointer: 0xC8DC0, RoutePointer: 0xC598,
			Scene: worldmap.SceneFormat{
				Image:           0xBF698,
				Palette:         0xBF6A4,
				PaletteMap:      0xBF69C,
				PaletteReload:   0xBFA3C,
				PaletteFade:     0xC4590,
				DarkPalette:     0xC4594,
				RoadTiles:       0xBDE60,
				RoadTilesReload: 0xC6FF0,
				RoadPalette:     0xBDE5C,
			},
		},
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
			Layout:  portrait.FE7Layout(),
			Pointer: 0x542C,
		},
		genericPortraits: portrait.GenericFormat{
			Pointer: 0x5E98,
			Hook:    0x5E70,
			Count:   8,
		},
		icons: itemicon.Format{
			Pointer:  0x36D4,
			Palette:  0x351C,
			Original: 0x5BA470,
			Maximum:  0xDF,
		},
	}
}
