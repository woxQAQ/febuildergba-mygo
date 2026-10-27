package game

// FE6's character pointer addresses ID 0, while the editor starts at ID 1.
// Its class records have no promotion bonuses, and items have no weapon EXP byte.
func fe6Format() gameFormat {
	return gameFormat{
		id: "FE6", header: "AFEJ01", minimum: 8 << 20,
		tables: []tableRule{
			{kind: Units, pointers: []int{0x17680}, stride: 48, firstID: 1, count: 226, probe: 0x2C, skip: 48, fields: unitFields()},
			{kind: Classes, pointers: []int{0x176E0}, stride: 72, count: 256, probe: -1, fields: fe6ClassFields()},
			{kind: Items, pointers: []int{0x16410}, stride: 32, count: 256, probe: -1, pointerFields: []int{12, 16}, fields: fe6ItemFields()},
		},
		chapters:  fe6Chapters(),
		maps:      mapFormat{graphics: 0x18AE4, palette: 0x18B18, config: 0x18A7C, layout: 0x2BB70},
		text:      textFormat{pointer: 0x13B10, recovery: 0xF635C, treePointer: 0x6E0, rootPointer: 0x6DC, shiftJIS: true, names: fe6NameSymbols},
		portraits: portraitFormat{pointer: 0x7FD8, stride: 16, cardMarker: 4, cardPointer: 0, stateOffset: -1, mouthOffset: -1},
		icons:     iconFormat{pointer: 0x4CEC, palette: 0x4AC4, original: 0xF9D80, maximum: 0x9F},
	}
}
