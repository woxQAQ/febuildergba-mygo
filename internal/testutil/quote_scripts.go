package testutil

// NewQuoteScriptFixture adds independent global scripts to the chapter/text
// fixture. Native quote pointers include shared payloads, separate records and
// FE7's two fallback roles; their bytes do not depend on production definitions.
func NewQuoteScriptFixture(version string) *FixtureBuilder {
	f := NewScriptTextFixture(version)
	if version == versionFE6 {
		return f
	}
	const root, child = 0x292000, 0x293000
	f.MarkPosition("quote_script_root", root)
	f.MarkPosition("quote_script_child", child)
	f.MarkPosition("quote_script_text", root)
	battleOffset, deathStride, callOffset := 12, 12, 4
	if version == versionFE8U || version == versionFE8J {
		f.WriteAt(root, []byte{
			0x20, 0x1B, 1, 0,
			0x40, 0x0A, 0, 0, 0, 0, 0, 0,
			0x20, 1, 0, 0,
		})
		f.WriteAt(child, []byte{0x20, 0x1B, 2, 0, 0x20, 1, 0, 0})
	} else {
		battleOffset, deathStride, callOffset = 8, 16, 20
		call := uint32(0xCB)
		if version == versionFE7J {
			call = 0xCC
		}
		for i, word := range []uint32{0x0D, 1, 0x0F, 1, 2, call, PointerWord(child), 0x0A, 0} {
			f.WriteU32(root+i*4, word)
		}
		for i, word := range []uint32{0x0D, 2, 0x0A, 0} {
			f.WriteU32(child+i*4, word)
		}
		for _, name := range []string{"death_lyn", "death_other"} {
			at := f.GetPosition("quote_" + name)
			f.WritePointer(at+4, root)
			f.WritePointer(at+16, root)
		}
	}
	f.WritePointer(root+callOffset+4, child)
	f.MarkPosition("quote_script_call", root+callOffset)
	battle, death := f.GetPosition("quote_battle"), f.GetPosition("quote_death")
	f.MarkPosition("quote_script_battle_site", battle+battleOffset)
	f.MarkPosition("quote_script_death_site", death+8)
	f.WritePointer(battle+battleOffset, root)
	f.WritePointer(battle+16+battleOffset, root)
	f.WritePointer(death+8, root)
	f.WritePointer(death+deathStride+8, root)
	// Exercise actual script fallback while retaining a second record with text.
	f.WriteU32(battle+battleOffset-4, 0)
	if deathStride == 12 {
		f.WriteU16(death+6, 0)
	} else {
		f.WriteU32(death+4, 0)
	}
	return f
}
