package testutil

// NewWorldMapFixture keeps native shop sharing and adds independently specified
// signed coordinates, route branches, distinct icons and reserved ship bits.
func NewWorldMapFixture(version string) *FixtureBuilder {
	f := NewNativeShopFixture(version)
	if version != "FE8U" && version != "FE8J" {
		return f
	}
	at := f.GetPosition("worldmap_nodes")
	f.WriteAt(at, []byte{3, 2, 5, 15, 0, 1, 0xFF, 0xFF, 2, 3, 4, 0xFF})
	f.WriteAt(at+24, []byte{0xFE, 0xFF, 0x34, 0x12, 1, 0, 0xA4, 0xD7})
	return f
}
