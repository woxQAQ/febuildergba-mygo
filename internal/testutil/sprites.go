package testutil

import (
	"bytes"

	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// AddSpriteResources supplies all three native standing shapes and eight
// distinct weapon symbols. Later animation frames have a different palette index.
func AddSpriteResources(f *FixtureBuilder, version string) {
	var site, palette, weapons, weaponPalette, x, y int
	switch version {
	case "FE6":
		site, palette, weapons, weaponPalette, x, y = 0x21C74, 0x100968, 0x750DC, 0x750E4, 6, 8
	case "FE7U":
		site, palette, weapons, weaponPalette, x = 0x24DA0, 0x194594, 0x965F4, 0x90530, 3
	case "FE7J":
		site, palette, weapons, weaponPalette, x = 0x2522C, 0x1900E8, 0x96D78, 0x90E70, 3
	case "FE8U":
		site, palette, weapons, weaponPalette, x = 0x26730, 0x59EE20, 0x9DC5C, 0x91178, 3
	case "FE8J":
		site, palette, weapons, weaponPalette, x = 0x266D4, 0x5C7340, 0x9FEFC, 0x93450, 3
	default:
		panic("unknown fixture version")
	}
	f.WritePointer(site, 0x3C0000)
	f.MarkPosition("standing_pointer", site)
	f.MarkPosition("standing_table", 0x3C0000)
	f.MarkPosition("standing_palette", palette)
	colors := []uint16{
		0,
		31,
		31 << 5,
		31 << 10,
		0x7FFF,
		0x3FF,
		0x7C1F,
		0x7FE0,
		0x4210,
	}
	for i, value := range colors {
		f.WriteU16(palette+i*2, value)
	}
	for i, size := range []int{128, 256, 512} {
		f.WriteU16(0x3C0000+i*8+2, uint16(i))
		at := 0x3C1000 + i*0x1000
		f.WritePointer(0x3C0000+i*8+4, at)
		tiles := bytes.Repeat([]byte{0x44}, size*3)
		copy(tiles, bytes.Repeat([]byte{byte(i+1) * 0x11}, size))
		tiles[0] &= 0xF0
		compressed, err := gba.CompressLZ77(tiles)
		if err != nil {
			panic(err)
		}
		f.WriteAt(at, compressed)
	}
	f.WritePointer(weapons, 0x3C5000)
	f.WritePointer(weaponPalette, palette)
	f.MarkPosition("weapon_pointer", weapons)
	f.MarkPosition("weapon_data", 0x3C5000)
	tiles := make([]byte, 256*(16+y)/2)
	for id := range 8 {
		for row := range 2 {
			for col := range 2 {
				at := ((y/8+row)*32 + (x+id)*2 + col) * 32
				copy(tiles[at:], bytes.Repeat([]byte{byte(id+1) * 0x11}, 32))
			}
		}
	}
	compressed, err := gba.CompressLZ77(tiles)
	if err != nil {
		panic(err)
	}
	f.WriteAt(0x3C5000, compressed)
}
