package testutil

// NewTurnFixture includes a short FE7 record, an editable turn condition and an
// opaque condition. Pointer aliases and script bytes are independent of codecs.
func NewTurnFixture(version string) *FixtureBuilder {
	f := NewDeploymentFixture(version)
	header, turns, script := f.GetPosition(
		"deploy_header",
	), f.GetPosition(
		"deploy_turns",
	), f.GetPosition(
		"deploy_script",
	)
	f.WritePointer(header, turns)
	at := turns
	if version == versionFE7U || version == versionFE7J {
		f.WriteAt(
			at,
			[]byte{1, 0, 0x44, 0x33, 0, 0, 0, 0, 0x78, 0x56, 0x34, 0x12},
		)
		f.WritePointer(at+4, script)
		at += 12
	}
	f.MarkPosition("turn_condition", at)
	f.WriteAt(at, []byte{2, 0, 0x34, 0x12, 0, 0, 0, 0, 3, 5, 0x80, 0})
	f.WritePointer(at+4, script)
	stride := 12
	if version == versionFE7U || version == versionFE7J {
		stride = 16
		f.WriteU32(at+12, 4)
	}
	at += stride
	f.MarkPosition("turn_opaque", at)
	f.WriteAt(
		at,
		[]byte{
			0x7F,
			0,
			0x88,
			0x77,
			0xEF,
			0xBE,
			0xAD,
			0xDE,
			0x55,
			0x66,
			0x77,
			0x88,
		},
	)
	if stride == 16 {
		f.WriteU32(at+12, 0xAABBCCDD)
	}
	f.WriteU32(at+stride, 0)
	f.MarkPosition("turn_terminator", at+stride)
	return f
}
