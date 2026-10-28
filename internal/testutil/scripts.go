package testutil

// NewScriptFixture supplies a complete shared script, a child script and an
// early terminator followed by a referenced label. Native bytes are independent
// of the editor schema and preserve ordinary chapter resources around them.
func NewScriptFixture(version string) *FixtureBuilder {
	f := NewObjectiveFixture(version)
	const root, child = 0x250000, 0x251000
	site := f.GetPosition("objective_table") + 4
	f.WritePointer(site, root)
	f.WritePointer(f.GetPosition("objective_table")+40, root)
	at := root
	emit := func(name string, raw []byte) {
		if name != "" {
			f.MarkPosition(name, at)
		}
		f.WriteAt(at, raw)
		at += len(raw)
	}
	word := func(name string, values ...uint32) {
		if name != "" {
			f.MarkPosition(name, at)
		}
		for _, value := range values {
			f.WriteU32(at, value)
			at += 4
		}
	}
	switch version {
	case versionFE8U, versionFE8J:
		emit("script_label_one", []byte{0x20, 8, 1, 0})
		emit("script_flag", []byte{0x29, 2, 0x20, 0})
		emit("", []byte{0x20, 0x1A, 0, 0})
		emit("script_text", []byte{0x20, 0x1B, 1, 0})
		emit("", []byte{0x20, 0x1D, 0, 0})
		emit("", []byte{0x22, 0x1B, 0, 0})
		emit("script_load", []byte{0x40, 0x2C, 1, 0, 0, 0, 0, 0})
		f.WritePointer(at-4, f.GetPosition("objective_units"))
		emit("", []byte{0x20, 0x30, 0, 0})
		emit("script_call", []byte{0x40, 0x0A, 0, 0, 0, 0, 0, 0})
		f.WritePointer(at-4, child)
		emit("script_goto", []byte{0x20, 9, 2, 0})
		emit("", []byte{0x20, 1, 0, 0})
		emit("script_label_two", []byte{0x20, 8, 2, 0})
		emit("script_wait", []byte{0x20, 0x0E, 5, 0})
		emit("", []byte{0x20, 1, 0, 0})
		f.WriteAt(child, []byte{0x20, 0x0E, 3, 0, 0x20, 1, 0, 0})
	case versionFE7U, versionFE7J:
		label, flag, load, wait, call, gotoOp := uint32(
			0x42,
		), uint32(
			0x00200074,
		), uint32(
			0x30,
		), uint32(
			0x39,
		), uint32(
			0xCC,
		), uint32(
			0x43,
		)
		if version == versionFE7U {
			label, flag, load, wait, call, gotoOp = 0x44, 0x00200076, 0x32, 0x3B, 0xCB, 0x45
		}
		word("script_label_one", label, 1)
		word("script_flag", flag)
		word("script_text", 0x0D, 1)
		word("", 9)
		word("script_load", load, 0)
		f.WritePointer(at-4, f.GetPosition("objective_units"))
		word("", wait)
		word("script_call", call, 0)
		f.WritePointer(at-4, child)
		word("script_goto", gotoOp, 2)
		word("", 0x0A, 0)
		word("script_label_two", label, 2)
		word("script_wait", 0x00050002)
		word("", 0x0A, 0)
		f.WriteU32(child, 0x00030002)
		f.WriteU32(child+4, 0x0A)
		f.WriteU32(child+8, 0)
	case versionFE6:
		word("script_label_one", 0x1B, 1)
		word("script_flag", 0x34, 0x20)
		word("script_text", 7, 1)
		word("", 5)
		word("script_load", 0x12, 0)
		f.WritePointer(at-4, f.GetPosition("objective_units"))
		word("", 0x14)
		word("script_goto", 0x1C, 2)
		word("script_call", 0x25, 0)
		f.WritePointer(at-4, child)
		word("", 6, 0)
		word("script_label_two", 0x1B, 2)
		word("script_wait", 2, 5)
		word("", 6, 0)
		f.WriteU32(child, 2)
		f.WriteU32(child+4, 3)
		f.WriteU32(child+8, 6)
		f.WriteU32(child+12, 0)
	}
	f.MarkPosition("script_site", site)
	f.MarkPosition("script_alias_site", f.GetPosition("objective_table")+40)
	f.MarkPosition("script_root", root)
	f.MarkPosition("script_child", child)
	f.MarkPosition("script_end", at)
	return f
}
