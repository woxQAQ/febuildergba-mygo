package testutil

// NewWorldMapEventFixture supplies two non-chapter-index event IDs, shared roots,
// a shared child, an opaque instruction and nonzero unused operand bytes.
func NewWorldMapEventFixture(version string) *FixtureBuilder {
	f := NewWorldMapRouteFixture(version)
	if version != "FE8U" && version != "FE8J" {
		return f
	}
	roots := []int{0xBA3D0, 0xBA41C, 0x9EC0, 0x9ED8}
	if version == "FE8J" {
		roots = []int{0xBF264, 0xBF2B0, 0x9E20, 0x9E38}
	}
	const beginning, intro, script, child, opaque, alias = 0x289000, 0x289100, 0x28A000, 0x28B000, 0x28C000, 0x28D000
	f.WritePointer(roots[0], beginning)
	f.WritePointer(roots[1], intro)
	f.WritePointer(roots[2], script)
	f.WritePointer(roots[3], opaque)
	for _, base := range []int{beginning, intro} {
		f.WriteAt(base, make([]byte, 59*4))
	}
	f.WritePointer(beginning+4, child)
	f.WritePointer(beginning+7*4, script)
	f.WritePointer(beginning+9*4, script)
	f.WritePointer(intro+7*4, child)
	f.WritePointer(alias, script)
	f.WriteU8(f.GetPosition("chapters")+117, 7)
	f.WriteU8(f.GetPosition("chapters")+148+117, 9)
	f.WriteAt(script, []byte{
		0x40,
		0x83,
		0x11,
		0x22,
		4,
		0,
		6,
		0,
		0x40,
		0x90,
		0xA1,
		0xB2,
		2,
		0,
		0,
		0,
		0x60,
		0x9E,
		0xA3,
		0xB4,
		2,
		0,
		1,
		0,
		2,
		0,
		3,
		0,
		0x80,
		0xA9,
		0xC3,
		0xD4,
		2,
		0,
		0x85,
		0xA6,
		2,
		0,
		3,
		0,
		0xFC,
		0xFF,
		10,
		0,
		0x40,
		0xAC,
		0xC5,
		0xD6,
		0xFF,
		0xFF,
		0xAB,
		0xCD,
		0x60,
		0xA7,
		0xC7,
		0xD8,
		2,
		0,
		0xDE,
		0xAD,
		0xFE,
		0xFF,
		0x90,
		1,
		0xC0,
		0xA8,
		0xC9,
		0xDA,
		2,
		0,
		0,
		0xA7,
		0xFF,
		0xFF,
		0xFE,
		0xFF,
		0x2C,
		1,
		0x96,
		0,
		60,
		0,
		0xC2,
		0xF4,
		0,
		0,
		0xCC,
		0xDD,
		0x40,
		0x0A,
		0,
		0,
		0,
		0,
		0,
		0,
		0x20,
		1,
		0,
		0,
	})
	f.WritePointer(script+92, child)
	f.WriteAt(child, []byte{0x40, 0x93, 0xCD, 0xAB, 5, 0, 0, 0, 0x20, 1, 0, 0})
	f.WriteAt(
		opaque,
		[]byte{
			0x20,
			0xD0,
			0xCD,
			0xAB,
			0x40,
			0x91,
			0,
			0,
			3,
			0,
			0,
			0,
			0x20,
			1,
			0,
			0,
		},
	)
	for name, at := range map[string]int{"wm_beginning": beginning, "wm_intro": intro, "wm_script": script, "wm_child": child, "wm_opaque": opaque, "wm_alias": alias, "wm_site": beginning + 7*4, "wm_alias_site": beginning + 9*4, "wm_beginning_root": roots[0], "wm_intro_root": roots[1], "wm_ending": roots[2], "wm_ending_other": roots[3]} {
		f.MarkPosition(name, at)
	}
	return f
}
