package testutil

// NewObjectiveFixture combines flag/rout rules, areas, opaque callbacks and a
// flag-only marker. Ending-scene units and paths are exclusive to that scene.
func NewObjectiveFixture(version string) *FixtureBuilder {
	f := NewTalkFixture(version)
	const ending, units, path = 0x249000, 0x249400, 0x249800
	endOffset, defeat, rout, routKind, unitStride := 76, 0x592104, 6, byte(
		1,
	), 20
	switch version {
	case versionFE8J:
		defeat = 0x5B9EE8
	case versionFE7U:
		endOffset, defeat, rout, routKind, unitStride = 60, 0xCA749C, 0x08079FE9, 14, 16
	case versionFE7J:
		endOffset, defeat, rout, routKind, unitStride = 60, 0xD6F71C, 0x0807AA5D, 14, 16
	case versionFE6:
		endOffset, defeat, rout, routKind, unitStride = 24, 0x66AC6C, 0x0806C2F9, 13, 16
	}
	raw := f.Bytes()
	sourceUnits, sourceScript := f.GetPosition(
		"talk_units",
	), f.GetPosition(
		"talk_script",
	)
	f.WriteAt(units, raw[sourceUnits:sourceUnits+unitStride*2+4])
	f.WriteAt(ending, raw[sourceScript:sourceScript+32])
	if unitStride == 20 {
		sourcePath := f.GetPosition("talk_path")
		f.WriteAt(path, raw[sourcePath:sourcePath+16])
		f.WritePointer(units+8, path)
		f.WritePointer(ending+20, units)
		f.WriteAt(
			defeat,
			[]byte{
				0x20,
				0x1A,
				0,
				0,
				0x20,
				0x1B,
				2,
				0,
				0x20,
				0x1D,
				0,
				0,
				0x22,
				0x1B,
				0,
				0,
				0x20,
				1,
				0,
				0,
			},
		)
	} else {
		f.WritePointer(ending+16, units)
		words := []uint32{0x0D, 2, 9, 0x0A, 0}
		if version == versionFE6 {
			words = []uint32{7, 2, 5, 6, 0}
		}
		for i, word := range words {
			f.WriteU32(defeat+i*4, word)
		}
	}
	table := f.GetPosition("area_table")
	f.WritePointer(f.GetPosition("deploy_header")+endOffset, ending)
	for i := range 7 {
		f.WriteAt(table+i*12, make([]byte, 12))
	}
	f.WriteAt(table, []byte{1, 0, 3, 0, 0, 0, 0, 0, 2, 0, 0, 0})
	if unitStride != 20 {
		f.WriteAt(table+10, []byte{0xAA, 0x55})
	}
	f.WritePointer(table+4, ending)
	f.WriteAt(table+12, raw[table+12:table+24])
	f.WriteAt(table+24, []byte{routKind, 0, 3, 0})
	f.WritePointer(table+28, ending)
	f.WriteU32(table+32, uint32(rout))
	f.WriteAt(table+36, []byte{1, 0, 0, 0, 0, 0, 0, 0, 0x65, 0, 0, 0})
	f.WritePointer(table+40, defeat)
	asmKind := byte(14)
	if version == versionFE6 {
		asmKind = 13
	}
	f.WriteAt(table+48, []byte{asmKind, 0, 0x33, 0, 0, 0, 0, 0, 1, 1, 0, 8})
	f.WritePointer(table+52, ending)
	f.WriteAt(table+60, raw[table+24:table+36])
	f.WriteAt(table+72, []byte{1, 0, 0x2A, 0, 1, 0, 0, 0, 0x29, 0, 0, 0})
	f.WriteU32(table+84, 0)
	for name, at := range map[string]int{
		"objective_table": table, "objective_ending": ending, "objective_ending_site": f.GetPosition("deploy_header") + endOffset,
		"objective_defeat": defeat, "objective_units": units, "objective_path": path, "objective_terminator": table + 84,
	} {
		f.MarkPosition(name, at)
	}
	return f
}
