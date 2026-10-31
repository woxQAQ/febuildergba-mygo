package testutil

// NewNativeShopFixture includes shared chapter/default stock, a null chapter
// fallback, FE8's preparation fallback, and all native world-map slots.
func NewNativeShopFixture(version string) *FixtureBuilder {
	f := NewShopFixture(version)
	defaults := map[string]int{
		"FE6":  0x95F10,
		"FE7U": 0xB04D4,
		"FE7J": 0xB12FC,
		"FE8U": 0xB42B0,
		"FE8J": 0xB8E38,
	}
	inventory := f.GetPosition("location_inventory")
	f.WritePointer(defaults[version], inventory)
	f.MarkPosition("shop_default_pointer", defaults[version])
	f.WriteU32(f.GetPosition("location_table")+11*12+4, 0)
	if version != "FE8U" && version != "FE8J" {
		return f
	}
	root, prep := 0xBE84, 0x99E64
	if version == "FE8J" {
		root, prep = 0xC8DC0, 0x9C144
	}
	const nodes, empty, unusual = 0x255000, 0x256000, 0x256100
	f.WritePointer(root, nodes)
	f.WriteU32(prep, 0)
	f.MarkPosition("shop_preparation_pointer", prep)
	f.MarkPosition("worldmap_nodes_pointer", root)
	f.MarkPosition("worldmap_nodes", nodes)
	for i := range 29 {
		at := nodes + i*32
		f.WriteAt(at, []byte{3, 2, 0xA3, 0xB4, 1, 2, 0xCC, 0xDD, 3, 4, 5, 6})
		for slot := range 3 {
			f.WritePointer(at+12+slot*4, empty)
		}
		f.WriteAt(at+24, []byte{0x11, 0x22, 0x33, 0x44, 1, 0, 0xEE, 0xFF})
	}
	f.WritePointer(nodes+12, inventory)
	f.WriteU32(nodes+20, 0)
	f.WritePointer(nodes+32+20, unusual)
	f.WriteAt(unusual, []byte{0, 0xC3})
	return f
}
