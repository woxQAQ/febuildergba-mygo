package testutil

// NewMapResizeFixture provides three chapters, a 6×10 layout, coordinate
// resources and independent PLIST roots. Chapters 0 and 1 share the edited
// entry; chapter 2 retains an independent entry to the same old bytes.
func NewMapResizeFixture(version string) *FixtureBuilder {
	f := NewAreaFixture(version)
	stride, changes := 148, 0x345B4
	switch version {
	case versionFE6:
		stride, changes = 68, 0x2BB9C
	case versionFE7J:
		changes = 0x31AE8
	case versionFE7U:
		stride, changes = 152, 0x315B8
	case versionFE8U:
		changes = 0x346AC
	}
	layout := make([]byte, 2+6*10*2)
	layout[0], layout[1] = 6, 10
	layout[2+(2*6+1)*2] = 4
	layout[2+(8*6+4)*2] = 4
	layout[2+(9*6+5)*2] = 4
	layout = append(layout, 0xA5, 0x5A, 0xC3, 0x7E)
	f.WriteAt(0x238000, literalLZ(layout))
	f.WritePointer(f.GetPosition("map_layout_list")+4*4, 0x238000)
	f.WritePointer(f.GetPosition("map_layout_list")+6*4, 0x238000)
	f.MarkPosition("map_layout", 0x238000)
	base := f.GetPosition("chapters")
	f.WriteU8(base+11, 9)
	raw := f.Bytes()
	f.WriteAt(base+stride, raw[base:base+stride])
	f.WriteAt(base+2*stride, raw[base:base+stride])
	f.WriteU8(base+2*stride+8, 6)
	f.WritePointer(changes, 0x221000)
	f.WritePointer(0x221000+9*4, 0x238400)
	f.WriteAt(0x238400, []byte{1, 2, 3, 3, 4, 0xCD, 0xAB, 0xEF})
	f.WritePointer(0x238408, 0x238800)
	f.WriteAt(0x23840C, []byte{2, 5, 9, 4, 4, 0xFE, 0xDC, 0xBA})
	f.WritePointer(0x238414, 0x238840)
	f.WriteAt(0x238418, []byte{0xFF, 0x5A, 0xA5})
	f.MarkPosition("resize_changes", 0x238400)
	f.MarkPosition("resize_changes_root", changes)
	units, script := f.GetPosition(
		"deploy_units",
	), f.GetPosition(
		"deploy_script",
	)
	if version == versionFE8J || version == versionFE8U {
		f.WriteU16(units+20+4, 7|(12<<6))
		f.WriteAt(
			script+8,
			[]byte{
				0x40,
				0x2F,
				0,
				0,
				1,
				0,
				4,
				8,
				0x20,
				0x26,
				0xFF,
				0xFF,
				0x20,
				1,
				0,
				0,
			},
		)
		return f
	}
	f.WriteAt(units+16+4, []byte{7, 12, 7, 12})
	move, path := uint32(0x24), uint32(0x23)
	switch version {
	case versionFE6:
		move, path = 0x0F, 0x0E
	case versionFE7U:
		move, path = 0x26, 0x25
	}
	f.WriteU32(script+8, move)
	f.WriteU32(script+12, 1)
	f.WriteU32(script+16, 4|(8<<16))
	f.WriteU32(script+20, path)
	f.WriteU32(script+24, 4|(8<<16))
	f.WritePointer(script+28, 0x243800)
	f.WriteU32(script+32, 0)
	f.WriteU32(script+36, 0)
	f.WriteAt(0x243800, []byte{1, 2, 4})
	return f
}
