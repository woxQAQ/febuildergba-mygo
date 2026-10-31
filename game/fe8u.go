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
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
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
		supports: support.FE8(false),
		quotes:   quote.FE8(false),
		chapters: chapter.FE8(
			0xB5F98,
			0xB61C0,
			0xB6328,
			0xB6500,
			0x3462C,
			0xB5E68,
		),
		events: event.FE8(0x346DC, 0x9EE248, 0x592104),
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
			Pointer:     0x5524,
			Stride:      28,
			CardMarker:  0,
			CardPointer: 16,
			StateOffset: 24,
			MouthOffset: 22,
			Mode:        portrait.FE8UMode,
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
	switch portrait.FE8UMode(data) {
	case portrait.Exceed:
		patches = append(patches, "MUG_EXCEED")
	case portrait.HalfBody:
		patches = append(patches, "HALFBODY")
	}
	return aliases, patches
}
