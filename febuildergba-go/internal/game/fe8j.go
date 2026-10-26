package game

// Locations follow ROMFE8JP; field layouts are shared with FE8U.
func fe8jFormat() gameFormat {
	return gameFormat{
		id: "FE8J", header: "BE8J01", minimum: 16 << 20,
		tables: []tableRule{
			{kind: Units, pointers: []int{0x10650, 0x10268, 0x1069C, 0xA3B64, 0xA3CE0, 0xA4928, 0xA49A0, 0xA4C3C, 0xA5458, 0xA56A8, 0xA5860, 0xA595C, 0xA5BB0, 0xA5D28, 0xA61F8}, stride: 52, firstID: 1, count: 255, probe: 0x2C, fields: unitFields()},
			{kind: Classes, pointers: []int{0x17860, 0x17ADC, 0x18AC0, 0x19130}, stride: 84, count: 256, probe: 0x88, fields: classFields()},
			{kind: Items, pointers: []int{0x17568, 0x17554, 0x1753C, 0x17510, 0x172BC}, stride: 36, count: 256, probe: -1, pointerFields: []int{12, 16}, fields: itemFields()},
		},
		text:      textFormat{pointer: 0xA000, recovery: 0x14D08C, treePointer: 0x6E0, rootPointer: 0x6DC, shiftJIS: true, names: fe8JNameSymbols},
		portraits: portraitFormat{pointer: 0x542C, stride: 28, cardMarker: 0, cardPointer: 16, stateOffset: 24, mouthOffset: 22},
		icons:     iconFormat{pointer: 0x36D4, palette: 0x351C, original: 0x5BA470, maximum: 0xDF},
	}
}
