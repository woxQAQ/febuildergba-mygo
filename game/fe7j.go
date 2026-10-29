package game

// Locations follow ROMFE7JP; FE7J uses Shift-JIS and its own compact name
// glyphs.
func fe7jFormat() gameFormat {
	return gameFormat{
		id:      fe7JID,
		header:  "AE7J01",
		minimum: 16 << 20,
		tables: []tableRule{
			{
				kind:     Units,
				pointers: []int{0x9AC50},
				stride:   52,
				firstID:  1,
				count:    253,
				probe:    0x2C,
				fields:   unitFields(fe7JID),
			},
			{
				kind:     Classes,
				pointers: []int{0x17CE0},
				stride:   84,
				count:    256,
				probe:    0x88,
				fields:   classFields(),
			},
			{
				kind:          Items,
				pointers:      []int{0x164DC},
				stride:        36,
				count:         256,
				probe:         -1,
				pointerFields: []int{12, 16},
				fields:        itemFields(),
			},
		},
		chapters:    fe7Chapters(0x31A6C, false),
		deployments: fe7Deployments(0x31B18, eventFE7J),
		maps: mapFormat{
			graphics:    0x19618,
			palette:     0x1964C,
			config:      0x195B0,
			layout:      0x31ABC,
			animation:   0x2D824,
			animation2:  0x2E364,
			changes:     0x31AE8,
			worldEvents: 0,
		},
		text: textFormat{
			pointer:     0x13370,
			recovery:    0xBBB370,
			treePointer: 0x6E0,
			rootPointer: 0x6DC,
			shiftJIS:    true,
			names:       fe7JNameSymbols,
		},
		portraits: portraitFormat{
			pointer:     0x69C0,
			stride:      28,
			cardMarker:  0,
			cardPointer: 16,
			stateOffset: 24,
			mouthOffset: 22,
		},
		icons: iconFormat{
			pointer:  0x4CFC,
			palette:  0x4C1C,
			original: 0xC12F4,
			maximum:  0xAC,
		},
	}
}
