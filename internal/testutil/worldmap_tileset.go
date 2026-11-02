package testutil

// NewWorldMapTilesetFixture keeps two native tiles with flips and independent
// references. The reload consumer shares the image, but unrelated aliases do not.
func NewWorldMapTilesetFixture(version string) *FixtureBuilder {
	f := NewWorldMapRoadFixture(version)
	if version != "FE8U" && version != "FE8J" {
		return f
	}
	roots := []int{0xB8F98, 0xC2238, 0xB8F94}
	if version == "FE8J" {
		roots = []int{0xBDE60, 0xC6FF0, 0xBDE5C}
	}
	for i, name := range []string{"road_tiles", "road_tiles_reload", "road_palette"} {
		f.MarkPosition(name+"_pointer", roots[i])
	}
	f.WritePointer(roots[1], f.GetPosition("worldmap_road_tiles"))
	f.WritePointer(0x258030, f.GetPosition("worldmap_road_tiles"))
	f.WritePointer(0x258034, f.GetPosition("worldmap_road_palette"))
	palette := f.GetPosition("worldmap_road_palette")
	f.WriteU16(palette, 0x9234)
	f.WriteU16(palette+8, 0x83FF)
	return f
}
