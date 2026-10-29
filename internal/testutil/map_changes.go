package testutil

// NewMapChangesFixture defines overlapping changes, shared tile bytes, two
// chapters using one entry and a third chapter retaining an independent alias.
func NewMapChangesFixture(version string) *FixtureBuilder {
	f := NewMapResizeFixture(version)
	stride := 148
	if version == versionFE6 {
		stride = 68
	}
	if version == versionFE7U {
		stride = 152
	}
	f.WriteU8(f.GetPosition("chapters")+2*stride+11, 10)
	f.WritePointer(0x221000+10*4, 0x238400)
	f.WriteAt(0x238400, []byte{3, 1, 2, 3, 2, 0xCD, 0xAB, 0xEF})
	f.WriteAt(0x23840C, []byte{7, 2, 2, 2, 2, 0xFE, 0xDC, 0xBA})
	f.WritePointer(0x238414, 0x238800)
	f.WriteAt(0x238800, []byte{0, 0, 4, 0, 0, 0, 4, 0, 0, 0, 4, 0})
	f.WriteAt(
		0x238418,
		[]byte{
			0x80,
			0x5A,
			0xA5,
			0x5A,
			0xCA,
			0xFE,
			0xBE,
			0xEF,
			0x10,
			0x32,
			0x54,
			0x76,
		},
	)
	script := f.GetPosition("deploy_script")
	switch version {
	case versionFE6:
		f.WriteAt(
			script,
			[]byte{
				0x29,
				0,
				0,
				0,
				3,
				0,
				0,
				0,
				0x2A,
				0,
				0,
				0,
				2,
				2,
				0,
				0,
				0,
				0,
				0,
				0,
			},
		)
	case versionFE7J:
		f.WriteAt(script, []byte{0x5D, 0, 3, 0, 0x5E, 0, 2, 2, 0, 0, 0, 0})
	case versionFE7U:
		f.WriteAt(script, []byte{0x5F, 0, 3, 0, 0x60, 0, 2, 2, 0, 0, 0, 0})
	default:
		f.WriteAt(
			script,
			[]byte{
				0x20,
				0x27,
				3,
				0,
				0x40,
				5,
				0x0B,
				0,
				2,
				0,
				2,
				0,
				0x20,
				0x27,
				0xFF,
				0xFF,
				0x20,
				1,
				0,
				0,
			},
		)
	}
	return f
}
