package testutil

// NewAreaFixture places fixed 12-byte general conditions beside the mixed FE7
// turn list. Flag, area and opaque records retain independent script operands.
func NewAreaFixture(version string) *FixtureBuilder {
	f := NewTurnFixture(version)
	const area, script, units, path = 0x242400, 0x242800, 0x244800, 0x243400
	raw := f.Bytes()
	stride := 16
	if version == versionFE8U || version == versionFE8J {
		stride = 20
	}
	sourceScript, sourceUnits := f.GetPosition(
		"deploy_script",
	), f.GetPosition(
		"deploy_units",
	)
	f.WriteAt(script, raw[sourceScript:sourceScript+12])
	f.WriteAt(units, raw[sourceUnits:sourceUnits+stride*2+4])
	f.WritePointer(script+4, units)
	if stride == 20 {
		sourcePath := 0x243000
		f.WriteAt(path, raw[sourcePath:sourcePath+16])
		f.WritePointer(units+8, path)
	}
	f.MarkPosition("area_script", script)
	f.MarkPosition("area_units", units)
	f.MarkPosition("area_path", path)
	f.WritePointer(f.GetPosition("deploy_header")+12, area)
	f.MarkPosition("area_table", area)
	f.WriteAt(area, []byte{1, 0, 3, 0, 0xEF, 0xBE, 0xAD, 0xDE, 6, 0, 0, 0})
	f.WriteAt(area+12, []byte{11, 0, 0x2A, 0, 0, 0, 0, 0, 2, 3, 4, 6})
	f.WritePointer(area+16, script)
	f.WriteAt(
		area+24,
		[]byte{
			0x7F,
			0,
			0x88,
			0x77,
			0xEF,
			0xBE,
			0xAD,
			0xDE,
			0x11,
			0x22,
			0x33,
			0x44,
		},
	)
	f.WriteU32(area+36, 0)
	return f
}
