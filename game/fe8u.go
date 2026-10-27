package game

import "bytes"

// Locations follow ROMFE8U and the table initializers in the pinned legacy
// source.
func fe8uFormat() gameFormat {
	return gameFormat{
		id:      "FE8U",
		header:  "BE8E01",
		minimum: 16 << 20,
		tables: []tableRule{
			{
				kind: Units,
				pointers: []int{
					0x10108,
					0x104EC,
					0x10538,
					0xA0B40,
					0xA0BB8,
					0xBE954,
				},
				stride:  52,
				firstID: 1,
				count:   255,
				probe:   0x2C,
				fields:  unitFields(),
			},
			{
				kind:     Classes,
				pointers: []int{0x17AB8, 0x17DBC, 0x18DAC, 0x19458},
				stride:   84,
				count:    256,
				probe:    0x88,
				fields:   classFields(),
			},
			{
				kind: Items,
				pointers: []int{
					0x177C0,
					0x16570,
					0x166D0,
					0x1671C,
					0x167E0,
					0x16A10,
					0x16B18,
					0x16BB4,
				},
				stride:        36,
				count:         256,
				probe:         -1,
				pointerFields: []int{12},
				fields:        itemFields(),
			},
		},
		chapters: fe8Chapters(
			0xB5F98,
			0xB61C0,
			0xB6328,
			0xB6500,
			0x3462C,
			0xB5E68,
		),
		deployments: fe8Deployments(0x346DC, 0x9EE248),
		maps: mapFormat{
			graphics: 0x19968,
			palette:  0x1999C,
			config:   0x19900,
			layout:   0x34680,
		},
		text: textFormat{
			pointer:     0xA2A0,
			recovery:    0x15D48C,
			treePointer: 0x6E0,
			rootPointer: 0x6DC,
		},
		portraits: portraitFormat{
			pointer:     0x5524,
			stride:      28,
			cardMarker:  0,
			cardPointer: 16,
			stateOffset: 24,
			mouthOffset: 22,
			mode:        fe8uPortraitMode,
		},
		icons: iconFormat{
			pointer:  0x36B4,
			palette:  0x35D0,
			original: 0x5926F4,
			maximum:  0xDF,
		},
		features: fe8uFeatures,
	}
}

func fe8uFeatures(data []byte) (map[byte]string, []string) {
	aliases := fe8uNameAliases(data)
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
	case portraitExceed:
		patches = append(patches, "MUG_EXCEED")
	case portraitHalfBody:
		patches = append(patches, "HALFBODY")
	}
	return aliases, patches
}
