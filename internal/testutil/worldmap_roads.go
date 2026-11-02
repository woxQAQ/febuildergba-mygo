package testutil

// NewWorldMapRoadFixture includes duplicate writes, a run crossing a row, gaps,
// a nonzero terminator tail, and an unregistered pointer to the shared source.
func NewWorldMapRoadFixture(version string) *FixtureBuilder {
	f := NewWorldMapRouteFixture(version)
	if version != "FE8U" && version != "FE8J" {
		return f
	}
	road := f.GetPosition("worldmap_road")
	f.WriteAt(road, []byte{
		1, 1, 2, 2, 0, 0, 1, 4, 1, 8, 1, 12,
		2, 1, 1, 1, 0, 0,
		59, 2, 2, 1, 1, 0, 1, 4,
		0xFF, 0xA5, 0xB6, 0xC7,
	})
	f.WritePointer(0x258004, road)
	return f
}
