package testutil

// NewWorldMapBackgroundFixture includes all four banks, opaque color zero,
// unused palette bits, row padding and independent references to each source.
func NewWorldMapBackgroundFixture(version string) *FixtureBuilder {
	f := NewWorldMapRouteFixture(version)
	if version != "FE8U" && version != "FE8J" {
		return f
	}
	roots := []int{0xBA7FC, 0xBA808, 0xBA800}
	if version == "FE8J" {
		roots = []int{0xBF698, 0xBF6A4, 0xBF69C}
	}
	for i, name := range []string{"worldmap_image", "worldmap_palette", "worldmap_palette_map"} {
		f.MarkPosition(name+"_pointer", roots[i])
		f.WritePointer(0x258010+i*4, f.GetPosition(name))
	}
	palette := f.GetPosition("worldmap_palette")
	additional := []int{0xBABEC, 0xBF7AC, 0xBF7B0}
	if version == "FE8J" {
		additional = []int{0xBFA3C, 0xC4590, 0xC4594}
	}
	for i, name := range []string{"worldmap_palette_reload", "worldmap_palette_fade", "worldmap_palette_dark"} {
		f.MarkPosition(name+"_pointer", additional[i])
		f.WritePointer(additional[i], palette)
	}
	// Even an initially aliased dark palette remains an independent consumer.
	for i := range 64 {
		f.WriteU16(palette+i*2, uint16(i&31|(i>>5)<<5|(i%2)<<15))
	}
	banks := make([]byte, 1280)
	banks[0], banks[32], banks[39*32] = 0x10, 2, 3
	for y := range 40 {
		banks[y*32+30], banks[y*32+31] = 0xA5, 0xD7
	}
	f.WriteAt(
		f.GetPosition("worldmap_palette_map"),
		append(literalLZ(banks), 0xDE, 0xAD, 0xBE, 0xEF),
	)
	return f
}
