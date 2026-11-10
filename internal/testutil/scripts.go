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
		word("", 0x0A, 1)
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
		word("", 6, 1)
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

// NewScriptTextFixture isolates shared chapter entries and direct text operands.
// Independent world-map directories are valid and empty, so incomplete results
// must arise from the specific script or resource damaged by a workflow.
func NewScriptTextFixture(version string) *FixtureBuilder {
	f := NewScriptFixture(version)
	header := f.GetPosition("deploy_header")
	f.WriteAt(header, make([]byte, 80))
	ending := 76
	roots := []int{0xBA3D0, 0xBA41C}
	endings := []int{0x9EC0, 0x9ED8}
	switch version {
	case versionFE8J:
		roots, endings = []int{0xBF264, 0xBF2B0}, []int{0x9E20, 0x9E38}
	case versionFE7U:
		ending, roots, endings = 60, []int{0xB561C}, []int{0x12A94, 0x12ADC}
	case versionFE7J:
		ending, roots, endings = 60, []int{0xB643C}, []int{0x1314C, 0x13194}
	case versionFE6:
		ending, roots, endings = 24, []int{0x9310C}, nil
	}
	for i, site := range roots {
		base := 0x289000 + i*0x400
		f.WritePointer(site, base)
		f.WriteAt(base, make([]byte, 256*4))
	}
	for _, site := range endings {
		f.WriteU32(site, 0)
	}
	root, child, conditions := f.GetPosition(
		"script_root",
	), f.GetPosition(
		"script_child",
	), f.GetPosition(
		"objective_table",
	)
	f.WritePointer(header+ending, root)
	f.WritePointer(header+12, conditions)
	f.WriteAt(conditions, make([]byte, 24))
	f.WriteU32(conditions, 1)
	f.WritePointer(conditions+4, root)
	f.WriteU32(conditions+8, 2)
	f.MarkPosition("text_entry", header+ending)
	f.MarkPosition("text_alias", conditions+4)
	f.MarkPosition("text_first", root)
	if version == versionFE8U || version == versionFE8J {
		f.WriteAt(root, []byte{
			0x20, 0x1B, 1, 0,
			0x21, 0x1B, 1, 0,
			0x40, 0x0A, 0, 0, 0, 0, 0, 0,
			0x20, 1, 0, 0,
		})
		f.WritePointer(root+12, child)
		f.WriteAt(child, []byte{0x20, 0x1B, 2, 0, 0x20, 1, 0, 0})
		f.MarkPosition("text_second", root+4)
		f.MarkPosition("text_call", root+8)
	} else {
		words := []uint32{7, 1, 8, 1, 0x25, PointerWord(child)}
		if version != versionFE6 {
			call := uint32(0xCB)
			if version == versionFE7J {
				call = 0xCC
			}
			words = []uint32{
				0x0D,
				1,
				0x0F,
				1,
				2,
				call,
				PointerWord(child),
				0x0A,
				0,
			}
		}
		for i, word := range words {
			f.WriteU32(root+i*4, word)
		}
		for i, word := range []uint32{7, 2, 6, 0} {
			if version != versionFE6 {
				word = []uint32{0x0D, 2, 0x0A, 0}[i]
			}
			f.WriteU32(child+i*4, word)
		}
		f.MarkPosition("text_second", root+8)
		callAt := root + 16
		if version != versionFE6 {
			callAt = root + 20
		}
		f.MarkPosition("text_call", callAt)
	}
	f.quoteTables(version)
	return f
}
