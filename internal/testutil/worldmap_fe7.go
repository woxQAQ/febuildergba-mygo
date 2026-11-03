package testutil

import "testing"

// NewFE7WorldMapFixture supplies a relocated native directory, independent
// endings and aliased narration with word operands and retained padding.
func NewFE7WorldMapFixture(t *testing.T, version string) *FixtureBuilder {
	t.Helper()
	f := NewScriptFixture(version)
	index, root, ending, other, chapterStride, eventOffset := 2, 0xB561C, 0x12A94, 0x12ADC, 152, 121
	shift, call := uint32(0), uint32(0xCB)
	if version == "FE7J" {
		index, root, ending, other, chapterStride, eventOffset = 3, 0xB643C, 0x1314C, 0x13194, 148, 117
		shift, call = 1, 0xCC
	}
	images := ImageFixture(t, index).Bytes()
	f.WriteAt(ImageTable, images[ImageTable:ImageIcons])
	f.WritePointer(ImageLayouts[index].PortraitPointer, ImageTable)
	f.WriteU16(f.GetPosition("units")+6, 1)
	f.WriteU16(f.GetPosition("units")+52+6, 2)
	const table, scene, child, opaque, alias = 0x289000, 0x28A000, 0x28B000, 0x28C000, 0x28D000
	f.WritePointer(root, table)
	f.WriteAt(table, make([]byte, 45*4))
	for _, site := range []int{table + 7*4, table + 9*4, ending, alias} {
		f.WritePointer(site, scene)
	}
	f.WritePointer(other, opaque)
	f.WritePointer(table+44*4, child)
	// A pointer-shaped neighbor is outside the native directory.
	f.WritePointer(table+45*4, scene)
	f.WriteU8(f.GetPosition("chapters")+eventOffset, 7)
	f.WriteU8(f.GetPosition("chapters")+chapterStride+eventOffset, 9)
	at := scene
	emit := func(name string, words ...uint32) {
		if name != "" {
			f.MarkPosition(name, at)
		}
		for _, word := range words {
			f.WriteU32(at, word)
			at += 4
		}
	}
	emit("fe7_wm_init", 0xABCD0000|0xAC+shift, 0xAA000001, 0x00C80064, 0x84)
	emit("fe7_wm_face", 0xAABB0000|0xB1+shift, 3, 1, 0xDEADA534, 0xFFFFFFFF)
	emit("fe7_wm_text", 0xCCDD0000|0xB3+shift, 1)
	emit("fe7_wm_unit", 0x33440000|0xB7+shift, 2, 0x0064FFFF, 0xA1016001, 0)
	emit("fe7_wm_scroll", 0x55660000|0xC1+shift, 0x012C0064, 0xAABB003C, 5)
	emit("fe7_wm_clear", 0x77880000|0xB2+shift, 3, 0xDEADD25B, 0xFFFFFFFF)
	emit("", 0xB6+shift)
	emit("fe7_wm_fade", 0xAD+shift)
	emit("fe7_wm_end_scene", 0xAE+shift)
	emit("fe7_wm_call", call, 0x08000000+child)
	emit("", 0x00070002, 0x0A, 0)
	f.WriteAt(child, []byte{2, 0, 3, 0, 0x0A, 0, 0, 0, 0, 0, 0, 0})
	f.WriteAt(
		opaque,
		[]byte{
			byte(0xAF + shift),
			0,
			0x11,
			0x22,
			0x33,
			0x44,
			0x55,
			0x66,
			byte(0xB3 + shift),
			0,
			0xAA,
			0xBB,
			1,
			0,
			0,
			0,
			0x0A,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
		},
	)
	for name, pos := range map[string]int{"fe7_wm_root": root, "fe7_wm_table": table, "fe7_wm_site": table + 7*4, "fe7_wm_alias_site": table + 9*4, "fe7_wm_alias": alias, "fe7_wm_scene": scene, "fe7_wm_child": child, "fe7_wm_opaque": opaque, "fe7_wm_ending": ending, "fe7_wm_other_ending": other, "fe7_wm_scene_end": at, "fe7_wm_portrait_root": ImageLayouts[index].PortraitPointer} {
		f.MarkPosition(name, pos)
	}
	return f
}

// NewFE7WorldMapMarkersFixture replaces the shared narration with fixed native
// marker bytes. Opcode padding, palette bits and the fade-all operand are opaque.
func NewFE7WorldMapMarkersFixture(
	t *testing.T,
	version string,
) *FixtureBuilder {
	t.Helper()
	f := NewFE7WorldMapFixture(t, version)
	shift := uint32(0)
	if version == "FE7J" {
		shift = 1
	}
	at := f.GetPosition("fe7_wm_scene")
	for _, command := range []struct {
		name  string
		words []uint32
	}{
		{"highlight", []uint32{0xA1B200B9 + shift, 3, 8, 0xFFFFFFFE}},
		{"clear_highlight", []uint32{0x334400BA + shift, 2, 7}},
		{"clear_highlights", []uint32{0x556600BB + shift}},
		{"fade_highlights", []uint32{0x778800BC + shift, 0xDEADC0DE}},
		{"dot", []uint32{0x99AA00BD + shift, 4, 0x7FFFFFFF, 0xAABBCCD2, 0xFFFFFFFF}},
		{"clear_dot", []uint32{0xBBCC00BE + shift, 1}},
		{"after_markers", []uint32{0x00070002, 0x0A, 0}},
	} {
		f.MarkPosition("fe7_wm_"+command.name, at)
		for _, word := range command.words {
			f.WriteU32(at, word)
			at += 4
		}
	}
	f.MarkPosition("fe7_wm_scene_end", at)
	return f
}
