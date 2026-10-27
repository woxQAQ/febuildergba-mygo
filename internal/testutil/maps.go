package testutil

import "encoding/binary"

// NewMapFixture adds independently specified pointer lists and two small maps.
// The first metatile exercises both flip bits and four palette banks. The
// second uses a tile held in a separate graphics resource when it is selected.
func NewMapFixture(version string) *FixtureBuilder {
	var f *FixtureBuilder
	var roots [4]int
	switch version {
	case "FE8U":
		f, roots = NewMinimalFE8U(), [4]int{0x19968, 0x1999C, 0x19900, 0x34680}
	case "FE8J":
		f, roots = NewMinimalFE8J(), [4]int{0x19640, 0x19674, 0x195D8, 0x34588}
	case versionFE7U:
		f, roots = NewMinimalFE7U(), [4]int{0x19230, 0x19264, 0x191C8, 0x3159C}
	case versionFE7J:
		f, roots = NewMinimalFE7J(), [4]int{0x19618, 0x1964C, 0x195B0, 0x31ABC}
	case versionFE6:
		f, roots = NewMinimalFE6(), [4]int{0x18AE4, 0x18B18, 0x18A7C, 0x2BB70}
	default:
		panic("unknown map fixture version: " + version)
	}
	for i, name := range []string{"graphics", "palette", "config", "layout"} {
		base := 0x220000 + i*0x400
		f.WritePointer(roots[i], base)
		f.MarkPosition("map_"+name+"_pointer", roots[i])
		f.MarkPosition("map_"+name+"_list", base)
	}
	graphics := make([]byte, 64)
	for i := range 32 {
		graphics[i], graphics[i+32] = 0x11, 0x55
	}
	graphics[0], graphics[3], graphics[28], graphics[31] = 0x01, 0x21, 0x13, 0x41
	f.WriteAt(0x230000, literalLZ(graphics))
	f.WriteAt(0x230100, literalLZ(graphics[:32]))
	f.WriteAt(0x230200, literalLZ(graphics[32:]))
	f.WritePointer(f.GetPosition("map_graphics_list")+4, 0x230000)
	f.WritePointer(f.GetPosition("map_graphics_list")+5*4, 0x230100)
	f.WritePointer(f.GetPosition("map_graphics_list")+8*4, 0x230200)
	f.MarkPosition("map_graphics", 0x230000)

	// Color zero is cyan and remains opaque. Other entries distinguish the
	// source corners, the palette bank and the second graphics resource.
	for bank := range 16 {
		for index, value := range []uint16{0x7FE0, 0x001F, 0x03E0, 0x7C00, 0x7FFF, 0x7C1F} {
			f.WriteU16(0x231000+bank*32+index*2, value)
		}
		f.WriteU16(0x231000+bank*32+2, uint16(bank+1)*0x001F)
	}
	f.WriteAt(0x231200, f.data[0x231000:0x231200])
	f.WriteU16(
		0x231202,
		0x03E0,
	) // Palette 7 changes the first red pixel to green.
	f.WritePointer(f.GetPosition("map_palette_list")+2*4, 0x231000)
	f.WritePointer(f.GetPosition("map_palette_list")+7*4, 0x231200)
	f.MarkPosition("map_palette", 0x231000)

	config := make([]byte, 0x2400)
	for i, word := range []uint16{0x0000, 0x1400, 0x2800, 0x3C00, 1, 1, 1, 1} {
		binary.LittleEndian.PutUint16(config[i*2:], word)
	}
	config[0x2000], config[0x2001] = 0x01, 0x21
	f.WriteAt(0x232000, literalLZ(config))
	f.WritePointer(f.GetPosition("map_config_list")+3*4, 0x232000)
	f.MarkPosition("map_config", 0x232000)

	f.WriteAt(0x236000, literalLZ([]byte{2, 1, 0, 0, 4, 0}))
	f.WriteAt(0x236010, literalLZ([]byte{1, 2, 4, 0, 0, 0}))
	f.WritePointer(f.GetPosition("map_layout_list")+4*4, 0x236000)
	f.WritePointer(f.GetPosition("map_layout_list")+6*4, 0x236010)
	f.MarkPosition("map_layout", 0x236000)
	base := f.GetPosition("chapters")
	stride := 148
	switch version {
	case versionFE7U:
		stride = 152
	case versionFE6:
		stride = 68
	}
	for i := range 2 {
		f.WriteAt(base+i*stride+4, []byte{1, 0, 2, 3, 4})
	}
	f.WriteU8(base+stride+8, 6)
	return f
}

// Literal-only streams keep expected pixels independent of our LZ77 encoder.
func literalLZ(data []byte) []byte {
	out := []byte{
		0x10,
		byte(len(data) & 0xFF),
		byte((len(data) >> 8) & 0xFF),
		byte((len(data) >> 16) & 0xFF),
	}
	for at := 0; at < len(data); at += 8 {
		out = append(out, 0)
		out = append(out, data[at:min(at+8, len(data))]...)
	}
	return out
}
