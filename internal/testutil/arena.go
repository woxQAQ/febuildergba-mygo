package testutil

// NewArenaFixture has weighted class pools, shared and interior references,
// native weapon buffers and link slots beyond an early zero. Addresses and
// fixed bytes are independent of production format configuration.
func NewArenaFixture(version string) *FixtureBuilder {
	var f *FixtureBuilder
	var pointers [][]int
	stride, flags := 84, 40
	switch version {
	case "FE6":
		f = NewMinimalFE6()
		stride, flags = 72, 36
		pointers = [][]int{
			{0x299FC},
			{0x29A08},
			{0x29A58},
			{0x29C44},
			{0x29C64},
		}
	case "FE7U":
		f = NewMinimalFE7U()
		pointers = [][]int{
			{0x2EB58},
			{0x2EB64},
			{0x2EBB4},
			{0x2EDBC, 0x2F1C0},
			{0x2EDDC},
		}
	case "FE7J":
		f = NewMinimalFE7J()
		pointers = [][]int{
			{0x2F024},
			{0x2F030},
			{0x2F080},
			{0x2F288, 0x2F68C},
			{0x2F2A8},
		}
	case "FE8U":
		f = NewMinimalFE8U()
		pointers = [][]int{
			{0x3194C},
			{0x31958},
			{0x319A8},
			{0x31BB8, 0x31FB4},
			{0x31BD8},
			{0x97E8C},
		}
	case "FE8J":
		f = NewMinimalFE8J()
		pointers = [][]int{
			{0x31898},
			{0x318A4},
			{0x318F4},
			{0x31B04, 0x31F00},
			{0x31B24},
			{0x9A164},
		}
	default:
		panic("unknown arena fixture version")
	}
	for i := 1; i <= 3; i++ {
		at := f.GetPosition("classes") + i*stride
		f.WriteU8(at+4, byte(i))
		if i == 2 {
			f.WriteU32(at+flags, 0x100)
		}
	}
	// Keep the existing ID 1 item and expose a small contiguous item table.
	itemStride := 36
	if version == "FE6" {
		itemStride = 32
	}
	for i := 2; i <= 9; i++ {
		at := f.GetPosition("items") + i*itemStride
		f.WriteU32(at+12, 0)
		f.WriteU8(at+6, byte(i))
	}
	f.WriteU32(f.GetPosition("items")+10*itemStride+12, 0xFFFFFFFF)
	names := []string{
		"arena_melee",
		"arena_ranged",
		"arena_magic",
		"arena_base",
		"arena_upgrades",
		"arena_link",
	}
	addresses := []int{
		0x290001,
		0x290001,
		0x290002,
		0x290100,
		0x290120,
		0x290160,
	}
	for i, sites := range pointers {
		for _, site := range sites {
			f.WritePointer(site, addresses[i])
		}
		f.MarkPosition(names[i], addresses[i])
		f.tablePointers[names[i]] = sites
	}
	f.WriteAt(addresses[0], []byte{1, 2, 1, 0, 0xA5, 0xA6})
	f.WriteAt(addresses[3], []byte{1, 2, 3, 4, 0, 5, 6, 7, 0xB5, 0xB6})
	f.WriteAt(
		addresses[4],
		[]byte{
			1,
			8,
			9,
			0,
			2,
			8,
			9,
			0,
			3,
			8,
			9,
			0,
			4,
			8,
			9,
			0,
			5,
			8,
			9,
			0,
			6,
			8,
			0,
			7,
			0,
			0xFF,
			0xC5,
			0xC6,
		},
	)
	if len(pointers) == 6 {
		f.WriteAt(
			addresses[5],
			[]byte{
				1,
				0,
				0,
				0,
				2,
				0,
				0x34,
				0x12,
				3,
				0,
				4,
				0,
				5,
				0,
				6,
				0,
				7,
				0,
				8,
				0,
				9,
				0,
				0xD5,
				0xD6,
			},
		)
	}
	f.WritePointer(0x290200, addresses[0])
	f.MarkPosition("arena_alias", 0x290200)
	return f
}
