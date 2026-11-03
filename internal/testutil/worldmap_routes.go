package testutil

import "bytes"

// NewWorldMapRouteFixture adds a fixed route graph, shared movement, a null path,
// nonzero padding, a noncanonical negative terminator and independent graphics.
func NewWorldMapRouteFixture(version string) *FixtureBuilder {
	f := NewWorldMapFixture(version)
	if version != "FE8U" && version != "FE8J" {
		return f
	}
	roots := []int{0xC2C0, 0xBA7FC, 0xBA808, 0xBA800, 0xB8F98, 0xB8F94}
	if version == "FE8J" {
		roots = []int{0xC598, 0xBF698, 0xBF6A4, 0xBF69C, 0xBDE60, 0xBDE5C}
	}
	const routes, road, movement = 0x257000, 0x257800, 0x257900
	const pixels, palette, banks, tiles, roadPalette = 0x260000, 0x273000, 0x274000, 0x275000, 0x275100
	for i, at := range []int{routes, pixels, palette, banks, tiles, roadPalette} {
		f.WritePointer(roots[i], at)
	}
	for name, at := range map[string]int{"worldmap_routes": routes, "worldmap_routes_pointer": roots[0], "worldmap_road": road, "worldmap_movement": movement, "worldmap_image": pixels, "worldmap_palette": palette, "worldmap_palette_map": banks, "worldmap_road_tiles": tiles, "worldmap_road_palette": roadPalette} {
		f.MarkPosition(name, at)
	}
	for i := range 29 {
		at := f.GetPosition("worldmap_nodes") + i*32
		f.WriteU16(at+24, uint16(16+(i%8)*48))
		f.WriteU16(at+26, uint16(16+(i/8)*64))
	}
	for i := range 32 {
		at := routes + i*12
		f.WritePointer(at, road)
		a, b := i, (i+1)%29
		if i >= 29 {
			a, b = i-29, i-26
		}
		f.WriteAt(
			at+4,
			[]byte{byte(a), byte(b), 0xC3, 0xD7},
		) // #nosec G115 -- Fixture endpoints are bounded to the 29 native nodes.
		f.WriteU32(at+8, 0)
	}
	f.WritePointer(routes+8, movement)
	f.WritePointer(routes+12+8, movement)
	f.WritePointer(
		0x258000,
		movement,
	) // An independent, unregistered reference.
	f.WriteAt(movement, []byte{
		0, 4, 0, 0, 0xFE, 0xFF, 24, 0,
		0, 12, 0, 0, 48, 0, 40, 0,
		0xFE, 0xFF, 0xFF, 0xFF, 0x91, 0xA2, 0xB3, 0xC4,
	})
	// A repeated run writes four contiguous cells, not two rows. All flip
	// combinations are represented and transparent pixels remain independent.
	f.WriteAt(road, []byte{1, 1, 2, 2, 0, 0, 1, 4, 1, 8, 1, 12, 0xFF})
	f.WriteAt(pixels, bytes.Repeat([]byte{0x11}, 480*320/2))
	for bank, color := range []uint16{0x001F, 0x03E0, 0x7C00, 0x7FFF} {
		f.WriteU16(palette+bank*32+2, color)
	}
	mapData := make([]byte, 1280)
	mapData[0] = 0x10
	mapData[32] = 2
	mapData[39*32] = 3
	f.WriteAt(banks, literalLZ(mapData))
	pixelsData := make([]byte, 64)
	pixelsData[0], pixelsData[32], pixelsData[63] = 1, 2, 0x30
	f.WriteAt(tiles, literalLZ(pixelsData))
	f.WriteU16(roadPalette+2, 0x001F)
	f.WriteU16(roadPalette+4, 0x03E0)
	f.WriteU16(roadPalette+6, 0x7C00)
	return f
}
