package testutil

const (
	versionFE7U = "FE7U"
	versionFE7J = "FE7J"
	versionFE6  = "FE6"
)

// NewDeploymentFixture defines native records, aliased pointer sites, two
// chapters sharing an event header, and a typed opening/first-turn load.
func NewDeploymentFixture(version string) *FixtureBuilder {
	f := NewMapFixture(version)
	var root, chapterStride, eventOffset, first, second, stride, opening int
	switch version {
	case "FE8U":
		root, chapterStride, eventOffset, first, second, stride, opening = 0x346DC, 148, 116, 40, 44, 20, 72
	case "FE8J":
		root, chapterStride, eventOffset, first, second, stride, opening = 0x345E4, 148, 116, 40, 44, 20, 72
	case versionFE7U:
		root, chapterStride, eventOffset, first, second, stride, opening = 0x315D4, 152, 120, 24, 28, 16, 56
	case versionFE7J:
		root, chapterStride, eventOffset, first, second, stride, opening = 0x31B18, 148, 116, 24, 28, 16, 56
	case versionFE6:
		root, chapterStride, eventOffset, first, second, stride, opening = 0x2BBCC, 68, 58, 16, 20, 16, -1
	default:
		panic("unknown deployment fixture: " + version)
	}
	const list, header, units, path, script, turns = 0x240000, 0x240400, 0x241000, 0x243000, 0x242000, 0x242100
	f.WritePointer(root, list)
	f.WritePointer(list+4, header)
	f.WritePointer(list+8, header)
	f.WritePointer(header+first, units)
	f.WritePointer(header+second, units)
	f.WriteU8(f.GetPosition("chapters")+eventOffset, 1)
	f.WriteU8(f.GetPosition("chapters")+chapterStride+eventOffset, 2)
	if stride == 20 {
		f.WriteAt(
			units,
			[]byte{
				1,
				2,
				3,
				0x6D,
				0x81,
				0xD0,
				0xA7,
				2,
				0,
				0,
				0,
				0,
				0x11,
				0x22,
				0x33,
				0x44,
				0x55,
				0x66,
				0x77,
				0x88,
			},
		)
		f.WritePointer(units+8, path)
		f.WriteAt(
			path,
			[]byte{
				1,
				0xA1,
				0x40,
				0,
				0xDE,
				0xAD,
				0x34,
				0x12,
				4,
				0xB2,
				0x50,
				0,
				0xBE,
				0xEF,
				0x78,
				0x56,
			},
		)
		f.WriteAt(
			units+stride,
			[]byte{2, 0, 1, 8, 0, 0, 0xB9, 0, 0x78, 0x56, 0x34, 0x12},
		)
	} else {
		f.WriteAt(
			units,
			[]byte{
				1,
				2,
				3,
				0x6D,
				1,
				2,
				4,
				8,
				0x11,
				0x22,
				0x33,
				0x44,
				0x55,
				0x66,
				0x77,
				0x88,
			},
		)
		f.WriteAt(units+stride, []byte{2, 0, 1, 8, 0, 0, 0, 0})
	}
	f.WriteAt(units+2*stride, []byte{0, 0xD1, 0xE2, 0xF3})
	switch {
	case stride == 20:
		f.WriteAt(script, []byte{0x40, 0x2C, 1, 0})
		f.WritePointer(script+4, units)
		f.WriteAt(script+8, []byte{0x20, 1, 0, 0})
	case opening >= 0:
		op := uint32(0x30)
		if eventOffset == 120 {
			op = 0x32
		}
		f.WriteU32(script, op)
		f.WritePointer(script+4, units)
		f.WriteU32(script+8, 0x0A)
	default:
		f.WriteU32(script, 0x12)
		f.WritePointer(script+4, units)
		f.WriteU32(script+8, 0x06)
		f.WritePointer(header, turns)
		f.WriteU32(turns, 2)
		f.WritePointer(turns+4, script)
		f.WriteAt(turns+8, []byte{1, 1, 0, 0})
	}
	if opening >= 0 {
		f.WritePointer(header+opening, script)
	}
	for name, offset := range map[string]int{
		"deploy_root": root, "deploy_list": list, "deploy_header": header,
		"deploy_first": header + first, "deploy_second": header + second,
		"deploy_units": units, "deploy_path": path, "deploy_script": script,
		"deploy_script_site": script + 4, "deploy_turns": turns,
	} {
		f.MarkPosition(name, offset)
	}
	return f
}
