package game

// Locations and the 253-character limit follow ROMFE7U and UnitFE7Form.
func fe7uFormat() gameFormat {
	return gameFormat{
		id:      fe7UID,
		header:  "AE7E01",
		minimum: 16 << 20,
		tables: []tableRule{
			{
				kind:     Units,
				pointers: []int{0x9A274},
				stride:   52,
				firstID:  1,
				count:    253,
				probe:    0x2C,
				fields:   unitFields(fe7UID),
			},
			{
				kind:     Classes,
				pointers: []int{0x178F0},
				stride:   84,
				count:    256,
				probe:    0x88,
				fields:   classFields(),
			},
			{
				kind:          Items,
				pointers:      []int{0x174D4},
				stride:        36,
				count:         256,
				probe:         -1,
				pointerFields: []int{12, 16},
				fields:        itemFields(),
			},
		},
		chapters:    fe7Chapters(0x31580, true),
		deployments: fe7Deployments(0x315D4, eventFE7U),
		maps: mapFormat{
			graphics: 0x19230,
			palette:  0x19264,
			config:   0x191C8,
			layout:   0x3159C,
		},
		text: textFormat{
			pointer:     0x12CB8,
			recovery:    0xB808AC,
			treePointer: 0x6BC,
			rootPointer: 0x6B8,
		},
		portraits: portraitFormat{
			pointer:     0x6B30,
			stride:      28,
			cardMarker:  0,
			cardPointer: 16,
			stateOffset: 24,
			mouthOffset: 22,
		},
		icons: iconFormat{
			pointer:  0x4E20,
			palette:  0x4D40,
			original: 0xC5EA4,
			maximum:  0xAC,
		},
	}
}
