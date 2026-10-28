package testutil

// NewTalkFixture places directional talk, ASM and opaque conditions beside
// turn/area lists. Script, unit and movement resources are exclusive to talks;
// the text table includes compressed, plain-ROM and unreadable RAM entries.
func NewTalkFixture(version string) *FixtureBuilder {
	f := NewAreaFixture(version)
	const table, script, units, path = 0x246400, 0x247000, 0x248800, 0x248C00
	const textTable, tree, root, stream, plain = 0x312000, 0x310000, 0x310100, 0x311000, 0x313000
	textRoot, treeRoot, rootRoot := 0, 0x6E0, 0x6DC
	kind, asmKind, stride, unitStride := byte(3), byte(4), 16, 16
	switch version {
	case versionFE8U:
		textRoot, unitStride = 0xA2A0, 20
	case versionFE8J:
		textRoot, unitStride = 0xA000, 20
	case versionFE7U:
		textRoot, treeRoot, rootRoot = 0x12CB8, 0x6BC, 0x6B8
	case versionFE7J:
		textRoot = 0x13370
	case versionFE6:
		textRoot, kind, asmKind, stride = 0x13B10, 4, 13, 12
	default:
		panic("unknown talk fixture: " + version)
	}
	for i, word := range []uint32{0x00020001, 0x80000041, 0x80000000} {
		f.WriteU32(tree+i*4, word)
	}
	f.WritePointer(treeRoot, tree)
	f.WritePointer(rootRoot, root)
	f.WritePointer(root, tree)
	f.WritePointer(textRoot, textTable)
	f.WritePointer(textTable, stream)
	f.WritePointer(textTable+4, stream)
	f.WriteU32(textTable+8, 0x88000000|plain)
	f.WriteU32(textTable+12, 0x02000000)
	f.WriteU32(textTable+16, 0)
	f.WriteAt(stream, []byte{2, 0, 0, 0}) // A=0, end=1.
	f.WriteAt(plain, []byte{'B', 0})
	f.MarkPosition("talk_text_root", textRoot)
	f.MarkPosition("talk_text_table", textTable)

	raw := f.Bytes()
	originalUnits := f.GetPosition("area_units")
	f.WriteAt(units, raw[originalUnits:originalUnits+unitStride*2+4])
	if unitStride == 20 {
		originalPath := f.GetPosition("area_path")
		f.WriteAt(path, raw[originalPath:originalPath+16])
		f.WritePointer(units+8, path)
		f.WriteAt(
			script,
			[]byte{
				0x20,
				0x1A,
				0,
				0,
				0x20,
				0x1B,
				1,
				0,
				0x20,
				0x1D,
				0,
				0,
				0x22,
				0x1B,
				0,
				0,
				0x40,
				0x2C,
				1,
				0,
				0,
				0,
				0,
				0,
				0x20,
				0x30,
				0,
				0,
				0x20,
				1,
				0,
				0,
			},
		)
		f.WritePointer(script+20, units)
	} else {
		words := []uint32{0x0D, 1, 0x09, 0x30, 0, 0x39, 0x0A, 0}
		if version == versionFE7U {
			words[3], words[5] = 0x32, 0x3B
		}
		if version == versionFE6 {
			words = []uint32{0x07, 1, 0x05, 0x12, 0, 0x14, 0x06, 0}
		}
		for i, word := range words {
			f.WriteU32(script+i*4, word)
		}
		f.WritePointer(script+16, units)
	}
	f.WritePointer(f.GetPosition("deploy_header")+4, table)
	for i, k := range []byte{asmKind, kind, 0x7F} {
		at := table + i*stride
		f.WriteAt(at, []byte{k, 0, 0x55, 0, 0, 0, 0, 0, 1, 2, 0xA6, 0xB7})
		f.WritePointer(at+4, script)
		if stride == 16 {
			f.WriteU32(at+12, 0x002A0003)
			if version == versionFE7U || version == versionFE7J {
				f.WriteU8(at+13, 0xD4) // Ignored by the FE7 mode-byte check.
			}
		}
		if i == 0 && stride == 16 {
			f.WriteU32(at+12, 0x08000101)
		}
	}
	f.WriteU32(table+3*stride, 0)
	for name, at := range map[string]int{"talk_table": table, "talk_script": script, "talk_units": units, "talk_path": path, "talk_terminator": table + 3*stride} {
		f.MarkPosition(name, at)
	}
	return f
}
