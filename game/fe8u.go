package game

import (
	"bytes"

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
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// Locations follow ROMFE8U and the table initializers in the pinned legacy
// source.
func fe8uFormat() format {
	return format{
		id:      version.FE8U,
		header:  "BE8E01",
		minimum: 16 << 20,
		tables: []records.Rule{
			{
				Kind: records.Units,
				Pointers: []int{
					0x10108,
					0x104EC,
					0x10538,
					0xA0B40,
					0xA0BB8,
					0xBE954,
				},
				Stride:  52,
				FirstID: 1,
				Count:   255,
				Probe:   0x2C,
				Fields:  records.UnitFields(version.FE8U),
			},
			{
				Kind:     records.Classes,
				Pointers: []int{0x17AB8, 0x17DBC, 0x18DAC, 0x19458},
				Stride:   84,
				Count:    256,
				Probe:    0x88,
				Fields:   records.ClassFields(),
			},
			{
				Kind: records.Items,
				Pointers: []int{
					0x177C0,
					0x16570,
					0x166D0,
					0x1671C,
					0x167E0,
					0x16A10,
					0x16B18,
					0x16BB4,
				},
				Stride:        36,
				Count:         256,
				Probe:         -1,
				PointerFields: []int{12},
				Fields:        records.ItemFields(),
			},
		},
		sprites: sprite.Format{
			Standing: []int{
				0x26730,
				0x267B0,
				0x26838,
				0x26C88,
				0x28064,
				0x27F74,
			},
			Palette:       0x59EE20,
			Weapons:       0x9DC5C,
			WeaponPalette: 0x91178,
			WeaponX:       3,
			WeaponY:       0,
		},
		classAnimationOffset: 52,
		terrain:              terrain.FE7Layout(),
		supports: support.Format{
			Layout:       support.FE8Layout(),
			TalkPointers: []int{0x84784, 0x847FC},
		},
		quotes: quote.Format{
			AnyChapter:       0xFF,
			BattleConditions: true,
			SilentDeath:      true,
			Rules: []quote.Rule{
				{
					Kind:     quote.Battle,
					Pointers: []int{0x846AC},
					Stride:   16,
					EndWidth: 2,
					EndValue: 0xFFFF,
					Fields:   quote.FE8BattleFields(),
				},
				{
					Kind:     quote.Death,
					Pointers: []int{0x8472C},
					Stride:   12,
					EndWidth: 2,
					EndValue: 0xFFFF,
					Fields:   quote.FE8DeathFields(),
				},
			},
		},
		chapters: chapter.Format{
			Layout: chapter.FE8Layout(),
			Pointers: []int{
				0xB5F98, 0xB61C0, 0xB6328, 0xB6500, 0x3462C, 0xB5E68,
			},
		},
		events: event.Format{
			Layout: event.FE8Layout(),
			Root:   0x346DC, LoadHelper: 0x9EE248, Defeat: 0x592104,
		},
		shops: shop.Format{
			DefaultPointer:     0xB42B0,
			PreparationPointer: 0x99E64,
		},
		worldmap: worldmap.Format{Pointer: 0xBE84},
		maps: maps.Format{
			Graphics:    0x19968,
			Palette:     0x1999C,
			Config:      0x19900,
			Layout:      0x34680,
			Animation:   0x30134,
			Animation2:  0x30C78,
			Changes:     0x346AC,
			WorldEvents: 0,
		},
		text: text.Format{
			Pointer:     0xA2A0,
			Recovery:    0x15D48C,
			TreePointer: 0x6E0,
			RootPointer: 0x6DC,
		},
		portraits: portrait.Format{
			Layout:  portrait.FE7Layout(),
			Pointer: 0x5524,
			Mode:    fe8uPortraitMode,
		},
		genericPortraits: portrait.GenericFormat{
			Pointer: 0x5F94,
			Hook:    0x5F6C,
			Count:   8,
		},
		icons: itemicon.Format{
			Pointer:  0x36B4,
			Palette:  0x35D0,
			Original: 0x5926F4,
			Maximum:  0xDF,
		},
		features: fe8uFeatures,
	}
}

func fe8uFeatures(data []byte) (map[byte]string, []string) {
	aliases := text.FE8UAliases(data)
	var patches []string
	if aliases[0x9A] == "z" {
		patches = append(patches, "NarrowFont")
	}
	if bytes.Equal(data[0x2ACF8:0x2ACFA], []byte{0x70, 0x47}) {
		patches = append(patches, "SkillSystem")
	}
	if bytes.Equal(
		data[0x2BB44:0x2BB4A],
		[]byte{1, 0x4B, 0xA5, 0xF0, 0xC1, 0xFE},
	) ||
		bytes.Equal(
			data[0x17E5C:0x17E62],
			[]byte{1, 0x4B, 0xB9, 0xF0, 0x35, 0xFD},
		) {
		patches = append(patches, "Str/Mag split")
	}
	switch fe8uPortraitMode(data) {
	case portrait.Exceed:
		patches = append(patches, "MUG_EXCEED")
	case portrait.HalfBody:
		patches = append(patches, "HALFBODY")
	}
	return aliases, patches
}

// fe8uPortraitMode detects the FE8U portrait patches by their native
// instruction signatures.
func fe8uPortraitMode(data []byte) portrait.Mode {
	if gba.Within(data, 0x55D2, 6) &&
		bytes.Equal(
			data[0x55D2:0x55D8],
			[]byte{0xC0, 0x46, 0x01, 0xB0, 0x03, 0x4B},
		) {
		return portrait.Exceed
	}
	if gba.Within(data, 0x8540, 2) &&
		(gba.Uint16(data, 0x8540) == 0x1C0A || gba.Uint16(data, 0x8540) == 0x3A01) {
		return portrait.HalfBody
	}
	return portrait.Standard
}
